#!/usr/bin/env python3
"""Small real C HTTP load for validating recorder coverage, not performance acceptance."""
import json, pathlib, sys, time, urllib.request
ready=pathlib.Path(sys.argv[1]); deadline=time.monotonic()+60
while not ready.exists():
    if time.monotonic()>deadline: raise TimeoutError('recorder did not become ready')
    time.sleep(.1)
base=sys.argv[2]
def request(method,path,body=None,token=None):
    headers={'Content-Type':'application/json'}
    if token: headers['Authorization']='Bearer '+token
    r=urllib.request.urlopen(urllib.request.Request(base+path,data=json.dumps(body).encode() if body else None,headers=headers,method=method),timeout=10)
    with r:
        data=json.load(r);print(json.dumps({'utc_ns':time.time_ns(),'method':method,'path':path,'status':r.status,'upstream':r.headers.get('X-Upstream-Addr')}),flush=True);return data,r.headers.get('X-Upstream-Addr')
token=request('POST','/auth/login',{'username':'seller','password':'CampusDemo123!'})[0]['accessToken']
product=request('POST','/products',{'categoryId':'textbooks','title':'Measurement probe','description':'real sample','condition':'轻度使用','images':['https://example.invalid/probe.jpg'],'attributes':{'author':'Probe'},'priceCents':12345},token)[0]['id']
end=time.monotonic()+6;upstreams=set();requests=0
while time.monotonic()<end:
    detail,upstream=request('GET','/products/'+product);assert detail['product']['title']=='Measurement probe';upstreams.add(upstream);requests+=1;time.sleep(.02)
assert len(upstreams)==2 and None not in upstreams
print(json.dumps({'result':'PASS','requests':requests,'upstreams':sorted(upstreams),'product':product}),flush=True)
