#!/usr/bin/env python3
"""Unified V2 offline evidence verification and sequential isolated reproduction."""
import argparse,collections,math,statistics,random,gzip,hashlib,json,os,pathlib,shutil,subprocess,sys,tempfile
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
def bind_sale(p,stage):
 configuration,round_number=stage[0],int(stage[1:])
 invocation=load(p/'sale/invocation.json');result=load(p/'result.json');window=load(p/'sale/window.json')
 if invocation['configuration']!=configuration or result['configuration']!=configuration or result['round']!=round_number:raise ValueError('sale stage configuration/round mismatch: '+stage)
 if not invocation['formal'] or invocation['duration']!=600 or invocation['products']!=30000:raise ValueError('sale protocol mismatch')
 if invocation['database']!=load(p/'sale/before.json.gz')['database']:raise ValueError('sale database identity mismatch')
 return {'configuration':configuration,'round':round_number,'identity':(invocation['project'],invocation['database'],window['startUnixNs']),'startUnixNs':window['startUnixNs']}
def bind_detail(p,stage):
 name=stage.removeprefix('detail-');parts=name.split('-');configuration=parts[0]
 state='disabled' if configuration=='A' else parts[1];pattern=parts[-1] if configuration!='C' else 'hotspot'
 invocation=load(p/'invocation.json');window=load(p/'window.json');pool=load(p/'pool.json.gz');before=load(p/'dataset-before.json');preparation=load(p/'preparation.json');empty=load(p/'cache-empty.json')
 if (invocation['configuration'],invocation['state'],invocation['pattern'])!=(configuration,state,pattern):raise ValueError('detail stage configuration/state/pattern mismatch: '+stage)
 if not invocation['formal'] or not invocation['fullDataset'] or invocation['duration']!=60 or invocation['seed']!=42:raise ValueError('detail protocol mismatch')
 expected={'users':10000,'products':230000,'listings':50000,'transactions':300000,'priceChanges':575000}
 if before['counts']!=expected or before['sold']!=180000:raise ValueError('wrong full initial detail fixture')
 ids=pool['ids'];size=100 if pattern=='hotspot' else 230000
 if pool['pattern']!=pattern or pool['seed']!=42 or pool['completeArchivePool']!=230000 or len(ids)!=size or len(set(ids))!=size:raise ValueError('detail pool shape mismatch')
 expected_ids=[format(i,'024x') for i in range(1,230001)];random.Random(42).shuffle(expected_ids)
 expected_ids=expected_ids[:size]
 digest=hashlib.sha256(json.dumps(ids,separators=(',',':')).encode()).hexdigest()
 if ids!=expected_ids or pool['sha256']!=digest:raise ValueError('detail pool/sequence hash mismatch')
 if window['expectedUsers']!=50 or window['readyUsers']!=50 or window['durationSeconds']!=60 or window['deadlineUnixNs']-window['startUnixNs']!=60_000_000_000:raise ValueError('detail window/ready protocol mismatch')
 positions=collections.Counter()
 for receipt in rows(p/'requests.jsonl.gz'):
  user=receipt['user'];index=receipt['sequenceIndex'];position=(user+50*index)%size
  if user not in range(50) or index!=positions[user] or receipt['poolPosition']!=position or receipt['productId']!=ids[position]:raise ValueError('detail per-user sequence duplicate/gap/pool mismatch')
  if not window['startUnixNs']<=receipt['startedUnixNs']<window['deadlineUnixNs'] or receipt['receivedUnixNs']<receipt['startedUnixNs']:raise ValueError('detail request timestamp outside launch window')
  positions[user]+=1
 if set(positions)!=set(range(50)):raise ValueError('detail receipts do not cover all 50 users')
 if empty['dbsize']!=0 or empty['unixNs']>=window['startUnixNs']:raise ValueError('detail cache was not empty before preparation')
 if preparation['errors']!=0:raise ValueError('detail prewarm errors')
 if state=='hot':
  receipts=list(rows(p/'prewarm.jsonl.gz'))
  if preparation['prewarmRequests']!=size or preparation['prewarmSeconds']<=0 or len(receipts)!=size or [r.get('productId') for r in receipts]!=ids or any(r.get('valid') is not True or r.get('status')!=200 for r in receipts):raise ValueError('incomplete/invalid hot prewarm')
 else:
  if preparation['prewarmRequests']!=0 or preparation['prewarmSeconds']!=0 or preparation['cacheDbsize']!=0:raise ValueError('cold/disabled preparation not empty')
 # Recorded command chronology proves no cache clear after complete hot prewarm.
 command_path=p/('commands.jsonl.gz' if (p/'commands.jsonl.gz').exists() else 'commands.jsonl')
 for command in rows(command_path):
  if 'FLUSHDB' in command.get('argv',[]) and command['startedUnixNs']>empty['unixNs']:raise ValueError('cache cleared after prewarm/reset evidence')
 return {'configuration':configuration,'state':state,'pattern':pattern,'poolSHA256':digest,'identity':(invocation['project'],invocation['database'],window['startUnixNs'])}
