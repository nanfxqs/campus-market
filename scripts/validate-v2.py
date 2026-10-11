#!/usr/bin/env python3
"""Unified V2 offline evidence verification and sequential isolated reproduction."""
import argparse,collections,math,statistics,gzip,hashlib,json,os,pathlib,shutil,subprocess,sys,tempfile
ROOT=pathlib.Path(__file__).resolve().parents[1]
RUN_LOG=None
def load(p):
 with (gzip.open(p,'rt') if str(p).endswith('.gz') else open(p)) as f:return json.load(f)
def rows(p):
 with (gzip.open(p,'rt') if str(p).endswith('.gz') else open(p)) as f:
  for line in f:
   if line.strip():yield json.loads(line)
def functions(p):
 grouped=collections.defaultdict(list)
 for r in rows(p/'http.jsonl'):
  if r['phase']=='measurement':grouped[r['feature']].append(r)
 if len(grouped)!=8 or any(len(v)!=100 or len({r.get('sample') for r in v})!=100 or any(r.get('status')!=r.get('expected_status') or 'error' in r for r in v) for v in grouped.values()):raise ValueError('raw eight-function evidence requires 100 successful measurements each')
 ttl=load(p/'ttl-observations.json')
 if ttl['result']!='PASS' or len(ttl['observations'])!=10 or len({r['product_id'] for r in ttl['observations']})!=10:raise ValueError('raw TTL evidence requires ten observations')
 metrics={}
 for name,rs in grouped.items():
  lat=sorted(r['elapsed_ms'] for r in rs);metrics[name]={'medianMs':statistics.median(lat),'p95Ms':lat[math.ceil(.95*len(lat))-1],'p99Ms':lat[math.ceil(.99*len(lat))-1]}
 require_files(p,['instant-expiry.json','ttl-polls.jsonl','post-ttl-retention.txt'])
 return {'features':8,'samples':800,'ttlSamples':10,'metrics':metrics}
def run(argv,env=None):
 with (open(RUN_LOG,'a') if RUN_LOG else open(os.devnull,'w')) as log:
  if RUN_LOG:log.write('COMMAND '+json.dumps([str(x) for x in argv])+'\n');log.flush()
  subprocess.run([str(x) for x in argv],cwd=ROOT,env=env,check=True,stdout=log,stderr=subprocess.STDOUT)
def require_files(p,names):
 for name in names:
  if not (p/name).is_file():raise ValueError('missing raw artifact '+str(p/name))
def sales(p,out):
 require_files(p,['restore/manifest.json.gz','sale/window.json','sale/requests.jsonl.gz','sale/before.json.gz','sale/after.json.gz','resources/samples.jsonl','resources/storage-before.json','resources/storage-after.json','idle/environment.json','initial-fingerprints.txt'])
 before=load(p/'sale/before.json.gz')
 expected={'users':10000,'products':230000,'sold':180000,'listings':50000,'transactions':300000,'successfulTransactions':180000,'priceChanges':575000}
 if before['counts']!=expected:raise ValueError('wrong full initial sale fixture')
 with tempfile.TemporaryDirectory() as td:
  t=pathlib.Path(td);shutil.copytree(p/'sale',t/'sale');shutil.copytree(p/'resources',t/'resources')
  run([sys.executable,ROOT/'scripts/measure-v2.py','--summarize','--output',t/'resources','--window',t/'sale/window.json'])
  run([sys.executable,ROOT/'experiments/summarize_sales.py','--run-dir',t/'sale','--manifest',p/'restore/manifest.json.gz'])
  d=load(t/'sale/summary.json');r=load(t/'resources/summary.json')
  # CLI writes summary-window.json when --window is supplied.
  if (t/'resources/summary-window.json').exists():r=load(t/'resources/summary-window.json')
  if not d['formalPassed'] or d['client']['onTimeUniqueSales']<30000 or not d['database']['consistent'] or not r['coverage']['window']['covered']:raise ValueError('formal sale/raw resource failure '+str(p))
  compact={k:d[k] for k in ['formalPassed','configuredCutoffSeconds','measuredWindowSeconds','barrierValidated']};compact['uniqueSales']=d['client']['onTimeUniqueSales'];compact['metrics']=d['metrics'];compact['fingerprint']=hashlib.sha256((p/'initial-fingerprints.txt').read_bytes()).hexdigest();compact['samples']=r['sample_count'];return compact
