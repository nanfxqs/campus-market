#!/usr/bin/env python3
"""HTTP race groups, independent fixtures, and read-only MongoDB facts."""
import argparse
import concurrent.futures
import json
import math
import pathlib
import statistics
import subprocess
import threading
import time
import urllib.error
import urllib.request

p=argparse.ArgumentParser()
p.add_argument('--project',required=True)
p.add_argument('--url',required=True)
p.add_argument('--configuration',choices=['A','B','C'],required=True)
p.add_argument('--output',required=True)
p.add_argument('--groups',type=int,default=100)
p.add_argument('--functional',action='store_true')
a=p.parse_args()
if not a.project.startswith('campus-v2-race-') or (a.groups<1 or a.groups>100) or (not a.functional and a.groups!=100):p.error('owned race project and exactly 100 groups required for formal runs')
out=pathlib.Path(a.output)
raw=open(out/'responses.jsonl','w',buffering=1)
facts=open(out/'database-facts.jsonl','w',buffering=1)
compose=['docker','compose','-p',a.project,'-f','compose.v2.yaml']
def request(method,path,body=None,token=None):
 headers={'Content-Type':'application/json'}
 if token:headers['Authorization']='Bearer '+token
 req=urllib.request.Request(a.url+path,data=None if body is None else json.dumps(body).encode(),headers=headers,method=method)
 begin=time.monotonic()
 try:
  try:response=urllib.request.urlopen(req,timeout=30)
  except urllib.error.HTTPError as e:response=e
  with response:
   text=response.read().decode();status=response.status;upstream=response.headers.get('X-Upstream-Addr')
  result=json.loads(text)
  return {'status':status,'elapsedMs':(time.monotonic()-begin)*1000,'upstream':upstream,'response':result}
 except Exception as e:return {'status':0,'elapsedMs':(time.monotonic()-begin)*1000,'error':type(e).__name__+': '+str(e)}
def required(method,path,body=None,token=None,want=200):
 r=request(method,path,body,token)
 redacted=dict(r)
 if 'response' in r:redacted['response']={k:v for k,v in r['response'].items() if k!='accessToken'}
 raw.write(json.dumps(dict(redacted,phase='preparation',method=method,path=path))+'\n')
 if r['status']!=want:raise RuntimeError(f'preparation failed: {redacted}')
 return r['response']
def classify(r,mode):
 response=r.get('response',{})
 if r['status']==200 and response.get('success') is True and response.get('code')=='sold':return 'success'
 if r['status']==409 and response.get('success') is False and response.get('code')=='product_sold':return 'business_conflict'
 if mode=='changed-content' and r['status']==409 and response.get('error',{}).get('code')=='idempotency_conflict':return 'business_conflict'
 if r['status']==0 or r['status']>=500:return 'infrastructure_error'
 return 'other_failure'