def sales(p,out,stage):
 binding=bind_sale(p,stage)
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
  compact={k:d[k] for k in ['formalPassed','configuredCutoffSeconds','measuredWindowSeconds','barrierValidated']};compact['uniqueSales']=d['client']['onTimeUniqueSales'];compact['metrics']=d['metrics'];compact['fingerprint']=hashlib.sha256((p/'initial-fingerprints.txt').read_bytes()).hexdigest();compact['samples']=r['sample_count'];compact.update(binding);return compact
def detail(p,out,stage):
 binding=bind_detail(p,stage)
 require_files(p,['pool.json.gz','requests.jsonl.gz','window.json','invocation.json','dataset-before.json','dataset-after.json','resources/samples.jsonl','resources/storage-before.json','resources/storage-after.json'])
 with tempfile.TemporaryDirectory() as td:
  t=pathlib.Path(td);shutil.copytree(p,t/'detail');t=t/'detail'
  run([sys.executable,ROOT/'scripts/measure-v2.py','--summarize','--output',t/'resources','--window',t/'window.json'])
  run([sys.executable,ROOT/'experiments/summarize_details.py','--run-dir',t]);d=load(t/'summary.json')
  before=load(p/'dataset-before.json');after=load(p/'dataset-after.json')
  unchanged=all(after[k]==(before['sold'] if k=='sold' else before['counts'][k]) for k in ['products','sold','transactions'])
  if not d['formalPassed'] or not unchanged:raise ValueError('detail raw evidence failure '+str(p))
  return {**binding,**{k:d[k] for k in ['formalPassed','requests','throughputPerSecond','latencyMs','telemetry','cacheHealthy']}}
