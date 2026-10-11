#!/usr/bin/env python3
"""Single-client eight-feature HTTP timing and separate ten-sample TTL evidence."""
import argparse
import datetime
import json
import math
import pathlib
import re
import statistics
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument('--project', required=True)
parser.add_argument('--database', required=True)
parser.add_argument('--url', required=True)
parser.add_argument('--output', required=True)
parser.add_argument('--samples', type=int, default=100)
parser.add_argument('--ttl-timeout', type=float, default=180)
args = parser.parse_args()
if not re.fullmatch(r'campus-v2-[a-z0-9-]+', args.project) or not re.fullmatch(r'campus_v2_[A-Za-z0-9_]+', args.database):
    parser.error('explicit dedicated V2 project/database required')
if args.samples < 1 or args.ttl_timeout <= 0:
    parser.error('positive samples and TTL timeout required')
out = pathlib.Path(args.output)
out.mkdir(parents=True, exist_ok=False)
raw = (out / 'http.jsonl').open('w')
compose = ['docker', 'compose', '-p', args.project, '-f', 'compose.v2.yaml']
run_id = 'feature22' + str(time.time_ns())

def external(js, label):
    result = subprocess.run(compose + ['exec', '-T', 'mongo', 'mongosh', '--quiet', args.database, '--eval', js], text=True, capture_output=True)
    (out / (label + '.txt')).write_text(result.stdout + result.stderr)
    if result.returncode:
        raise RuntimeError('external Mongo evidence failed: ' + label)
    return json.loads(result.stdout)

def request(method, path, body=None, token=None, expected=200, phase='preparation', feature=None, sample=None):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(args.url + path, data=None if body is None else json.dumps(body).encode(), headers=headers, method=method)
    started = time.monotonic_ns()
    try:
        response = urllib.request.urlopen(req, timeout=30)
    except urllib.error.HTTPError as error:
        response = error
    except Exception as error:
        raw.write(json.dumps({'phase': phase, 'feature': feature, 'sample': sample, 'method': method, 'path': path, 'error': str(error), 'elapsed_ms': (time.monotonic_ns()-started)/1e6})+'\n')
        raw.flush()
        raise
    with response:
        payload = response.read()
        elapsed = (time.monotonic_ns()-started)/1e6
        data = json.loads(payload)
        record = {'phase': phase, 'feature': feature, 'sample': sample, 'method': method, 'path': path, 'status': response.status, 'expected_status': expected, 'elapsed_ms': elapsed, 'response_bytes': len(payload), 'upstream': response.headers.get('X-Upstream-Addr')}
        # Login tokens never enter artifacts. Write/input cases are explicit below.
        if path != '/auth/login':
            record['response'] = data
        if body is not None and path != '/auth/login':
            record['input'] = body
        raw.write(json.dumps(record, ensure_ascii=False)+'\n')
        raw.flush()
        if response.status != expected:
            raise AssertionError((method, path, response.status, data))
        return data, elapsed

def http(*positional, **keywords):
    return request(*positional, **keywords)[0]

def publish(title, phase='preparation', feature=None, sample=None):
    return request('POST', '/products', {'categoryId': 'textbooks', 'title': title, 'description': 'fixed functionality measurement', 'condition': '轻度使用', 'images': ['https://example.invalid/feature22.png'], 'attributes': {'author': 'V2'}, 'priceCents': 12345}, token=token, expected=201, phase=phase, feature=feature, sample=sample)

def search_wait(query, expected):
    deadline = time.monotonic()+180
    last = None
    while time.monotonic()<deadline:
        last = http('GET', '/search?q='+urllib.parse.quote(query))
        if last['total']==expected:
            return last
        time.sleep(.5)
    raise AssertionError(('Search readiness timeout', query, expected, last))

def count_stats(data):
    return sum(item['count'] for item in data['items'])

# Read-only original state: timings never begin on a partially imported dataset.
initial = external('''const m=db.seed_metadata.findOne({_id:'dataset'});print(JSON.stringify({mode:m&&m.mode,metadata:m&&m.counts,users:db.users.countDocuments({}),products:db.products.countDocuments({}),sold:db.products.countDocuments({sold:true}),listings:db.listings.countDocuments({}),transactions:db.transactions.countDocuments({}),priceChanges:db.priceChanges.countDocuments({}),bicycles:db.products.countDocuments({categoryId:'bicycles',sold:false,expiresAt:{$gt:new Date()}}),buyerActive:db.products.countDocuments({sellerId:'buyer',sold:false,expiresAt:{$gt:new Date()}})}));''', 'initial-scale')
assert initial['mode']=='v2' and initial['users']==10000 and initial['products']==230000 and initial['sold']==180000 and initial['listings']==50000 and initial['transactions']==300000 and initial['priceChanges']==575000, initial
configuration = subprocess.check_output(compose+['config'], text=True)
(out/'compose.yaml').write_text(configuration)
for label, command in [('machine',['uname','-a']),('cpu',['lscpu']),('memory',['free','-b']),('disk',['df','-B1','.']),('docker',['docker','version']),('compose-version',['docker','compose','version']),('images',compose+['images']),('git',['git','rev-parse','HEAD']),('git-status',['git','status','--short'])]:
    result=subprocess.run(command,text=True,capture_output=True)
    (out/(label+'.txt')).write_text(result.stdout+result.stderr)
