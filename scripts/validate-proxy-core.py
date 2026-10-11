#!/usr/bin/env python3
"""Core C HTTP contracts with a bounded external expiry fixture."""
import argparse, json, subprocess, time, urllib.request, urllib.error
p=argparse.ArgumentParser();p.add_argument('--project',required=True);p.add_argument('--url',required=True);p.add_argument('--database',required=True);a=p.parse_args()
if not a.project.startswith('campus-v2-') or not a.database.startswith('campus_v2_'):p.error('dedicated V2 fixture required')
def http(method,path,body=None,token=None,status=200):
 h={'Content-Type':'application/json'}
 if token:h['Authorization']='Bearer '+token
 req=urllib.request.Request(a.url+path,data=None if body is None else json.dumps(body).encode(),headers=h,method=method)
 try:r=urllib.request.urlopen(req,timeout=20)
 except urllib.error.HTTPError as e:r=e
 with r:
  data=json.load(r);print(json.dumps({'method':method,'path':path,'status':r.status,'upstream':r.headers.get('X-Upstream-Addr'),'cache':r.headers.get('X-Detail-Cache')}),flush=True)
  assert r.status==status,(r.status,data)
  return data
seller=http('POST','/auth/login',{'username':'seller','password':'CampusDemo123!'})['accessToken']
buyer=http('POST','/auth/login',{'username':'buyer','password':'CampusDemo123!'})['accessToken']
body={'categoryId':'textbooks','title':'nginxproxyunique'+str(time.time_ns()),'description':'proxy core','condition':'轻度使用','images':['https://example.invalid/proxy.png'],'attributes':{'author':'C'},'priceCents':12345}
published=http('POST','/products',body,seller,201);path='/products/'+published['id']
http('POST','/products',body,status=401)
http('PATCH',path+'/price',{'priceCents':22222},buyer,403)
http('PATCH',path+'/price',{'priceCents':0},seller,400)
for _ in range(2):assert http('GET',path)['product']['title']==body['title']
body.pop('priceCents');body['description']='edited via another upstream'
http('PUT',path,body,seller)
assert http('GET',path)['product']['description']==body['description']
http('PATCH',path+'/price',{'priceCents':22222},seller)
http('PATCH','/users/me',{'nickname':'Proxy current seller'},seller)
detail=http('GET',path)
assert detail['product']['priceCents']==22222 and detail['seller']['nickname']=='Proxy current seller' and detail['priceHistory'][0]['newPriceCents']==22222
http('GET','/products?categoryId=textbooks');http('GET','/users/seller/home');http('GET','/categories/textbooks/condition-counts');http('GET','/openapi.json')
# Unique title avoids an unbounded full-seed query while checking actual Search ingestion.
for _ in range(60):
 result=http('GET','/search?q='+body['title'])
 if result['total']==1:break
 time.sleep(1)
assert result['total']==1 and result['results'][0]['id']==published['id']
# Agreed external expiry fixture mutates only this newly published disposable product.
js='const id='+json.dumps(published['id'])+';const expiry=new Date(Date.now()-1000);db.products.updateOne({_id:id},{$set:{expiresAt:expiry}});db.listings.updateOne({_id:id},{$set:{expiresAt:expiry}});'
subprocess.run(['docker','compose','-p',a.project,'-f','compose.v2.yaml','exec','-T','mongo','mongosh','--quiet',a.database,'--eval',js],check=True)
assert not http('GET',path)['available']
http('PATCH',path+'/price',{'priceCents':33333},seller,409)
http('POST',path+'/sale',{'buyerId':'buyer','priceCents':10000,'idempotencyKey':'proxy-expired-'+published['id']},seller,409)
assert http('GET','/search?q='+body['title'])['total']==0
print(json.dumps({'result':'PASS','coverage':['authentication','ownership','validation','cold/hot detail','edit','current price/profile/history','browse','seller home','category statistics','Search ingestion/current eligibility','expired sale/price rejection','retained archive']}),flush=True)
