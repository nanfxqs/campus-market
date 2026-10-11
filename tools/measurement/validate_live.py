#!/usr/bin/env python3
"""Run real HTTP client and recorder together; requires caller-owned ready C stack."""
import argparse, pathlib, subprocess, sys
p=argparse.ArgumentParser();p.add_argument('--project',required=True);p.add_argument('--database',required=True);p.add_argument('--url',required=True);p.add_argument('--output',required=True);p.add_argument('--locust',default='locust');a=p.parse_args()
out=pathlib.Path(a.output)
# Recorder owns directory creation; keep HTTP temporary log next to it.
log=out.parent/(out.name+'-http.jsonl');log.parent.mkdir(parents=True,exist_ok=True)
with log.open('w') as f:
    client=subprocess.Popen([sys.executable,'tools/measurement/http_probe.py',str(out/'ready.json'),a.url],stdout=f,stderr=f)
    try:
        collector=subprocess.run([sys.executable,'scripts/measure-v2.py','--project',a.project,'--database',a.database,'--output',str(out),'--phase','detail-load','--seconds','10','--client-pid',str(client.pid),'--locust',a.locust,'--git-base','integration/v2'])
        assert collector.returncode==0
        assert client.wait(timeout=30)==0,log
    finally:
        if client.poll() is None: client.terminate();client.wait(timeout=10)
subprocess.run([sys.executable,'tools/measurement/verify.py',str(out),'C'],check=True)