def detail(p,out):
 require_files(p,['pool.json.gz','requests.jsonl.gz','window.json','invocation.json','dataset-before.json','dataset-after.json','resources/samples.jsonl','resources/storage-before.json','resources/storage-after.json'])
 with tempfile.TemporaryDirectory() as td:
  t=pathlib.Path(td);shutil.copytree(p,t/'detail');t=t/'detail'
  run([sys.executable,ROOT/'scripts/measure-v2.py','--summarize','--output',t/'resources','--window',t/'window.json'])
  run([sys.executable,ROOT/'experiments/summarize_details.py','--run-dir',t]);d=load(t/'summary.json')
  before=load(p/'dataset-before.json');after=load(p/'dataset-after.json')
  unchanged=all(after[k]==(before['sold'] if k=='sold' else before['counts'][k]) for k in ['products','sold','transactions'])
  if not d['formalPassed'] or not unchanged:raise ValueError('detail raw evidence failure '+str(p))
  return {k:d[k] for k in ['formalPassed','requests','throughputPerSecond','latencyMs','telemetry','cacheHealthy']}
def competition(p):
 require_files(p,['responses.jsonl','database-facts.jsonl','summary.json','cleanup.log','result.json'])
 d=load(p/'summary.json');receipt=list(rows(p/'responses.jsonl'));facts=list(rows(p/'database-facts.jsonl'))
 if not d['passed'] or len(d['groups'])!=100 or d['requestsPerCompetitionGroup']!=50 or len(receipt)<5000 or len(facts)<100:raise ValueError('competition evidence missing')
 for g in d['groups']:
  raw=[r for r in receipt if r.get('phase')=='different-keys' and r.get('group')==g['group']]
  if len(raw)!=50 or sum(r['status']==200 and r['response'].get('success') is True for r in raw)!=1 or sum(r['status']==409 for r in raw)!=49:raise ValueError('raw competition receipts mismatch')
  if g['counts']['success']!=1 or g['counts']['business_conflict']!=49 or len(g['database']['successfulTransactions'])!=1 or g['database']['listings']!=0 or not g['database']['sold']:raise ValueError('competition DB mismatch')
 if load(p/'result.json')['exitCode']!=0:raise ValueError('competition cleanup/run failure')
 return {'groups':100,'requestsPerGroup':50,'passed':True,'sameKey':d['sameKey']}
