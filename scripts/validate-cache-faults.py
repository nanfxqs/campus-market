#!/usr/bin/env python3
"""Real Redis faults through HTTP; only the explicitly named isolated project."""
import argparse, concurrent.futures, json, subprocess, time, urllib.request, urllib.error, re
p=argparse.ArgumentParser();p.add_argument('--project',required=True);p.add_argument('--url',required=True);p.add_argument('--compose-override');p.add_argument('--mongo-failure',action='store_true');a=p.parse_args()
if not a.project.startswith('campus-v2-'):p.error('project must use isolated campus-v2-* namespace')
compose=['docker','compose','-p',a.project,'-f','compose.v2.yaml']
if a.compose_override:compose+=['-f',a.compose_override]
def dc(*args):
 r=subprocess.run(compose+list(args),check=True,text=True,capture_output=True)
 print(json.dumps({'command':list(args),'stdout':r.stdout.strip(),'stderr':r.stderr.strip()}),flush=True);return r.stdout.strip()
def redis(*args):return dc('exec','-T','redis','redis-cli',*args)
def http(method,path,body=None,token=None,want=None,max_seconds=10,want_cache=None):
 headers={'Content-Type':'application/json'}
 if token:headers['Authorization']='Bearer '+token
 req=urllib.request.Request(a.url+path,data=None if body is None else json.dumps(body).encode(),headers=headers,method=method)
 start=time.monotonic()
 try:r=urllib.request.urlopen(req,timeout=10)
 except urllib.error.HTTPError as e:r=e
 with r:
  result=json.load(r);elapsed=time.monotonic()-start
  redacted={k:v for k,v in result.items() if k!='accessToken'}
  print(json.dumps({'method':method,'path':path,'status':r.status,'elapsed_ms':elapsed*1000,'cache':r.headers.get('X-Detail-Cache'),'queries':r.headers.get('X-Mongo-Queries'),'bytes':r.headers.get('X-Mongo-BSON-Bytes'),'title':result.get('product',{}).get('title'),'sold':result.get('product',{}).get('sold'),'response':redacted}),flush=True)
  expected=want if want is not None else (201 if method=='POST' and path=='/products' else 200)
  assert r.status==expected,f'{path}: got {r.status}, expected {expected}'
  if want_cache is not None:assert r.headers.get('X-Detail-Cache')==want_cache,r.headers
  assert elapsed<max_seconds,f'{path}: {elapsed}s exceeds {max_seconds}s observation bound'
  if expected==503:assert result=={'error':{'code':'unavailable'}},result
  return result