parameters = {'run_id':run_id,'started_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'project':args.project,'database':args.database,'url':args.url,'samples_per_feature':args.samples,'formal':args.samples==100,'single_client':True,'warmups_per_feature':1,'percentile_method':'nearest rank ceil(p*n), milliseconds','initial_scale':initial,'read_cases':{'category':'bicycles','search_query':'单车','detail_product':'000000000000000000000001','seller':'buyer'},'write_price_cents':12345,'sale_price_cents':10001,'expiry_samples':10,'ttl_poll_seconds':2,'ttl_timeout_seconds':args.ttl_timeout}
(out/'parameters.json').write_text(json.dumps(parameters,ensure_ascii=False,indent=2))
token = http('POST','/auth/login',{'username':'seller','password':'CampusDemo123!'})['accessToken']
ready = search_wait('单车',initial['bicycles'])
assert len(ready['results'])==20 and all(row['highlights'] and row['matchedFields'] for row in ready['results'])

# Every expiry fixture gets a real price history; Search readiness precedes expiry.
expiry_title=run_id+'expiry'
fixtures=[]
for index in range(10):
    product,_=publish(expiry_title)
    product_id=product['id']
    http('PATCH','/products/'+product_id+'/price',{'priceCents':23456},token=token)
    fixtures.append(product_id)
search_wait(expiry_title,10)
home_before=http('GET','/users/seller/home')
stats_before=count_stats(http('GET','/categories/textbooks/condition-counts'))
expiry = external('const ids='+json.dumps(fixtures)+''';const expiry=new Date(Date.now()+2000);for(const id of ids){db.products.updateOne({_id:id},{$set:{expiresAt:expiry}});db.listings.updateOne({_id:id},{$set:{expiresAt:expiry}});}print(JSON.stringify({expires_at:expiry.toISOString(),fixtures:ids,initial_listing_count:db.listings.countDocuments({_id:{$in:ids}})}));''','expiry-fixture')
assert expiry['initial_listing_count']==10
expires_at=datetime.datetime.fromisoformat(expiry['expires_at'].replace('Z','+00:00'))
while datetime.datetime.now(datetime.timezone.utc)<expires_at:
    time.sleep(.05)
for product_id in fixtures:
    detail=http('GET','/products/'+product_id)
    assert not detail['available'] and not detail['product']['sold'] and len(detail['priceHistory'])==1 and detail['product']['images']==['https://example.invalid/feature22.png']
    result=http('POST','/products/'+product_id+'/sale',{'buyerId':'buyer','priceCents':10001,'idempotencyKey':'expired-'+product_id},token=token,expected=409)
    assert result['code']=='product_expired' and not result['success'], result
assert http('GET','/search?q='+expiry_title)['total']==0
home_after=http('GET','/users/seller/home')
assert home_after['currentOnSaleCount']==home_before['currentOnSaleCount']-10
assert not set(fixtures).intersection(item['id'] for item in home_after['items'])
assert count_stats(http('GET','/categories/textbooks/condition-counts'))==stats_before-10
browse=http('GET','/products?categoryId=textbooks')
assert not set(fixtures).intersection(item['id'] for item in browse['items'])
(out/'instant-expiry.json').write_text(json.dumps({'result':'PASS','expires_at':expiry['expires_at'],'fixtures':fixtures,'search_total':0,'home_count_decrement':10,'statistics_count_decrement':10,'rejected_sales':10,'archives_and_history_retained':10},indent=2))

# TTL observation is separate preparation; no wait enters any HTTP sample.
remaining=set(fixtures)
previous_delay=0
observations=[]
deadline=time.monotonic()+args.ttl_timeout
while remaining and time.monotonic()<deadline:
    evidence=external('const ids='+json.dumps(fixtures)+''';print(JSON.stringify({observed_at:new Date().toISOString(),remaining:db.listings.find({_id:{$in:ids}},{_id:1}).toArray().map(x=>x._id)}));''','ttl-last-poll')
    observed=datetime.datetime.fromisoformat(evidence['observed_at'].replace('Z','+00:00'))
    delay=max(0,(observed-expires_at).total_seconds())
    now_remaining=set(evidence['remaining'])
    for product_id in sorted(remaining-now_remaining):
        observations.append({'product_id':product_id,'deletion_delay_lower_seconds':previous_delay,'deletion_delay_upper_seconds':delay,'observed_at':evidence['observed_at']})
    remaining=now_remaining
    previous_delay=delay
    with (out/'ttl-polls.jsonl').open('a') as stream:stream.write(json.dumps(evidence)+'\n')
    if remaining:time.sleep(2)
(out/'ttl-observations.json').write_text(json.dumps({'result':'PASS' if not remaining else 'FAIL','timeout_seconds':args.ttl_timeout,'observations':observations,'remaining':sorted(remaining),'meaning':'interval-censored observed cleanup delay, not a TTL SLA'},indent=2))
assert not remaining,'TTL bounded observation timed out; evidence retained'
retained=external('const ids='+json.dumps(fixtures)+''';print(JSON.stringify({archives:db.products.countDocuments({_id:{$in:ids},sold:false}),history:db.priceChanges.countDocuments({productId:{$in:ids}}),listings:db.listings.countDocuments({_id:{$in:ids}}),successes:db.transactions.countDocuments({productId:{$in:ids},success:true})}));''','post-ttl-retention')
assert retained=={'archives':10,'history':10,'listings':0,'successes':0},retained

features=['publish','category_browse','search','detail','seller_home','sale','automatic_delist','category_statistics']
summary={}
for feature in features:
    # Sale's independent fresh fixtures are built outside the measured request.
    products=[]
    if feature=='sale':
        for index in range(args.samples+1):products.append(publish(run_id+'sale'+str(index))[0]['id'])
    durations=[]
    failures=[]
    for index in range(args.samples+1):
        phase='warmup' if index==0 else 'measurement'
        keywords={'phase':phase,'feature':feature,'sample':index}
        started=time.monotonic_ns()
        try:
            if feature=='publish':
                data,elapsed=publish(run_id+'publish'+str(index),**keywords)
                assert data['title']==run_id+'publish'+str(index) and data['priceCents']==12345 and not data['sold']
            elif feature=='category_browse':
                data,elapsed=request('GET','/products?categoryId=bicycles',**keywords)
                assert len(data['items'])==20 and data['nextCursor'] and all(x['categoryId']=='bicycles' and not x['sold'] for x in data['items'])
            elif feature=='search':
                data,elapsed=request('GET','/search?q='+urllib.parse.quote('单车'),**keywords)
                assert data['total']==initial['bicycles'] and len(data['results'])==20 and all(x['highlights'] and x['matchedFields'] for x in data['results'])
            elif feature=='detail':
                data,elapsed=request('GET','/products/000000000000000000000001',**keywords)
                assert data['available'] and not data['product']['sold'] and data['product']['id']=='000000000000000000000001' and data['priceHistory']
            elif feature=='seller_home':
                data,elapsed=request('GET','/users/buyer/home',**keywords)
                assert data['currentOnSaleCount']==initial['buyerActive'] and data['profile']['id']=='buyer'
            elif feature=='sale':
                data,elapsed=request('POST','/products/'+products[index]+'/sale',{'buyerId':'buyer','priceCents':10001,'idempotencyKey':run_id+'sale'+str(index)},token=token,**keywords)
                assert data['success'] and data['productId']==products[index] and data['priceCents']==10001
            elif feature=='automatic_delist':
                data,elapsed=request('POST','/products/'+fixtures[index%10]+'/sale',{'buyerId':'buyer','priceCents':10001,'idempotencyKey':run_id+'reject'+str(index)},token=token,expected=409,**keywords)
                assert not data['success']
            elif feature=='category_statistics':
                data,elapsed=request('GET','/categories/bicycles/condition-counts',**keywords)
                assert len(data['items'])==4 and count_stats(data)==initial['bicycles']
            if index:durations.append(elapsed)
        except Exception as error:
            if not index:raise
            failures.append({'sample':index,'error':str(error)})
            durations.append((time.monotonic_ns()-started)/1e6)
    ordered=sorted(durations)
    def percentile(p):return ordered[math.ceil(p*len(ordered))-1]
    summary[feature]={'samples':args.samples,'successes':args.samples-len(failures),'errors':len(failures),'error_rate':len(failures)/args.samples,'median_ms':statistics.median(durations),'p95_ms':percentile(.95),'p99_ms':percentile(.99),'all_response_latency_scope':'all measured attempts, including failures','failures':failures}
    (out/'summary.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2))
    print(json.dumps({'feature':feature,**summary[feature]}),flush=True)
final=external("print(JSON.stringify({users:db.users.countDocuments({}),products:db.products.countDocuments({}),sold:db.products.countDocuments({sold:true}),listings:db.listings.countDocuments({}),transactions:db.transactions.countDocuments({}),priceChanges:db.priceChanges.countDocuments({})}));",'final-scale')
(out/'result.json').write_text(json.dumps({'result':'PASS' if all(x['errors']==0 for x in summary.values()) else 'FAIL','formal':args.samples==100,'finished_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'final_scale':final},indent=2))
raw.close()
assert all(x['errors']==0 for x in summary.values()),'HTTP/result validation errors retained'