def verify(index,out):
 matrix=[];fingerprints=[]
 for e in index['experiments']:
  p=(ROOT/e['path']).resolve();kind=e['kind'];r={'id':e['id'],'path':e['path'],'kind':kind}
  try:
   if kind=='sale':r['result']=sales(p,out);fingerprints.append(r['result']['fingerprint'])
   elif kind=='detail':r['result']=detail(p,out)
   elif kind=='functions':r['result']=functions(p)
   elif kind=='competition':r['result']=competition(p)
   elif kind=='regression':
    if load(p/'result.json')['exitCode'] or load(p/'result.json')['cleanupExitCode']:raise ValueError('deployed regression failure')
    for c in 'ABC':
     text=(p/c/'http.log').read_text()
     if '--- PASS: TestV1FullSeedHTTP' not in text or '--- PASS: TestV1FullSeedHTTP/full-seed_expiry' not in text:raise ValueError('missing deployed '+c+' regression')
    r['result']={'deployedConfigurations':['A','B','C'],'fixtureProducts':200000}
   elif kind=='fault':
    require_files(p,['result.json','faults.jsonl','services.log','script-hashes.txt'])
    if load(p/'result.json')['exitCode']!=0 or not any(r.get('result')=='PASS' and 'actual stop/restart' in r.get('coverage',[]) for r in rows(p/'faults.jsonl')):raise ValueError('fault run failure')
    r['result']={'passed':True}
   elif kind=='proof':
    require_files(ROOT,e['files']);r['result']={'scope':e['scope'],'files':e['files']}
    if e['id']=='cache-edit':
     text=(ROOT/e['files'][0]).read_text()
     if '--- PASS: TestCacheConcurrentEditsExpiryResetAndUnavailableRedis' not in text:raise ValueError('cache concurrent-edit proof failed/missing')
    if e['id']=='cross-instance':
     rs=list(rows(ROOT/e['files'][0]));upstreams={x.get('upstream') for x in rs if x.get('upstream')}
     if len(upstreams)<2:raise ValueError('cross-instance upstream proof missing')
    if e['id']=='seed':
     a=ROOT/'docs/validation/v2/seed/fingerprints-first.txt';b=ROOT/'docs/validation/v2/seed/fingerprints-restored.txt'
     if a.read_bytes()!=b.read_bytes():raise ValueError('full restore fingerprint proof differs')
   else:raise ValueError('unknown evidence kind')
   r['passed']=True
  except Exception as ex:r.update(passed=False,error=str(ex))
  matrix.append(r)
 ids={e['id'] for e in index['experiments']};required=set(index['requiredIds'])|{c+str(i) for c in 'ABC' for i in range(1,4)}|{'competition-'+c for c in 'ABC'}|{'detail-'+n for n in ['A-hotspot','A-dispersed','B-cold-hotspot','B-hot-hotspot','B-cold-dispersed','B-hot-dispersed','C-cold','C-hot']}|{'functions','deployed-regression','fault','seed','cache-edit','cross-instance','failed-details','failed-preparation'};errors=[]
 if len(ids)!=len(index['experiments']):errors.append('duplicate experiment IDs')
 for e in index['experiments']:
  expected='sale' if e['id'] in {c+str(i) for c in 'ABC' for i in range(1,4)} else 'detail' if e['id'].startswith('detail-') else 'competition' if e['id'].startswith('competition-') else {'functions':'functions','fault':'fault','deployed-regression':'regression'}.get(e['id'],'proof')
  if e['kind']!=expected:errors.append('wrong evidence kind for '+e['id'])
 if required-ids:errors.append('missing required stages: '+str(sorted(required-ids)))
 if len(fingerprints)!=9 or len(set(fingerprints))!=1:errors.append('nine sale initial fingerprints must match')
 for baseline in index['sourceReuse'].get('productionBaselines',[]):
  changed=subprocess.check_output(['git','diff','--name-only',baseline,'HEAD','--','cmd','internal','Dockerfile','go.mod','go.sum','compose.v2.yaml','deploy'],cwd=ROOT,text=True).splitlines()
  changed=[name for name in changed if not name.endswith('_test.go')]
  if changed:errors.append('production source drift requires affected evidence review/rerun: '+baseline+' '+str(changed))
 result={'passed':not errors and all(r['passed'] for r in matrix),'errors':errors,'experiments':matrix,'sourceReuse':index['sourceReuse'],'limitations':index['limitations']}
 (out/'verification.json').write_text(json.dumps(result,indent=2)+'\n');return result
