#!/usr/bin/env python3
"""Run real Locust sale load, save evidence, and reconcile MongoDB externally."""
import argparse
import math
from datetime import datetime,timezone
import gzip
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import urllib.request

ROOT=Path(__file__).resolve().parents[1]


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--configuration',choices=['A','B','C'],required=True)
    parser.add_argument('--project',required=True)
    parser.add_argument('--database',required=True)
    parser.add_argument('--api',required=True)
    parser.add_argument('--manifest',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--start-gate',type=Path,help='optional collector readiness file; Locust waits before login/spawn')
    parser.add_argument('--formal',action='store_true')
    parser.add_argument('--duration',type=float,default=600)
    parser.add_argument('--products',type=int,default=30000)
    parser.add_argument('--timeout',type=float,default=25)
    parser.add_argument('--retries',type=int,default=2)
    parser.add_argument('--settle',type=float,default=25)
    options=parser.parse_args()
    if options.duration<=0 or not 50<=options.products<=30000 or not 0<options.timeout<=60 or options.retries<0 or options.settle<0:
        parser.error('positive window/timeout, 50..30000 products, and nonnegative retries/settle required')
    if options.formal and (options.duration!=600 or options.products!=30000 or options.settle<20):
        parser.error('formal means exactly50 users,600 seconds,30000 products and >=20 seconds settlement')
    if not options.project.startswith('campus-v2-') or not options.database.startswith('campus_v2_'):
        parser.error('explicit isolated campus-v2- project and campus_v2_ database required')
    output=options.output.resolve()
    output.mkdir(parents=True,exist_ok=True)
    if any(output.iterdir()): parser.error('output directory must be empty; preserve earlier runs')
    manifest_path=options.manifest.resolve()
    manifest=json.loads(manifest_path.read_text())
    if manifest.get('mode')!='v2' or manifest.get('database')!=options.database or len(manifest['products'])!=30000:
        parser.error('manifest must identify this full V2 database and30000 dedicated products')
    products=manifest['products']
    if len({row['productId'] for row in products})!=30000 or len({row['idempotencyKey'] for row in products})!=30000:
        parser.error('product IDs and idempotency keys must be unique')
    accounts={account['id']:account for account in manifest['accounts']}
    for row in products:
        if row['sellerId']==row['buyerId'] or row['sellerId'] not in accounts or row['buyerId'] not in accounts or row['priceCents']<1:
            parser.error('illegal seller/buyer/price in manifest')
        if datetime.fromisoformat(row['expiresAt'].replace('Z','+00:00')).timestamp()<=time.time()+options.duration+120:
            parser.error('product eligibility does not cover preparation and load')
    selected=products[:options.products]
    if len({row['sellerId'] for row in selected})!=50:
        parser.error('selected products must cover all50 registered sellers')
    environment={**os.environ,'COMPOSE_PROJECT_NAME':options.project,'DB_NAME':options.database}
    dc=['docker','compose','-f',str(ROOT/'compose.v2.yaml'),'--profile',options.configuration]
    prepare=time.monotonic()
    (output/'invocation.json').write_text(json.dumps({'schemaVersion':1,**vars(options),'manifest':str(manifest_path),'output':str(output),'sourceHashes':{str(path.relative_to(ROOT)):hashlib.sha256(path.read_bytes()).hexdigest() for path in [ROOT/'scripts/run-sale-load.py',ROOT/'scripts/snapshot-sale-load.js',ROOT/'experiments/sale_locust.py',ROOT/'experiments/summarize_sales.py',ROOT/'experiments/uv.lock']},'locustVersion':subprocess.check_output(['uv','run','--frozen','--project',str(ROOT/'experiments'),'locust','--version'],cwd=ROOT,text=True).strip(),'startedUTC':datetime.now(timezone.utc).isoformat(),'git':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'worktree':subprocess.check_output(['git','status','--short'],cwd=ROOT,text=True)},default=str,indent=2)+'\n')
    subprocess.run([*dc,'cp',str(manifest_path),'mongo:/tmp/v2-load-manifest.json'],cwd=ROOT,env=environment,check=True)
    script=(ROOT/'scripts/snapshot-sale-load.js').read_text()
    def snapshot(stage):
        process=subprocess.run([*dc,'exec','-T','mongo','mongosh','--quiet',options.database,'--eval',script],cwd=ROOT,env=environment,capture_output=True,text=True)
        (output/(stage+'-stderr.txt')).write_text(process.stderr)
        if process.returncode: raise RuntimeError(f'{stage} snapshot failed: {process.stderr}')
        data=json.loads(process.stdout)
        with gzip.open(output/(stage+'.json.gz'),'wt',compresslevel=3) as destination: destination.write(process.stdout)
        return data
    before=snapshot('before')
    expected_counts={'users':10000,'products':230000,'sold':180000,'listings':50000,'transactions':300000,'successfulTransactions':180000,'priceChanges':575000}
    if before['counts']!=expected_counts:
        raise RuntimeError('load requires a restored full V2 initial dataset; actual='+str(before['counts']))
    if len(before['products'])!=30000 or any(row['sold'] or not row['listing'] or row['seedCohort']!='dedicated' for row in before['products'].values()) or before['successRecords']:
        raise RuntimeError('designated products must all be uncommitted and eligible before load')
    if before['sellerCounts']!={account['id']:account['completedSales'] for account in manifest['accounts']}:
        raise RuntimeError('seller counters differ from restored manifest baseline')
    # Health and Search ingestion are preparation, never measured sale requests.
    deadline=time.monotonic()+180
    with open(output/'search-readiness.jsonl','w') as observations:
        while True:
            try:
                with urllib.request.urlopen(options.api+'/search?q=%E5%8D%95%E8%BD%A6',timeout=15) as response: result=json.load(response)
                ready=result.get('total')==16667
                observations.write(json.dumps({'UTC':datetime.now(timezone.utc).isoformat(),'expectedTotal':16667,'actualTotal':result.get('total'),'ready':ready})+'\n');observations.flush()
                if ready: break
            except Exception as error:
                observations.write(json.dumps({'UTC':datetime.now(timezone.utc).isoformat(),'error':str(error)})+'\n');observations.flush()
            if time.monotonic()>=deadline: raise RuntimeError('Search full-cohort ingestion did not become ready within180s')
            time.sleep(1)
    environment.update({'SALE_RESULTS':str(output),'SALE_MANIFEST':str(manifest_path),'SALE_DURATION':str(options.duration),'SALE_PRODUCTS':str(options.products),'SALE_TIMEOUT':str(options.timeout),'SALE_RETRIES':str(options.retries),'SALE_FORMAL':'1' if options.formal else '0'})
    if options.start_gate: environment['SALE_MEASUREMENT_GATE']=str(options.start_gate.resolve())
    command=['uv','run','--frozen','--project',str(ROOT/'experiments'),'locust','-f',str(ROOT/'experiments/sale_locust.py'),'--headless','--users','50','--spawn-rate','50','--exit-code-on-error','0','--host',options.api,'--stop-timeout',str(math.ceil(options.timeout+10))+'s','--csv',str(output/'locust'),'--csv-full-history','--html',str(output/'locust.html')]
    (output/'preparation.json').write_text(json.dumps({'snapshotAndSearchSeconds':time.monotonic()-prepare,'command':command},indent=2)+'\n')
    with open(output/'locust.log','w') as log:
        completed=subprocess.run(command,cwd=ROOT,env=environment,stdout=log,stderr=subprocess.STDOUT,timeout=options.duration+options.timeout+330)
    (output/'locust-exit.json').write_text(json.dumps({'returncode':completed.returncode})+'\n')
    # Bounded settlement records eventual commits without reclassifying receipt.
    time.sleep(options.settle)
    snapshot('after')
    summarized=subprocess.run([sys.executable,str(ROOT/'experiments/summarize_sales.py'),'--run-dir',str(output),'--manifest',str(manifest_path)],cwd=ROOT,check=True,capture_output=True,text=True)
    result=json.loads(summarized.stdout)
    print(json.dumps({'output':str(output),'onTimeUniqueSales':result['client']['onTimeUniqueSales'],'consistent':result['database']['consistent'],'passed':result['passed'],'formalPassed':result['formalPassed'],'locustExit':completed.returncode},indent=2))
    return 0 if result['passed'] and completed.returncode==0 else 1

if __name__=='__main__':
    try: sys.exit(main())
    except Exception as error:
        print(f'load preparation/run failed: {error}',file=sys.stderr)
        sys.exit(2)