print(json.dumps({'started_at':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),'project':a.project,'url':a.url}),flush=True)
token=http('POST','/auth/login',{'username':'seller','password':'CampusDemo123!'} )['accessToken']
body={'categoryId':'textbooks','title':'Before faults','description':'Fault validation','condition':'轻度使用','images':['https://example.invalid/fault.png'],'attributes':{'author':'A'},'priceCents':12345}
product=http('POST','/products',body,token);path='/products/'+product['id'];del body['priceCents']
try:
 http('GET',path);http('GET',path)
 redis('CLIENT','PAUSE','2000','ALL')
 assert http('GET',path,max_seconds=1,want_cache='fallback')['product']['title']=='Before faults'
 redis('PING')
 # A real eviction: reducing maxmemory below current allocations evicts all keys.
 before=redis('INFO','stats');redis('CONFIG','SET','maxmemory','1');after=redis('INFO','stats');redis('CONFIG','SET','maxmemory','128mb')
 def evictions(info):return int(next(x.split(':')[1] for x in info.splitlines() if x.startswith('evicted_keys:')))
 assert evictions(after)>evictions(before),'no actual eviction observed'
 assert http('GET',path)['product']['title']=='Before faults'
 redis('FLUSHDB');redis('CONFIG','SET','maxmemory-policy','noeviction');redis('CONFIG','SET','maxmemory','1')
 assert 'OOM' in redis('SET','write-failure-probe','value'),'Redis did not reject actual SET'
 body['title']='Write failure current';http('PUT',path,body,token)
 assert http('GET',path)['product']['title']==body['title']
 assert http('GET',path)['product']['title']==body['title']
 redis('CONFIG','SET','maxmemory','128mb');redis('CONFIG','SET','maxmemory-policy','allkeys-lru')
 redis('FLUSHDB');redis('CLIENT','PAUSE','80','WRITE')
 with concurrent.futures.ThreadPoolExecutor() as pool:
  old=pool.submit(http,'GET',path);time.sleep(.01);body['title']='After late write';http('PUT',path,body,token);old.result()
 assert http('GET',path)['product']['title']==body['title']
 dc('stop','redis');assert http('GET',path,want_cache='fallback')['product']['title']==body['title']
 body['title']='Edited during actual Redis outage';http('PUT',path,body,token)
 http('PATCH',path+'/price',{'priceCents':23456},token)
 http('PATCH','/users/me',{'nickname':'Fault current seller'},token)
 current=http('GET',path);assert current['product']['title']==body['title'] and current['product']['priceCents']==23456 and current['seller']['nickname']=='Fault current seller' and current['priceHistory'][0]['newPriceCents']==23456
 sale={'buyerId':'buyer','priceCents':20000,'idempotencyKey':'redis-fault-'+product['id']}
 first=http('POST',path+'/sale',sale,token)
 assert http('POST',path+'/sale',sale,token)==first,'idempotent result changed during Redis outage'
 conflict=dict(sale,idempotencyKey='conflict-'+product['id'])
 http('POST',path+'/sale',conflict,token,want=409)
 current=http('GET',path);assert current['product']['sold'] and not current['available'] and current['product']['sale']['priceCents']==20000
 dc('start','redis')
 for i in range(50):
  try:
   if redis('PING')=='PONG':break
  except subprocess.CalledProcessError:pass
  time.sleep(.1)
 assert http('GET',path,want_cache='miss')['product']['title']==body['title'];assert http('GET',path,want_cache='hit')['product']['sold']
 assert re.fullmatch('[0-9a-f]{24}',product['id'])
 code="""const d=db.getSiblingDB('campus_v2_fault');const id='%s';const p=d.products.findOne({_id:id});const n=d.transactions.countDocuments({productId:id,success:true});const l=d.listings.countDocuments({_id:id});const seller=d.users.findOne({_id:'seller'});const result={productId:id,sold:p.sold,sale:p.sale,successfulTransactions:n,listings:l,sellerSales:seller.completedSales};print(JSON.stringify(result));if(!p.sold||n!==1||l!==0||seller.completedSales!==1)quit(1);""" % product['id']
 # This is an explicitly approved read-only external persistence seam.
 if a.mongo_failure:
  dc('exec','-T','mongo','mongosh','--quiet','--eval',code)
  dc('stop','mongo')
  for endpoint in [path,'/health','/users/me']:
   http('GET',endpoint,token=token if endpoint=='/users/me' else None,want=503,max_seconds=3)
  dc('up','-d','--wait','mongo')
  deadline=time.monotonic()+30
  while True:
   try:http('GET','/health');break
   except (AssertionError,urllib.error.URLError):
    if time.monotonic()>deadline:raise
    time.sleep(.2)
  assert http('GET',path)['product']['title']==body['title']
 print(json.dumps({'result':'PASS','coverage':['actual eviction','SET OOM','bounded stalled Redis','late old write','actual stop/restart','edit/price/seller/history/sale during outage','same-key retry and different-key conflict','recovered fill','MongoDB outage 503 and recovery' if a.mongo_failure else 'MongoDB outage not requested']}),flush=True)
finally:
 if a.mongo_failure:dc('start','mongo')
 dc('start','redis')
 redis('CONFIG','SET','maxmemory','128mb');redis('CONFIG','SET','maxmemory-policy','allkeys-lru')
