#!/usr/bin/env python3
"""Real Redis faults through HTTP; only the explicitly named isolated project."""
import argparse, concurrent.futures, json, subprocess, time, urllib.request
p=argparse.ArgumentParser();p.add_argument('--project',required=True);p.add_argument('--url',required=True);a=p.parse_args()
if not a.project.startswith('campus-v2-'):p.error('project must use isolated campus-v2-* namespace')
compose=['docker','compose','-p',a.project,'-f','compose.v2.yaml']
def dc(*args):
 r=subprocess.run(compose+list(args),check=True,text=True,capture_output=True)
 print(json.dumps({'command':list(args),'stdout':r.stdout.strip(),'stderr':r.stderr.strip()}),flush=True);return r.stdout.strip()
def redis(*args):return dc('exec','-T','redis','redis-cli',*args)
def http(method,path,body=None,token=None):
 headers={'Content-Type':'application/json'}
 if token:headers['Authorization']='Bearer '+token
 req=urllib.request.Request(a.url+path,data=None if body is None else json.dumps(body).encode(),headers=headers,method=method)
 start=time.monotonic()
 with urllib.request.urlopen(req,timeout=10) as r:
  result=json.load(r);print(json.dumps({'method':method,'path':path,'status':r.status,'elapsed_ms':(time.monotonic()-start)*1000,'cache':r.headers.get('X-Detail-Cache'),'queries':r.headers.get('X-Mongo-Queries'),'bytes':r.headers.get('X-Mongo-BSON-Bytes'),'title':result.get('product',{}).get('title'),'sold':result.get('product',{}).get('sold')}),flush=True);return result
print(json.dumps({'started_at':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()),'project':a.project,'url':a.url}),flush=True)
token=http('POST','/auth/login',{'username':'seller','password':'CampusDemo123!'} )['accessToken']
body={'categoryId':'textbooks','title':'Before faults','description':'Fault validation','condition':'轻度使用','images':['https://example.invalid/fault.png'],'attributes':{'author':'A'},'priceCents':12345}
product=http('POST','/products',body,token);path='/products/'+product['id'];del body['priceCents']
try:
 http('GET',path);http('GET',path)
 # A real eviction: reducing maxmemory below current allocations evicts all keys.
 before=redis('INFO','stats');redis('CONFIG','SET','maxmemory','1');after=redis('INFO','stats');redis('CONFIG','SET','maxmemory','128mb')
 def evictions(info):return int(next(x.split(':')[1] for x in info.splitlines() if x.startswith('evicted_keys:')))
 assert evictions(after)>evictions(before),'no actual eviction observed'
 assert http('GET',path)['product']['title']=='Before faults'
 redis('FLUSHDB');redis('CONFIG','SET','maxmemory-policy','noeviction');redis('CONFIG','SET','maxmemory','1')
 body['title']='Write failure current';http('PUT',path,body,token)
 assert http('GET',path)['product']['title']==body['title']
 assert http('GET',path)['product']['title']==body['title']
 redis('CONFIG','SET','maxmemory','128mb');redis('CONFIG','SET','maxmemory-policy','allkeys-lru')
 redis('FLUSHDB');redis('CLIENT','PAUSE','80','WRITE')
 with concurrent.futures.ThreadPoolExecutor() as pool:
  old=pool.submit(http,'GET',path);time.sleep(.01);body['title']='After late write';http('PUT',path,body,token);old.result()
 assert http('GET',path)['product']['title']==body['title']
 dc('stop','redis');assert http('GET',path)['product']['title']==body['title']
 body['title']='Edited during actual Redis outage';http('PUT',path,body,token)
 http('PATCH',path+'/price',{'priceCents':23456},token)
 http('PATCH','/users/me',{'nickname':'Fault current seller'},token)
 current=http('GET',path);assert current['product']['title']==body['title'] and current['product']['priceCents']==23456 and current['seller']['nickname']=='Fault current seller' and current['priceHistory'][0]['newPriceCents']==23456
 http('POST',path+'/sale',{'buyerId':'buyer','priceCents':20000,'idempotencyKey':'redis-fault-'+product['id']},token)
 current=http('GET',path);assert current['product']['sold'] and not current['available'] and current['product']['sale']['priceCents']==20000
 dc('start','redis')
 for i in range(50):
  try:
   if redis('PING')=='PONG':break
  except subprocess.CalledProcessError:pass
  time.sleep(.1)
 assert http('GET',path)['product']['title']==body['title'];assert http('GET',path)['product']['sold']
 print(json.dumps({'result':'PASS','coverage':['actual eviction','SET OOM','late old write','actual stop/restart','edit/price/seller/history/sale during outage','recovered fill']}),flush=True)
finally:
 dc('start','redis')
 redis('CONFIG','SET','maxmemory','128mb');redis('CONFIG','SET','maxmemory-policy','allkeys-lru')
