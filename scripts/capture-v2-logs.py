#!/usr/bin/env python3
"""Capture auxiliary stdout/stderr from exactly one owned experiment project."""
import argparse,gzip,json,pathlib,subprocess,sys
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--project',required=True);p.add_argument('--output',type=pathlib.Path,required=True)
a=p.parse_args()
if not a.project.startswith('campus-v2-'):p.error('dedicated experiment project required')
a.output.mkdir(parents=True,exist_ok=False)
results=[]
try:
    ids=subprocess.check_output(['docker','ps','-aq','--filter','label=com.docker.compose.project='+a.project],text=True,timeout=30).split()
    if not ids:raise RuntimeError('no owned containers remain to capture')
    for identifier in ids:
        try:
            c=json.loads(subprocess.check_output(['docker','inspect',identifier],text=True,timeout=30))[0]
            labels=c['Config']['Labels']
            if labels.get('com.docker.compose.project')!=a.project:raise RuntimeError('project ownership mismatch')
            name=labels['com.docker.compose.service']+'-'+identifier[:12]
            r=subprocess.run(['docker','logs','--timestamps',identifier],capture_output=True,timeout=60)
            for stream,data in [('stdout',r.stdout),('stderr',r.stderr)]:
                with gzip.open(a.output/(name+'-'+stream+'.log.gz'),'wb') as f:f.write(data)
            results.append({'container':identifier,'service':labels['com.docker.compose.service'],'exit':r.returncode,'captured':r.returncode==0})
        except Exception as error:results.append({'container':identifier,'captured':False,'error':str(error)})
except Exception as error:results.append({'captured':False,'error':str(error)})
passed=bool(results) and all(r['captured'] for r in results)
(a.output/'status.json').write_text(json.dumps({'project':a.project,'captured':passed,'containers':results},indent=2)+'\n')
sys.exit(0 if passed else 1)
