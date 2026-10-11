#!/usr/bin/env python3
"""Full-pool paired detail Locust experiments; reset only explicitly dedicated cache."""
import argparse,concurrent.futures,gzip,hashlib,json,os,pathlib,random,signal,subprocess,sys,time,urllib.request
from search_readiness import wait_for_full_cohort
ROOT=pathlib.Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--project',required=True);p.add_argument('--database',required=True);p.add_argument('--api',required=True);p.add_argument('--configuration',choices=['A','B','C'],required=True);p.add_argument('--state',choices=['disabled','cold','hot'],required=True);p.add_argument('--pattern',choices=['hotspot','dispersed'],required=True);p.add_argument('--output',type=pathlib.Path,required=True);p.add_argument('--duration',type=float,default=60);p.add_argument('--idle-seconds',type=float,default=30);p.add_argument('--seed',type=int,default=42);p.add_argument('--formal',action='store_true');p.add_argument('--locust',help='explicit pinned Locust executable; default frozen project runtime');p.add_argument('--confirm-isolated-cache',action='store_true')
a=p.parse_args()
if not a.project.startswith('campus-v2-') or not a.database.startswith('campus_v2_') or not a.confirm_isolated_cache:p.error('explicit dedicated campus-v2 project/database and --confirm-isolated-cache required')
if a.idle_seconds<=0 or a.duration<=0 or (a.formal and a.duration!=60):p.error('positive duration; formal exactly60 seconds')
if (a.configuration=='A') != (a.state=='disabled'):p.error('A disabled, B/C cold or hot required')
out=a.output.resolve();out.mkdir(parents=True,exist_ok=False)
commands=gzip.open(out/'commands.jsonl.gz','wt',compresslevel=3)
def command(argv,timeout=120):
    start=time.time_ns();r=subprocess.run(argv,capture_output=True,text=True,timeout=timeout,cwd=ROOT);commands.write(json.dumps({'argv':argv,'startedUnixNs':start,'finishedUnixNs':time.time_ns(),'exit':r.returncode,'stdout':r.stdout,'stderr':r.stderr})+'\n');commands.flush()
    if r.returncode:raise RuntimeError('command failed '+str(argv)+': '+r.stderr)
    return r.stdout
def save(name,value): (out/name).write_text(json.dumps(value,indent=2)+'\n')
ids=command(['docker','ps','-q','--filter','label=com.docker.compose.project='+a.project]).split()
if not ids:raise RuntimeError('project has no running containers')
containers=json.loads(command(['docker','inspect',*ids]));services={c['Config']['Labels']['com.docker.compose.service']:c for c in containers}
if 'mongo' not in services or 'redis' not in services:raise RuntimeError('dedicated Mongo and Redis must be healthy/running')
expected={'A':['api-a'],'B':['api-b'],'C':['api-c1','api-c2','nginx']}[a.configuration]
if not all(s in services for s in expected):raise RuntimeError('configuration service inventory mismatch')
for s in expected:
    c=services[s]
    if c['State'].get('Health',{}).get('Status')!='healthy':raise RuntimeError(s+' unhealthy')
    if s!='nginx':
        env=dict(entry.split('=',1) for entry in c['Config']['Env'])
        if env.get('DB_NAME')!=a.database or env.get('CACHE_ENABLED')!=('false' if a.configuration=='A' else 'true'):raise RuntimeError('actual application database/cache does not match experiment')
