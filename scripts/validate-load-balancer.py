#!/usr/bin/env python3
"""Real C HTTP distribution, shared sessions, transaction and external DB evidence."""
import argparse, concurrent.futures, json, subprocess, threading, time, urllib.request, urllib.error
p = argparse.ArgumentParser()
p.add_argument('--project', required=True)
p.add_argument('--url', required=True)
p.add_argument('--database', required=True)
a = p.parse_args()
if not a.project.startswith('campus-v2-') or not a.database.startswith('campus_v2_'):
    p.error('use a dedicated campus-v2-* project and campus_v2_* database')
compose = ['docker', 'compose', '-p', a.project, '-f', 'compose.v2.yaml']
seen = set()
lock = threading.Lock()
def http(method, path, body=None, token=None, status=200):
    headers = {'Content-Type': 'application/json'}
    if token: headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(a.url + path, data=None if body is None else json.dumps(body).encode(), headers=headers, method=method)
    start = time.monotonic()
    try: response = urllib.request.urlopen(req, timeout=20)
    except urllib.error.HTTPError as e: response = e
    with response as r:
        result = json.load(r)
        upstream = r.headers.get('X-Upstream-Addr')
        with lock:
            seen.add(upstream)
            print(json.dumps({'method': method, 'path': path, 'status': r.status, 'upstream': upstream, 'elapsed_ms': (time.monotonic()-start)*1000, 'result': result if path.endswith('/sale') else None}), flush=True)
        if status is not None: assert r.status == status, (r.status, result)
        return result, upstream, r.status

def publish(token):
    result, _, _ = http('POST', '/products', {'categoryId':'textbooks', 'title':'Cross instance validation', 'description':'Round robin', 'condition':'轻度使用', 'images':['https://example.invalid/lb.png'], 'attributes':{'author':'LB'}, 'priceCents':12345}, token, 201)
    return result['id']

print(json.dumps({'started_at':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()), 'project':a.project,'database':a.database,'url':a.url}), flush=True)
login, issuer, _ = http('POST','/auth/login',{'username':'seller','password':'CampusDemo123!'})
token=login['accessToken']
profile, verifier, _ = http('GET','/users/me',token=token)
assert issuer and verifier and issuer != verifier, 'token must be verified by the other instance'
before=profile['completedSales']
http('GET','/users/me',status=401)
product=publish(token)
body={'buyerId':'buyer','priceCents':10001,'idempotencyKey':'lb-retry-'+product}
first, first_node, _ = http('POST','/products/'+product+'/sale',body,token)
replay, replay_node, _ = http('POST','/products/'+product+'/sale',body,token)
assert first_node != replay_node and first == replay
http('POST','/products/'+product+'/sale',dict(body,priceCents=10002),token,409)
race=publish(token)
barrier=threading.Barrier(20)
def compete(i):
    barrier.wait()
    return http('POST','/products/'+race+'/sale',{'buyerId':'buyer','priceCents':10001,'idempotencyKey':'lb-race-'+race+'-'+str(i)},token,None)
with concurrent.futures.ThreadPoolExecutor(max_workers=20) as pool: results=list(pool.map(compete,range(20)))
assert sum(r[2]==200 for r in results)==1 and sum(r[2]==409 for r in results)==19
assert len({r[1] for r in results})==2
profile, _, _ = http('GET','/users/me',token=token)
assert profile['completedSales']==before+2
for product_id in (product,race):
    detail,_,_=http('GET','/products/'+product_id)
    assert detail['product']['sold'] and not detail['available']
assert len(seen)==2 and None not in seen
# Read-only, independently check all coupled durable facts and seller count.
js='''const ids=IDS; const rows=ids.map(id=>({id:String(id),sold:db.products.findOne({_id:id}).sold,successes:db.transactions.countDocuments({productId:id,success:true}),listings:db.listings.countDocuments({_id:id})})); const count=db.users.findOne({_id:"seller"}).completedSales; print(JSON.stringify({rows,sellerCompletedSales:count})); if(rows.some(x=>!x.sold||x.successes!==1||x.listings!==0)||count!==EXPECTED) quit(1);'''.replace('IDS',json.dumps([product,race])).replace('EXPECTED',str(before+2))
r=subprocess.run(compose+['exec','-T','mongo','mongosh','--quiet',a.database,'--eval',js],text=True,capture_output=True)
print(json.dumps({'database_evidence':r.stdout,'stderr':r.stderr,'exit':r.returncode}),flush=True)
assert r.returncode==0
print(json.dumps({'result':'PASS','upstreams':sorted(seen),'issuer':issuer,'verifier':verifier,'same_key_nodes':[first_node,replay_node],'race':{'success':1,'conflict':19}}),flush=True)