def reproduce(out):
 # Only this explicit mode mutates external resources. Every subentry owns cleanup.
 env={**os.environ,'BASE_TIME':'2026-10-11T00:00:00Z','RANDOM_SEED':'42'}
 run([ROOT/'scripts/validate-v2-regression.sh',out/'regression'],env)
 for c in 'ABC':
  project='campus-v2-unified-'+c.lower();db='campus_v2_unified_'+c.lower();dc=['docker','compose','-p',project,'-f','compose.v2.yaml','--profile','A','--profile','B','--profile','C']
  if subprocess.check_output(['docker','ps','-aq','--filter','label=com.docker.compose.project='+project]).strip() or subprocess.check_output(['docker','volume','ls','-q','--filter','label=com.docker.compose.project='+project]).strip():raise ValueError('refusing existing owned-name project '+project)
  try:
   run([ROOT/'scripts/run-formal-sales.py','--configuration',c,'--project',project,'--database',db,'--port','8098','--base-time',env['BASE_TIME'],'--output',out/('sales-'+c)],env)
   prep={**env,'COMPOSE_PROJECT_NAME':project,'DB_NAME':db,'API_PORT':'8098','RESULT_DIR':str(out/('detail-restore-'+c))}
   run([ROOT/'scripts/restore-v2.sh',c,'--reset'],prep)
   states=['disabled'] if c=='A' else ['cold','hot'];patterns=['hotspot'] if c=='C' else ['hotspot','dispersed']
   for pattern in patterns:
    for state in states:run([ROOT/'scripts/run-detail-load.py','--project',project,'--database',db,'--api','http://127.0.0.1:8098','--configuration',c,'--state',state,'--pattern',pattern,'--output',out/(c+'-'+state+'-'+pattern),'--formal','--confirm-isolated-cache'],prep)
   if c=='A':run([ROOT/'scripts/measure-functions.py','--project',project,'--database',db,'--url','http://127.0.0.1:8098','--output',out/'functions','--samples','100'],prep)
  finally:
   with (out/('cleanup-'+c+'.log')).open('w') as f:subprocess.run(dc+['down','--volumes','--remove-orphans'],cwd=ROOT,stdout=f,stderr=subprocess.STDOUT,check=True)
  run([ROOT/'scripts/validate-races.sh','--configuration',c,'--output',out/('competition-'+c),'--port','8098'],env)
 run([ROOT/'scripts/validate-cache-faults.sh','--output',out/'faults','--port','8098'],env)
def main():
 global RUN_LOG
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--check-functions',type=pathlib.Path);p.add_argument('--mode',choices=['verify','reproduce'],default='verify');p.add_argument('--index',type=pathlib.Path,default=ROOT/'docs/validation/v2-evidence.json');p.add_argument('--output',type=pathlib.Path);a=p.parse_args()
 try:
  if a.check_functions:print(json.dumps(functions(a.check_functions)));return 0
  if a.output is None:p.error('--output NEW_DIRECTORY required')
  a.output=a.output.resolve();a.output.mkdir(parents=True,exist_ok=False)
  if a.mode=='reproduce':
   if subprocess.check_output(['docker','ps','-q']).strip():raise ValueError('reproduction requires exclusive idle Docker host; foreign resources are never stopped')
   RUN_LOG=a.output/'commands.log';reproduce(a.output)
   index=load(a.index)
   for e in index['experiments']:
    if e['kind']=='sale':e['path']=str(a.output/('sales-'+e['id'][0])/('round-'+e['id'][1]))
    elif e['kind']=='detail':
     name=e['id'][7:];name=name.replace('A-','A-disabled-',1) if name.startswith('A-') else name
     if name.startswith('C-'):name+='-hotspot'
     e['path']=str(a.output/name)
    elif e['kind']=='competition':e['path']=str(a.output/e['id'])
    elif e['kind']=='regression':e['path']=str(a.output/'regression')
    elif e['kind']=='functions':e['path']=str(a.output/'functions')
    elif e['kind']=='fault':e['path']=str(a.output/'faults')
    elif e['id']=='cache-edit':e['files']=[str(a.output/'regression/boundary-tests.log')]
    elif e['id']=='cross-instance':e['files']=[str(a.output/'competition-C/responses.jsonl')]
   (a.output/'evidence-index.json').write_text(json.dumps(index,indent=2)+'\n');RUN_LOG=None
   r=verify(index,a.output);print(json.dumps({'passed':r['passed'],'experiments':len(r['experiments'])}));return 0 if r['passed'] else 1
  r=verify(load(a.index),a.output);print(json.dumps({'passed':r['passed'],'experiments':len(r['experiments']),'errors':r['errors']}));return 0 if r['passed'] else 1
 except Exception as e:print('raw verification failed: '+str(e));return 1
if __name__=='__main__':sys.exit(main())