mongo=services['mongo']['Id'];redis=services['redis']['Id']
def redis_cmd(*args):return command(['docker','exec',redis,'redis-cli',*args])
script='const names=["users","products","listings","transactions","priceChanges"]; const counts=Object.fromEntries(names.map(n=>[n,db.getCollection(n).countDocuments({})])); print(JSON.stringify({counts,sold:db.products.countDocuments({sold:true}),ids:db.products.find({},{_id:1}).sort({_id:1}).toArray().map(x=>String(x._id))}));'
data=json.loads(command(['docker','exec',mongo,'mongosh','--quiet',a.database,'--eval',script]))
expected_counts={'users':10000,'products':230000,'listings':50000,'transactions':300000,'priceChanges':575000}
# Collection spelling is preserved from actual data; independently record cardinalities.
full=data['counts']==expected_counts and data['sold']==180000
if a.formal and not full:raise RuntimeError('formal requires untouched full V2 dataset; actual '+str(data['counts']))
all_ids=data.pop('ids');save('dataset-before.json',data)
if not all_ids or len(set(all_ids))!=len(all_ids):raise RuntimeError('empty/duplicate product pool')
random.Random(a.seed).shuffle(all_ids);pool_ids=all_ids[:100] if a.pattern=='hotspot' else all_ids
if a.formal and a.pattern=='hotspot' and len(pool_ids)!=100:raise RuntimeError('formal hotspot needs100 distinct products')
pool={'ids':pool_ids,'seed':a.seed,'pattern':a.pattern,'sha256':hashlib.sha256(json.dumps(pool_ids,separators=(',',':')).encode()).hexdigest(),'sequence':'VU index i reads pool[(i+50*sequenceIndex)%poolSize]; serial/no think time','completeArchivePool':len(all_ids)}
with gzip.open(out/'pool.json.gz','wt',compresslevel=3) as f:json.dump(pool,f,separators=(',',':'))
# Search ingestion is preparation; it does not warm product-detail cache.
if a.formal:
    wait_for_full_cohort(a.api,out/'search-readiness.jsonl','detail')
# Explicit exclusive experiment cache, checked against exact project above.
redis_cmd('FLUSHDB');save('cache-empty.json',{'dbsize':int(redis_cmd('DBSIZE')),'unixNs':time.time_ns()})
if int(redis_cmd('DBSIZE'))!=0:raise RuntimeError('cache was not empty after explicit reset')
warm_start=time.monotonic();warm_errors=0
if a.state=='hot':
    with gzip.open(out/'prewarm.jsonl.gz','wt',compresslevel=3) as warm:
        def request(product):
            start=time.monotonic()
            try:
                with urllib.request.urlopen(a.api+'/products/'+product,timeout=15) as response:
                    body=json.load(response);valid=response.status==200 and body['product']['id']==product
                    return {'productId':product,'valid':valid,'status':response.status,'cache':response.headers.get('X-Detail-Cache'),'elapsedMs':(time.monotonic()-start)*1000}
            except Exception as exc:return {'productId':product,'valid':False,'error':str(exc)}
        # map limits live work to50 workers; each involved ID is visited once, including archives.
        with concurrent.futures.ThreadPoolExecutor(max_workers=50) as executor:
            for row in executor.map(request,pool_ids):warm.write(json.dumps(row)+'\n');warm_errors+=not row['valid']
        if warm_errors:raise RuntimeError('prewarm HTTP failures')
