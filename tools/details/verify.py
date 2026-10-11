#!/usr/bin/env python3
"""Check real detail-run artifacts at the CLI/HTTP boundary."""
import gzip,json,pathlib,sys
p=pathlib.Path(sys.argv[1]);w=json.loads((p/'window.json').read_text());s=json.loads((p/'summary.json').read_text())
assert w['readyUsers']==50 and w['finalized']
assert s['requests']>0 and s['errors']==0
assert s['telemetry']['missing']==0
assert s['resourceCoverage']['window']['covered']
with gzip.open(p/'requests.jsonl.gz','rt') as f:
 rows=[json.loads(line) for line in f]
assert len({r['user'] for r in rows})==50
assert all(r['sequenceIndex']>=0 and r['status']==200 for r in rows)
assert s['pool']['count']==len(json.loads(gzip.decompress((p/'pool.json.gz').read_bytes()))['ids'])
print('PASS 50 ready users, actual HTTP telemetry, sequence indices and resource window coverage')
