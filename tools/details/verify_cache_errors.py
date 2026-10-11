#!/usr/bin/env python3
"""Real isolated B HTTP error-count seam: SET OOM and stopped Redis."""
import argparse,json,subprocess,time,urllib.request
p=argparse.ArgumentParser();p.add_argument('--project',required=True);p.add_argument('--url',required=True);a=p.parse_args()
if not a.project.startswith('campus-v2-detail-smoke'):p.error('disposable detail smoke project only')
ids=subprocess.check_output(['docker','ps','-q','--filter','label=com.docker.compose.project='+a.project,'--filter','label=com.docker.compose.service=redis'],text=True).split()
assert len(ids)==1;redis=ids[0]
def command(*cmd):
 r=subprocess.run(cmd,capture_output=True,text=True);print(json.dumps({'command':cmd,'exit':r.returncode,'stdout':r.stdout,'stderr':r.stderr}),flush=True);assert r.returncode==0;return r.stdout

def redis_cmd(*cmd):return command('docker','exec',redis,'redis-cli',*cmd)
def http(method,path,body=None,token=None):
 h={'Content-Type':'application/json'}
 if token:h['Authorization']='Bearer '+token
 req=urllib.request.Request(a.url+path,data=json.dumps(body).encode() if body else None,headers=h,method=method)
 with urllib.request.urlopen(req,timeout=10) as r:
  data=json.load(r);print(json.dumps({'method':method,'path':path,'status':r.status,'cache':r.headers.get('X-Detail-Cache'),'cacheErrors':r.headers.get('X-Detail-Cache-Errors')}),flush=True);return data,r.headers
login,_=http('POST','/auth/login',{'username':'seller','password':'CampusDemo123!'})
product,_=http('POST','/products',{'categoryId':'textbooks','title':'Cache error metric','description':'HTTP seam','condition':'轻度使用','images':['https://example.invalid/error.jpg'],'attributes':{'author':'Metric'},'priceCents':12345},login['accessToken']);path='/products/'+product['id']
oldmax=redis_cmd('CONFIG','GET','maxmemory').splitlines()[-1];oldpolicy=redis_cmd('CONFIG','GET','maxmemory-policy').splitlines()[-1]
try:
 redis_cmd('FLUSHDB');redis_cmd('CONFIG','SET','maxmemory-policy','noeviction');redis_cmd('CONFIG','SET','maxmemory','1')
 data,h=http('GET',path);assert data['product']['title']=='Cache error metric';assert h['X-Detail-Cache']=='miss' and h['X-Detail-Cache-Errors']=='1'
finally:
 redis_cmd('CONFIG','SET','maxmemory',oldmax);redis_cmd('CONFIG','SET','maxmemory-policy',oldpolicy)
try:
 command('docker','stop',redis);data,h=http('GET',path);assert data['product']['title']=='Cache error metric';assert h['X-Detail-Cache']=='fallback' and h['X-Detail-Cache-Errors']=='1'
finally:command('docker','start',redis)
for _ in range(30):
 try:
  redis_cmd('PING');break
 except AssertionError:time.sleep(.1)
else:raise TimeoutError('Redis recovery')
_,h=http('GET',path);assert h['X-Detail-Cache-Errors']=='0'
print(json.dumps({'result':'PASS','cases':['HTTP200+SET OOM count1','HTTP200+unreachable GET fallback count1','recovered current detail count0']}))