def group(number,token,same_key=False):
 before=required('GET','/users/me',token=token)['completedSales']
 product=required('POST','/products',{'categoryId':'textbooks','title':f'Independent competition {number}','description':'Not the dedicated formal sale pool','condition':'轻度使用','images':['https://example.invalid/race.png'],'attributes':{'author':'Race fixture'},'priceCents':12345},token,201)
 path='/products/'+product['id']+'/sale'
 barrier=threading.Barrier(51);release=[0.0]
 def worker(index):
  body={'buyerId':'buyer','priceCents':10000,'idempotencyKey':f'race-{number}-'+('shared' if same_key else str(index))}
  barrier.wait(timeout=15)
  dispatched=time.monotonic()
  r=request('POST',path,body,token)
  r.update(group=number,index=index,phase='same-key' if same_key else 'different-keys',productId=product['id'],request=body,dispatchOffsetMs=(dispatched-release[0])*1000)
  r['classification']=classify(r,r['phase'])
  return r
 with concurrent.futures.ThreadPoolExecutor(max_workers=50) as pool:
  pending=[pool.submit(worker,index) for index in range(50)]
  release[0]=time.monotonic();barrier.wait(timeout=15)
  results=[future.result() for future in pending]
 for r in results:raw.write(json.dumps(r)+'\n')
 if same_key:
  for index in range(4):
   r=request('POST',path,{'buyerId':'buyer','priceCents':10000,'idempotencyKey':f'race-{number}-shared'},token);r.update(group=number,index=50+index,phase='same-key-sequential',productId=product['id']);r['classification']=classify(r,'same-key');raw.write(json.dumps(r)+'\n');results.append(r)
  for index in range(4):
   r=request('POST',path,{'buyerId':'buyer','priceCents':10001,'idempotencyKey':f'race-{number}-shared'},token);r.update(group=number,index=54+index,phase='changed-content',productId=product['id']);r['classification']=classify(r,'changed-content');raw.write(json.dumps(r)+'\n');results.append(r)
 successes=[r for r in results if r['classification']=='success']
 counts={kind:sum(r['classification']==kind for r in results) for kind in ['success','business_conflict','infrastructure_error','other_failure']}
 js=f'''const d=db.getSiblingDB('campus_v2_race');const id={json.dumps(product['id'])};const p=d.products.findOne({{_id:id}});const success=d.transactions.find({{productId:id,success:true}}).toArray();const listings=d.listings.countDocuments({{_id:id}});const seller=d.users.findOne({{_id:'seller'}});print(JSON.stringify({{productId:id,sold:p.sold,sale:{{buyerId:p.sale?.buyerId,priceCents:Number(p.sale?.priceCents),confirmedAt:p.sale?.confirmedAt}},successfulTransactions:success.map(s=>({{id:s._id,buyerId:s.buyerId,priceCents:Number(s.priceCents),key:s.idempotencyKey}})),listings:listings,sellerSales:seller.completedSales}}));'''
 check=subprocess.run(compose+['exec','-T','mongo','mongosh','--quiet','--eval',js],text=True,capture_output=True)
 facts.write(json.dumps({'group':number,'returnCode':check.returncode,'stdout':check.stdout,'stderr':check.stderr})+'\n')
 database=json.loads(check.stdout) if check.returncode==0 else {}
 transaction=database.get('successfulTransactions',[])
 valid=len(transaction)==1 and database.get('sold') is True and database.get('listings')==0 and database.get('sellerSales')==before+1 and transaction[0]['buyerId']=='buyer' and transaction[0]['priceCents']==10000 and database.get('sale',{}).get('buyerId')=='buyer' and database.get('sale',{}).get('priceCents')==10000
 valid=valid and counts==({'success':54,'business_conflict':4,'infrastructure_error':0,'other_failure':0} if same_key else {'success':1,'business_conflict':49,'infrastructure_error':0,'other_failure':0})
 if successes:valid=valid and all(r['response']['id']==transaction[0]['id'] for r in successes) if transaction else False
 upstreams=sorted({r['upstream'] for r in results if r.get('upstream')})
 if a.configuration=='C':
  valid=valid and len(upstreams)==2
  if same_key:valid=valid and len({r['upstream'] for r in results if r['phase']=='same-key-sequential'})==2 and len({r['upstream'] for r in results if r['phase']=='changed-content'})==2
 entry={'group':number,'sameKey':same_key,'counts':counts,'database':database,'sellerBefore':before,'upstreams':upstreams,'passed':valid,'dispatchSpreadMs':max(r['dispatchOffsetMs'] for r in results if 'dispatchOffsetMs' in r)-min(r['dispatchOffsetMs'] for r in results if 'dispatchOffsetMs' in r)}
 facts.write(json.dumps(entry)+'\n');print(json.dumps(entry),flush=True)
 return entry,results
summary={'configuration':a.configuration,'mode':'functional' if a.functional else 'formal','groupsRequested':a.groups,'requestsPerCompetitionGroup':50,'groups':[]}
all_results=[]
try:
 token=required('POST','/auth/login',{'username':'seller','password':'CampusDemo123!'})['accessToken']
 for n in range(a.groups):
  entry,results=group(n,token);summary['groups'].append(entry);all_results.extend(results)
 entry,results=group('retry',token,True);summary['sameKey']=entry;all_results.extend(results)
 summary['passed']=all(g['passed'] for g in summary['groups']) and summary['sameKey']['passed']
finally:
 summary['statistics']={}
 summary['statisticsByPhase']={}
 for kind in ['success','business_conflict','infrastructure_error','other_failure']:
  values=sorted(r['elapsedMs'] for r in all_results if r['classification']==kind)
  summary['statistics'][kind]={'count':len(values),'medianMs':statistics.median(values) if values else None,'p95Ms':values[math.ceil(.95*len(values))-1] if values else None,'p99Ms':values[math.ceil(.99*len(values))-1] if values else None}
 for phase in ['different-keys','same-key','same-key-sequential','changed-content']:
  summary['statisticsByPhase'][phase]={}
  for kind in ['success','business_conflict','infrastructure_error','other_failure']:
   values=sorted(r['elapsedMs'] for r in all_results if r['phase']==phase and r['classification']==kind)
   summary['statisticsByPhase'][phase][kind]={'count':len(values),'medianMs':statistics.median(values) if values else None,'p95Ms':values[math.ceil(.95*len(values))-1] if values else None,'p99Ms':values[math.ceil(.99*len(values))-1] if values else None}
 (out/'summary.json').write_text(json.dumps(summary,indent=2))
 raw.close();facts.close()
if not summary.get('passed'):raise SystemExit('race experiment failed; raw evidence preserved')