def competition(p,configuration):
 require_files(p,['responses.jsonl','database-facts.jsonl','summary.json','cleanup.log','result.json'])
 summary=load(p/'summary.json');receipts=list(rows(p/'responses.jsonl'));facts=list(rows(p/'database-facts.jsonl'))
 if summary['configuration']!=configuration or summary['mode']!='formal' or summary['groupsRequested']!=100 or summary['requestsPerCompetitionGroup']!=50:raise ValueError('competition configuration/protocol mismatch')
 authoritative={};fact_entries={}
 for fact in facts:
  group=fact['group']
  target=authoritative if 'stdout' in fact else fact_entries
  if group in target:raise ValueError('duplicate competition database fact')
  if 'stdout' in fact:
   if fact['returnCode']!=0 or fact.get('stderr'):raise ValueError('read-only database command failed')
   target[group]=json.loads(fact['stdout'])
  else:target[group]=fact
 expected_groups=set(range(100))|{'retry'}
 if set(authoritative)!=expected_groups or set(fact_entries)!=expected_groups:raise ValueError('missing competition/same-key database facts')
 # Before-count evidence comes from the actual preceding public /users/me response.
 publication_baselines={};before=None
 for receipt in receipts:
  if receipt.get('phase')!='preparation':continue
  if receipt.get('path')=='/users/me' and receipt.get('method')=='GET':
   if receipt['status']!=200:raise ValueError('seller baseline HTTP failed')
   before=receipt['response']['completedSales']
  if receipt.get('path')=='/products' and receipt.get('method')=='POST':
   if receipt['status']!=201 or before is None:raise ValueError('publication/baseline proof missing')
   product=receipt['response']['id']
   if product in publication_baselines:raise ValueError('duplicate independent competition product')
   publication_baselines[product]=before;before=None
 observed_groups={r['group'] for r in receipts if r.get('phase') in ['different-keys','same-key','same-key-sequential','changed-content']}
 if observed_groups!=expected_groups:raise ValueError('missing/unexpected competition receipt group')
 entries=[];all_classified=[]
 for group in [*range(100),'retry']:
  retry=group=='retry';raw=[r for r in receipts if r.get('group')==group]
  phases={'same-key':set(range(50)),'same-key-sequential':set(range(50,54)),'changed-content':set(range(54,58))} if retry else {'different-keys':set(range(50))}
  if len(raw)!=(58 if retry else 50):raise ValueError('competition receipt count mismatch')
  for phase,indices in phases.items():
   selected=[r for r in raw if r['phase']==phase]
   if len(selected)!=len(indices) or {r['index'] for r in selected}!=indices:raise ValueError('missing/duplicate competition phase receipts')
  if any(r['phase'] not in phases for r in raw):raise ValueError('unexpected competition phase')
  product_ids={r['productId'] for r in raw}
  if len(product_ids)!=1:raise ValueError('competition product association mismatch')
  product=next(iter(product_ids));database=authoritative[group]
  if product not in publication_baselines:raise ValueError('missing raw seller baseline')
  seller_before=publication_baselines[product]
  if database['productId']!=product or database['sold'] is not True or database['listings']!=0 or database['sellerSales']!=seller_before+1:raise ValueError('competition archive/listing/seller increment mismatch')
  transactions=database['successfulTransactions']
  if len(transactions)!=1:raise ValueError('competition must have exactly one successful transaction')
  transaction=transactions[0]
  if transaction['buyerId']!='buyer' or transaction['priceCents']!=10000 or database['sale']['buyerId']!='buyer' or database['sale']['priceCents']!=10000:raise ValueError('competition buyer/price mismatch')
  initial=[r for r in raw if r['phase'] in ['different-keys','same-key']]
  keys={r['request']['idempotencyKey'] for r in initial}
  if len(keys)!=(1 if retry else 50) or any(r['request']['buyerId']!='buyer' or r['request']['priceCents']!=10000 for r in initial):raise ValueError('competition request keys/buyer/price mismatch')
  if transaction['key'] not in keys:raise ValueError('transaction key not associated with request')
  counts=dict(success=0,business_conflict=0,infrastructure_error=0,other_failure=0);successes=[]
  for r in raw:
   response=r.get('response',{});phase=r['phase'];status=r['status']
   if phase=='changed-content':
    valid=status==409 and response.get('error',{}).get('code')=='idempotency_conflict';kind='business_conflict'
   elif status==200:
    valid=response.get('success') is True and response.get('code')=='sold' and response.get('id')==transaction['id'] and response.get('productId')==product and response.get('sellerId')=='seller' and response.get('buyerId')=='buyer' and response.get('priceCents')==10000 and response.get('idempotencyKey')==transaction['key'];kind='success';successes.append(response)
   else:
    valid=not retry and status==409 and response.get('success') is False and response.get('code')=='product_sold' and response.get('productId')==product and response.get('buyerId')=='buyer' and response.get('priceCents')==10000 and response.get('idempotencyKey')==r['request']['idempotencyKey'];kind='business_conflict'
   if not valid or r.get('classification')!=kind:raise ValueError('competition status/business code/transaction receipt mismatch')
   counts[kind]+=1;all_classified.append((r,kind))
  if counts!=dict(success=54 if retry else 1,business_conflict=4 if retry else 49,infrastructure_error=0,other_failure=0):raise ValueError('competition/same-key outcomes mismatch')
  if retry and any(response!=successes[0] for response in successes):raise ValueError('same-key responses differ')
  upstreams=sorted({r.get('upstream') for r in raw if r.get('upstream')})
  if configuration=='C':
   if len(upstreams)!=2:raise ValueError('C competition did not reach both upstreams')
   if retry and any(len({r.get('upstream') for r in raw if r['phase']==phase})!=2 for phase in ['same-key-sequential','changed-content']):raise ValueError('C same-key/changed-content did not reach both upstreams')
  entry={'group':group,'sameKey':retry,'counts':counts,'database':database,'sellerBefore':seller_before,'upstreams':upstreams,'passed':True}
  claimed=summary['sameKey'] if retry else next((g for g in summary['groups'] if g['group']==group),{})
  for field,value in entry.items():
   if claimed.get(field)!=value or fact_entries[group].get(field)!=value:raise ValueError('competition summary differs from raw HTTP/database facts')
  entries.append(entry)
 if len(summary['groups'])!=100 or len({g['group'] for g in summary['groups']})!=100:raise ValueError('duplicate/missing summary competition group')
 def classified_statistics(classified):
  result={}
  for kind in ['success','business_conflict','infrastructure_error','other_failure']:
   values=sorted(r['elapsedMs'] for r,c in classified if c==kind)
   result[kind]={'count':len(values),'medianMs':statistics.median(values) if values else None,'p95Ms':values[math.ceil(.95*len(values))-1] if values else None,'p99Ms':values[math.ceil(.99*len(values))-1] if values else None}
  return result
 statistics_raw=classified_statistics(all_classified)
 phase_statistics={phase:classified_statistics([(r,c) for r,c in all_classified if r['phase']==phase]) for phase in ['different-keys','same-key','same-key-sequential','changed-content']}
 if summary['statistics']!=statistics_raw or summary['passed'] is not True:raise ValueError('competition summary statistics/pass mismatch')
 if summary['statisticsByPhase']!=phase_statistics:raise ValueError('competition phase statistics differ from raw receipts')
 if load(p/'result.json')['exitCode']!=0 or load(p/'result.json')['cleanupExitCode']!=0:raise ValueError('competition cleanup/run failure')
 return {'configuration':configuration,'groups':100,'requestsPerGroup':50,'passed':True,'sameKey':entries[-1],'statistics':statistics_raw}