save('preparation.json',{'prewarmSeconds':time.monotonic()-warm_start if a.state=='hot' else 0,'prewarmRequests':len(pool_ids) if a.state=='hot' else 0,'errors':warm_errors,'cacheDbsize':int(redis_cmd('DBSIZE')),'memory':redis_cmd('INFO','memory'),'statistics':redis_cmd('INFO','stats'),'configuration':redis_cmd('CONFIG','GET','save','appendonly','maxmemory','maxmemory-policy')})
locust=[a.locust] if a.locust else ['uv','run','--frozen','--project',str(ROOT/'experiments'),'locust']
version=command([*locust,'--version'])
locust_executable=a.locust or command(['uv','run','--frozen','--project',str(ROOT/'experiments'),'python','-c','import shutil;print(shutil.which("locust"))']).strip()
if '2.46.6' not in version:raise RuntimeError('pinned Locust2.46.6 required')
save('invocation.json',{**vars(a),'output':str(out),'fullDataset':full,'locustVersion':version,'git':command(['git','rev-parse','HEAD']).strip(),'gitStatus':command(['git','status','--short']),'sourceHashes':{str(f.relative_to(ROOT)):hashlib.sha256(f.read_bytes()).hexdigest() for f in (ROOT/'scripts/run-detail-load.py',ROOT/'scripts/search_readiness.py',ROOT/'experiments/detail_locust.py',ROOT/'experiments/summarize_details.py')}})
env={**os.environ,'DETAIL_OUTPUT':str(out),'DETAIL_CONFIGURATION':a.configuration,'DETAIL_DURATION':str(a.duration),'DETAIL_FORMAL':'1' if a.formal else '0'}
command([sys.executable,str(ROOT/'scripts/measure-v2.py'),'--project',a.project,'--database',a.database,'--output',str(out/'idle-resources'),'--phase','idle','--seconds',str(a.idle_seconds),'--locust',locust_executable,'--git-base','integration/v2'])
client=None;collector=None
try:
    with (out/'locust.log').open('w') as log:
        client=subprocess.Popen([*locust,'-f',str(ROOT/'experiments/detail_locust.py'),'--headless','--users','50','--spawn-rate','50','--host',a.api,'--stop-timeout','15s','--csv',str(out/'locust'),'--csv-full-history','--html',str(out/'locust.html')],stdout=log,stderr=subprocess.STDOUT,env=env,cwd=ROOT)
        with (out/'collector.log').open('w') as collectorlog:
            cmd=[sys.executable,str(ROOT/'scripts/measure-v2.py'),'--project',a.project,'--database',a.database,'--output',str(out/'resources'),'--phase','detail-load','--client-pid',str(client.pid),'--git-base','integration/v2']
            cmd+=['--locust',locust_executable]
            collector=subprocess.Popen(cmd,stdout=collectorlog,stderr=subprocess.STDOUT,cwd=ROOT)
            rc=client.wait(timeout=a.duration+180)
            # Last sample must cover deadline; retain extra boundary sample.
            time.sleep(2);collector.send_signal(signal.SIGTERM);cr=collector.wait(timeout=120)
            save('exits.json',{'locust':rc,'collector':cr})
            if rc or cr:raise RuntimeError('Locust/collector exited unsuccessfully')
finally:
    for process in (client,collector):
        if process and process.poll() is None:process.terminate();process.wait(timeout=120)
# Capture owned service logs after timing; capture status remains explicit and auxiliary.
with (out/'server-log-capture.log').open('w') as log:
    subprocess.run([sys.executable,str(ROOT/'scripts/capture-v2-logs.py'),'--project',a.project,'--output',str(out/'server-logs')],cwd=ROOT,stdout=log,stderr=subprocess.STDOUT)
save('cache-after.json',{'dbsize':int(redis_cmd('DBSIZE')),'memory':redis_cmd('INFO','memory'),'statistics':redis_cmd('INFO','stats')})
after=json.loads(command(['docker','exec',mongo,'mongosh','--quiet',a.database,'--eval','print(JSON.stringify({products:db.products.countDocuments({}),sold:db.products.countDocuments({sold:true}),transactions:db.transactions.countDocuments({})}))']))
save('dataset-after.json',after)
if after['products']!=data['counts']['products'] or after['sold']!=data['sold'] or after['transactions']!=data['counts']['transactions']:raise RuntimeError('detail experiment mutated business counts')
command([sys.executable,str(ROOT/'scripts/measure-v2.py'),'--output',str(out/'resources'),'--summarize','--window',str(out/'window.json')])
command([sys.executable,str(ROOT/'experiments/summarize_details.py'),'--run-dir',str(out)])
commands.close();result=json.loads((out/'summary.json').read_text());print(json.dumps(result,indent=2));sys.exit(0 if result['normalPassed'] else 1)
