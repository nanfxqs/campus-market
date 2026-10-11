#!/usr/bin/env python3
"""Verify the public measurement artifacts of a real Docker/HTTP run."""
import json, pathlib, sys
p = pathlib.Path(sys.argv[1])
summary = json.loads((p/'summary.json').read_text())
samples = [json.loads(line) for line in (p/'samples.jsonl').read_text().splitlines()]
assert len(samples) >= 3, 'must cover a multi-second observation'
assert all(0.5 <= b['monotonic']-a['monotonic'] <= 2 for a,b in zip(samples,samples[1:])), 'one-second coverage'
for component in ('application','mongodb','search'):
    found = [x for s in samples for x in s['processes'] if x['component']==component]
    assert found and all(x['rss_bytes']>0 for x in found), component
assert summary['components']['redis']['status'] in ('absent','measured')
if summary['components']['redis']['status']=='absent': assert summary['components']['redis']['rss_bytes']['mean'] is None
if len(sys.argv)>2 and sys.argv[2]=='C':
    assert summary['components']['nginx']['status']=='measured'
    assert {'api-c1','api-c2'} <= {x.get('service') for x in samples[0]['processes'] if x['component']=='application'}
else:
    assert summary['components']['nginx']['status']=='absent'
    assert summary['components']['nginx']['rss_bytes']['mean'] is None
assert (p/'storage-before.json').exists() and (p/'storage-after.json').exists()
assert any('search' in m['destination'] or 'mongot' in m['destination'] for m in json.loads((p/'environment.json').read_text())['mounts'])
assert summary['collector']['cpu_seconds']>=0
assert summary['host']['cpu_percent_of_all_cores']['samples']>=2
assert all(c['cpu_percent']['samples']>=2 for c in summary['containers'].values())
assert all(m['allocated_bytes'] is not None for m in json.loads((p/'storage-after.json').read_text())['mounts'])
print('PASS real process separation, sample coverage, absent components, storage and overhead')
