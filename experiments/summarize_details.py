#!/usr/bin/env python3
"""Recompute every aggregate from detail-run raw receipts and sampling window."""
import argparse,collections,gzip,json,math,pathlib,statistics
p=argparse.ArgumentParser();p.add_argument('--run-dir',type=pathlib.Path,required=True);a=p.parse_args();root=a.run_dir
window=json.loads((root/'window.json').read_text());pool=json.loads(gzip.decompress((root/'pool.json.gz').read_bytes()));config=json.loads((root/'invocation.json').read_text())
rows=[];late=0
with gzip.open(root/'requests.jsonl.gz','rt') as f:
    for line in f:
        r=json.loads(line)
        if r['receivedUnixNs']<=window['deadlineUnixNs']: rows.append(r)
        else:late+=1
lat=sorted(r['elapsedMs'] for r in rows);errors=sum(not r['valid'] for r in rows);cache=collections.Counter(r.get('telemetry',{}).get('cache','missing') for r in rows);prefix=collections.defaultdict(list)
for r in rows:prefix[r['user']].append(r['sequenceIndex'])
sequence_valid=True
for r in rows:
    position=(r['user']+50*r['sequenceIndex'])%len(pool['ids'])
    sequence_valid &= position==r['poolPosition'] and r['productId']==pool['ids'][position]
def percentile(p):return lat[max(0,math.ceil(len(lat)*p)-1)] if lat else None
resources=json.loads((root/'resources/summary-window.json').read_text())
result={'schemaVersion':1,'formal':config['formal'],'requests':len(rows),'lateReceipts':late,'errors':errors,'errorRate':errors/len(rows) if rows else None,'throughputPerSecond':len(rows)/window['durationSeconds'],'latencyMs':{'median':statistics.median(lat) if lat else None,'p95':percentile(.95),'p99':percentile(.99)},'telemetry':dict(cache,missing=cache['missing'],mongoQueries=sum(r.get('telemetry',{}).get('mongoQueries',0) for r in rows),mongoBsonBytes=sum(r.get('telemetry',{}).get('mongoBsonBytes',0) for r in rows),cacheErrors=sum(r.get('telemetry',{}).get('cacheErrors',0) for r in rows),hitRate=cache['hit']/sum(cache[k] for k in ('hit','miss','fallback')) if sum(cache[k] for k in ('hit','miss','fallback')) else None),'pool':{'count':len(pool['ids']),'sha256':pool['sha256'],'seed':pool['seed'],'pattern':pool['pattern']},'consumedPrefixes':{str(user):{'requests':len(indices),'lastSequenceIndex':max(indices)} for user,indices in sorted(prefix.items())},'sequenceValid':bool(sequence_valid),'resourceCoverage':resources['coverage'],'passed':bool(window['finalized'] and window['readyUsers']==50 and rows and not errors and sequence_valid and resources['coverage']['window']['covered'])}
# Cache fallback is successful business HTTP but a normal-run infrastructure problem.
result['cacheFallbacks']=cache['fallback'];result['normalPassed']=result['passed'] and not cache['fallback'] and not result['telemetry']['cacheErrors'];result['formalPassed']=config['formal'] and window['durationSeconds']==60 and config['fullDataset'] and result['normalPassed']
(root/'summary.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result,indent=2))
