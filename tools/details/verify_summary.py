#!/usr/bin/env python3
"""Literal mathematical fixture at the summary CLI seam; not HTTP evidence."""
import gzip,json,pathlib,subprocess,tempfile
with tempfile.TemporaryDirectory(prefix='detail-summary-fixture-') as tmp:
 p=pathlib.Path(tmp);(p/'resources').mkdir()
 pool={'ids':['a','b','c'],'sha256':'fixture','seed':42,'pattern':'dispersed'}
 (p/'pool.json.gz').write_bytes(gzip.compress(json.dumps(pool).encode()))
 (p/'window.json').write_text(json.dumps({'startUnixNs':100,'deadlineUnixNs':1000,'durationSeconds':60,'finalized':True,'readyUsers':50}))
 (p/'invocation.json').write_text(json.dumps({'formal':False,'fullDataset':False}))
 (p/'resources/summary-window.json').write_text(json.dumps({'coverage':{'window':{'covered':True}}}))
 rows=[{'user':0,'sequenceIndex':0,'poolPosition':0,'productId':'a','receivedUnixNs':200,'elapsedMs':10,'valid':True,'telemetry':{'cache':'miss','mongoQueries':5,'mongoBsonBytes':100}}, {'user':0,'sequenceIndex':1,'poolPosition':2,'productId':'c','receivedUnixNs':300,'elapsedMs':20,'valid':True,'telemetry':{'cache':'hit','mongoQueries':4,'mongoBsonBytes':80}}, {'user':0,'sequenceIndex':2,'poolPosition':1,'productId':'b','receivedUnixNs':400,'elapsedMs':30,'valid':True,'telemetry':{'cache':'hit','mongoQueries':4,'mongoBsonBytes':80}}, {'user':0,'sequenceIndex':3,'poolPosition':0,'productId':'a','receivedUnixNs':1001,'elapsedMs':999,'valid':True,'telemetry':{'cache':'hit','mongoQueries':4,'mongoBsonBytes':80}}]
 with gzip.open(p/'requests.jsonl.gz','wt') as f:
  for row in rows:f.write(json.dumps(row)+'\n')
 subprocess.run(['python3','experiments/summarize_details.py','--run-dir',str(p)],check=True,stdout=subprocess.DEVNULL)
 s=json.loads((p/'summary.json').read_text())
 assert s['requests']==3 and s['lateReceipts']==1 and s['latencyMs']=={'median':20,'p95':30,'p99':30}
 assert s['telemetry']['mongoQueries']==13 and s['telemetry']['mongoBsonBytes']==260 and s['telemetry']['hitRate']==2/3
 assert s['sequenceValid'] and not s['formalPassed']
 print('PASS literal percentile, telemetry sums, late cutoff and deterministic sequence')
