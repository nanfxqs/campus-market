#!/usr/bin/env python3
"""Restore and independently verify three formal sale rounds (A/B/C)."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time

ROOT=Path(__file__).resolve().parents[1]


def read(path): return json.loads(Path(path).read_text())
def write(path,data): Path(path).write_text(json.dumps(data,indent=2)+'\n')


def summarize(output):
    configuration=read(output/'configuration.json')
    rounds=[]
    for number in range(1,configuration['rounds']+1):
        path=output/f'round-{number}'/'result.json'
        rounds.append(read(path) if path.exists() else {'round':number,'passed':False,'reason':'missing result'})
    passed=all(row.get('passed') and row.get('onTimeUniqueSales',0)>=30000 and row.get('consistent') and row.get('resourceCoverage') for row in rounds)
    result={'configuration':configuration,'passed':bool(passed),'rounds':rounds,'failedRounds':[row['round'] for row in rounds if not row.get('passed')],'method':'Every round independently satisfies >=30000 on-time unique new sales, persistence consistency and resource coverage. No average or best-round substitution.'}
    budgets=read(output/'budgets.json') if (output/'budgets.json').exists() else None
    if budgets:
        instances=2 if configuration['configuration']=='C' else 1
        result['resourceBudgets']={'applicationInstances':instances,'applicationPerInstanceCPU':budgets['APP_CPUS'],'applicationPerInstanceMemory':budgets['APP_MEMORY'],'applicationTotalCPU':instances*float(budgets['APP_CPUS']),'proxy':{'cpu':budgets['NGINX_CPUS'],'memory':budgets['NGINX_MEMORY']} if instances==2 else None,'configured':budgets}
    write(output/'summary.json',result)
    return result


def wait_file(path,process,timeout):
    deadline=time.monotonic()+timeout
    while time.monotonic()<deadline:
        if path.exists():
            try: return read(path)
            except json.JSONDecodeError: pass
        if process.poll() is not None: raise RuntimeError(f'process exited {process.returncode} before {path.name}')
        time.sleep(.1)
    raise RuntimeError(f'timed out waiting for {path}')


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--configuration',choices=['A','B','C'])
    parser.add_argument('--project')
    parser.add_argument('--database')
    parser.add_argument('--port',type=int,default=8088)
    parser.add_argument('--base-time')
    parser.add_argument('--random-seed',type=int,default=42)
    parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--expected-fingerprint',help='SHA256 of the exact full persisted fingerprint output; checked before every measured round')
    parser.add_argument('--summarize',action='store_true')
    parser.add_argument('--idle-seconds',type=int,default=10)
    arguments=parser.parse_args()
    output=arguments.output.resolve()
    if arguments.summarize:
        result=summarize(output);print(json.dumps(result,indent=2));return 0 if result['passed'] else 1
    if not all([arguments.configuration,arguments.project,arguments.database,arguments.base_time]): parser.error('configuration/project/database/explicit base-time required')
    if not arguments.project.startswith('campus-v2-') or not arguments.database.startswith('campus_v2_'): parser.error('dedicated experiment prefixes required')
    if arguments.idle_seconds<5: parser.error('at least5 idle samples required')
    if output.exists() and any(output.iterdir()): parser.error('new output directory required; preserve previous attempts')
    output.mkdir(parents=True,exist_ok=True)
    config={'configuration':arguments.configuration,'rounds':3,'project':arguments.project,'database':arguments.database,'port':arguments.port,'baseTime':arguments.base_time,'randomSeed':arguments.random_seed,'cutoffSeconds':600,'users':50,'targetProducts':30000,'idleSeconds':arguments.idle_seconds,'git':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'worktree':subprocess.check_output(['git','status','--short'],cwd=ROOT,text=True)}
    write(output/'configuration.json',config)
    environment={**os.environ,'COMPOSE_PROJECT_NAME':arguments.project,'DB_NAME':arguments.database,'API_PORT':str(arguments.port),'BASE_TIME':arguments.base_time,'RANDOM_SEED':str(arguments.random_seed)}
    # Capture budgets once and reuse them unmodified across all three rounds.
    for name,default in {'APP_CPUS':'2','APP_MEMORY':'512m','MONGO_CPUS':'4','MONGO_MEMORY':'8g','REDIS_CPUS':'1','REDIS_MEMORY':'256m','REDIS_MAXMEMORY':'128mb','REDIS_POLICY':'allkeys-lru','NGINX_CPUS':'1','NGINX_MEMORY':'128m'}.items(): environment[name]=os.environ.get(name,default)
    write(output/'budgets.json',{name:environment[name] for name in ['APP_CPUS','APP_MEMORY','MONGO_CPUS','MONGO_MEMORY','REDIS_CPUS','REDIS_MEMORY','REDIS_MAXMEMORY','REDIS_POLICY','NGINX_CPUS','NGINX_MEMORY']})
    dc=['docker','compose','-f',str(ROOT/'compose.v2.yaml'),'--profile','A','--profile','B','--profile','C']
    with open(output/'dependencies.log','w') as log:
        subprocess.run(['uv','sync','--frozen','--project',str(ROOT/'experiments')],cwd=ROOT,stdout=log,stderr=subprocess.STDOUT,check=True)
    initial_fingerprint=None
    for number in range(1,4):
        round_dir=output/f'round-{number}';round_dir.mkdir()
        recorder=None;client=None
        result={'round':number,'passed':False,'configuration':arguments.configuration}
        try:
            restore=round_dir/'restore';environment['RESULT_DIR']=str(restore)
            with open(round_dir/'restore.log','w') as log:
                subprocess.run([str(ROOT/'scripts/restore-v2.sh'),arguments.configuration,'--reset'],cwd=ROOT,env=environment,stdout=log,stderr=subprocess.STDOUT,check=True)
            subprocess.run([*dc,'config'],cwd=ROOT,env=environment,stdout=open(round_dir/'compose-resolved.yaml','w'),check=True)
            # Full logical persisted document hashes exclude only BSON field order.
            hashes=subprocess.check_output([*dc,'exec','-T','mongo','mongosh','--quiet',arguments.database,'--eval',(ROOT/'scripts/fingerprint-seed.js').read_text()],cwd=ROOT,env=environment,text=True)
            (round_dir/'initial-fingerprints.txt').write_text(hashes)
            if arguments.expected_fingerprint and hashlib.sha256(hashes.encode()).hexdigest()!=arguments.expected_fingerprint: raise RuntimeError('initial persisted document fingerprint differs from reference configuration A')
            if initial_fingerprint is None: initial_fingerprint=hashes
            if hashes!=initial_fingerprint: raise RuntimeError('initial persisted document fingerprints differ between rounds')
            locust=str(ROOT/'experiments/.venv/bin/locust')
            with open(round_dir/'idle.log','w') as log:
                subprocess.run([sys.executable,str(ROOT/'scripts/measure-v2.py'),'--project',arguments.project,'--database',arguments.database,'--phase','idle','--seconds',str(arguments.idle_seconds),'--output',str(round_dir/'idle'),'--git-base',config['git'],'--locust',locust],cwd=ROOT,env=environment,stdout=log,stderr=subprocess.STDOUT,check=True)
            sale=round_dir/'sale';gate=round_dir/'collector-ready'
            client_log=open(round_dir/'sale-runner.log','w')
            client=subprocess.Popen([sys.executable,str(ROOT/'scripts/run-sale-load.py'),'--formal','--configuration',arguments.configuration,'--project',arguments.project,'--database',arguments.database,'--api',f'http://127.0.0.1:{arguments.port}','--manifest',str(restore/'manifest.json'),'--output',str(sale),'--start-gate',str(gate)],cwd=ROOT,env=environment,stdout=client_log,stderr=subprocess.STDOUT)
            pid=wait_file(sale/'client-pid.json',client,300)['pid']
            resource_log=open(round_dir/'resources.log','w')
            recorder=subprocess.Popen([sys.executable,str(ROOT/'scripts/measure-v2.py'),'--project',arguments.project,'--database',arguments.database,'--phase','sale-load','--client-pid',str(pid),'--output',str(round_dir/'resources'),'--git-base',config['git'],'--locust',locust],cwd=ROOT,env=environment,stdout=resource_log,stderr=subprocess.STDOUT)
            wait_file(round_dir/'resources/ready.json',recorder,240)
            gate.touch()
            client_code=client.wait(timeout=1050)
            client_log.close()
            # Samples span actual end; storage-after occurs only after settled DB
            # reconciliation, while window summaries exclude settlement/prep.
            time.sleep(2)
            recorder.send_signal(signal.SIGTERM)
            recorder_code=recorder.wait(timeout=240)
            resource_log.close()
            subprocess.run([sys.executable,str(ROOT/'scripts/measure-v2.py'),'--output',str(round_dir/'resources'),'--summarize','--window',str(sale/'window.json')],cwd=ROOT,env=environment,check=True)
            summary=read(sale/'summary.json');measurement=read(round_dir/'resources/summary-window.json')
            coverage=measurement['coverage']['window']['covered'] and measurement['sample_count']>0
            result.update(onTimeUniqueSales=summary['client']['onTimeUniqueSales'],consistent=summary['database']['consistent'],formalPassed=summary['formalPassed'],resourceCoverage=coverage,measuredWindowSeconds=summary['measuredWindowSeconds'],configuredCutoffSeconds=600,clientExit=client_code,recorderExit=recorder_code,initialFingerprintSHA256=hashlib.sha256(hashes.encode()).hexdigest(),client=summary['client'],metrics=summary['metrics'],prepare=read(restore/'preparation.json'))
            result['passed']=bool(summary['formalPassed'] and coverage and client_code==0 and recorder_code==0)
        except Exception as error:
            result['reason']=str(error)
        finally:
            if client and client.poll() is None: client.terminate();client.wait(timeout=60)
            if recorder and recorder.poll() is None: recorder.send_signal(signal.SIGTERM);recorder.wait(timeout=240)
            write(round_dir/'result.json',result)
            manifest=round_dir/'restore/manifest.json'
            if manifest.exists():
                with manifest.open('rb') as source,gzip.open(str(manifest)+'.gz','wb',compresslevel=3) as destination: destination.write(source.read())
                manifest.unlink()
            with open(round_dir/'stop.log','w') as log: subprocess.run([*dc,'stop'],cwd=ROOT,env=environment,stdout=log,stderr=subprocess.STDOUT)
        print(json.dumps(result),flush=True)
    result=summarize(output)
    print(json.dumps({'passed':result['passed'],'failedRounds':result['failedRounds'],'output':str(output)},indent=2))
    return 0 if result['passed'] else 1

if __name__=='__main__': sys.exit(main())