def verify(index,out):
 matrix=[];fingerprints=[];identities=set();pool_hashes={}
 for e in index['experiments']:
  p=(ROOT/e['path']).resolve();kind=e['kind'];r={'id':e['id'],'path':e['path'],'kind':kind}
  try:
   if kind=='sale':r['result']=sales(p,out,e['id']);fingerprints.append(r['result']['fingerprint'])
   elif kind=='detail':r['result']=detail(p,out,e['id'])
   elif kind=='functions':r['result']=functions(p)
   elif kind=='competition':r['result']=competition(p,e['id'][-1])
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
   if kind in ['sale','detail']:
    identity=tuple(r['result']['identity'])
    if identity in identities:raise ValueError('duplicate measured experiment identity')
    identities.add(identity)
   if kind=='detail':
    pattern=r['result']['pattern'];digest=r['result']['poolSHA256']
    if pattern in pool_hashes and pool_hashes[pattern]!=digest:raise ValueError('paired detail pools differ')
    pool_hashes[pattern]=digest
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
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--check-functions',type=pathlib.Path);p.add_argument('--check-stage',nargs=2,metavar=('STAGE','DIRECTORY'));p.add_argument('--mode',choices=['verify','reproduce'],default='verify');p.add_argument('--index',type=pathlib.Path,default=ROOT/'docs/validation/v2-evidence.json');p.add_argument('--output',type=pathlib.Path);a=p.parse_args()
 try:
  if a.check_stage:
   stage,path=a.check_stage;p=pathlib.Path(path)
   result=competition(p,stage[-1]) if stage.startswith('competition-') else detail(p,None,stage) if stage.startswith('detail-') else sales(p,None,stage)
   print(json.dumps(result));return 0
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
