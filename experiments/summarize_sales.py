#!/usr/bin/env python3
"""Recompute sale metrics from immutable request/window/database artifacts."""
import argparse
import gzip
import json
import math
from pathlib import Path


def load(path):
    opener = gzip.open if str(path).endswith('.gz') else open
    with opener(path, 'rt') as source:
        return json.load(source)


def metrics(rows, duration):
    latencies = sorted(row['elapsedMs'] for row in rows)
    failures = sum(row['_kind'] != 'success' for row in rows)
    def percentile(percent):
        return latencies[max(0, math.ceil(len(latencies)*percent)-1)] if latencies else None
    return {'requests':len(rows),'throughput':len(rows)/duration,'p95Ms':percentile(.95),'p99Ms':percentile(.99),'errors':failures,'errorRate':failures/len(rows) if rows else 0}


def snapshot(root, stage):
    path=root/(stage+'.json.gz')
    if not path.exists(): path=root/(stage+'.json')
    return load(path) if path.exists() else None


def reconcile(before, after, products, selected, on_time, rows):
    violations=[]
    new=set()
    expected_seller_deltas={}
    for product_id, expected in products.items():
        initial=before['products'].get(product_id)
        final=after['products'].get(product_id)
        prior=before['successRecords'].get(product_id,[])
        current=after['successRecords'].get(product_id,[])
        if not initial or not final:
            violations.append({'productId':product_id,'reason':'missing archive'})
            continue
        additional=[record for record in current if record not in prior]
        if len(current)>1 or len(additional)>1:
            violations.append({'productId':product_id,'reason':'duplicate success records'})
        if additional:
            new.add(product_id)
            expected_seller_deltas[expected['sellerId']]=expected_seller_deltas.get(expected['sellerId'],0)+1
            record=additional[0]
            fields=['productId','sellerId','buyerId','priceCents','idempotencyKey']
            valid=all(record.get(field)==expected[field] for field in fields)
            valid=valid and record.get('success') is True and record.get('code')=='sold'
            sale=final.get('sale') or {}
            valid=valid and all(sale.get(field)==record.get(field) for field in ['buyerId','priceCents','confirmedAt'])
            valid=valid and final['sold'] is True and final['listing'] is False and final['sellerId']==expected['sellerId']
            valid=valid and not initial['sold'] and initial['listing'] and not prior
            if not valid: violations.append({'productId':product_id,'reason':'partial or mismatched committed sale'})
            if product_id not in selected: violations.append({'productId':product_id,'reason':'unassigned product changed'})
        elif final!=initial or current!=prior:
            violations.append({'productId':product_id,'reason':'archive/listing/records changed without new success'})
    sellers=[]
    for seller, initial in before['sellerCounts'].items():
        actual=after['sellerCounts'].get(seller,0)-initial
        expected=expected_seller_deltas.get(seller,0)
        if actual!=expected: violations.append({'sellerId':seller,'reason':'seller counter mismatch','actual':actual,'expected':expected})
        if expected or actual: sellers.append({'sellerId':seller,'delta':actual,'expected':expected})
    deltas={key:after['counts'][key]-before['counts'][key] for key in ['successfulTransactions','sold','listings']}
    for key,expected in [('successfulTransactions',len(new)),('sold',len(new)),('listings',-len(new))]:
        if deltas[key]!=expected: violations.append({'reason':'global count mismatch','field':key,'actual':deltas[key],'expected':expected})
    for unchanged in ['users','products','priceChanges']:
        if unchanged in before['counts'] and after['counts'].get(unchanged)!=before['counts'][unchanged]: violations.append({'reason':'unexpected static collection count change','field':unchanged})
    if set(after['sellerCounts'])!=set(before['sellerCounts']): violations.append({'reason':'registered seller set changed'})
    transaction_delta=after['counts'].get('transactions',0)-before['counts'].get('transactions',0) if 'transactions' in before['counts'] and 'transactions' in after['counts'] else None
    if transaction_delta is not None and transaction_delta<len(new): violations.append({'reason':'transaction delta smaller than new successes'})
    missing=on_time-new
    if missing: violations.append({'reason':'on-time client success without new persisted fact','products':sorted(missing)})
    timed_out={row['productId'] for row in rows if row['_kind']=='transportError'}
    started={row['productId'] for row in rows}
    valid_receipts={row['productId'] for row in rows if row['_kind']=='success'}
    late_receipts={row['productId'] for row in rows if row['_kind']=='success' and row['productId'] not in on_time}
    return {'consistent':not violations,'violations':violations,'newSuccessfulProducts':sorted(new),'newSuccessfulCount':len(new),'committedWithoutOnTimeResponse':sorted(new-on_time),'timeoutCommittedProducts':sorted((new-on_time)&timed_out),'lateCommittedProducts':sorted((new-on_time)&late_receipts),'committedWithoutAnyValidResponse':sorted(new-valid_receipts),'unknownTransportProducts':sorted(timed_out-new),'unresolvedProducts':sorted(started-new),'neverStartedProducts':sorted(selected-started),'sellerDeltas':sellers,'globalDeltas':deltas,'allTransactionDelta':transaction_delta,'nonSuccessTransactionDelta':transaction_delta-len(new) if transaction_delta is not None else None,'snapshotTime':after.get('snapshotTime')}


def summarize(root, manifest):
    window = load(root/'window.json')
    duration = window['durationSeconds']
    measured=window.get('measuredWindowSeconds',duration)
    if not 0<measured<=duration: raise ValueError('actual measured window must be positive and no longer than configured cutoff')
    products = {row['productId']:row for row in manifest['products']}
    selected = set(window['selectedProducts'])
    request_path = root/'requests.jsonl.gz' if (root/'requests.jsonl.gz').exists() else root/'requests.jsonl'
    opener = gzip.open if str(request_path).endswith('.gz') else open
    with opener(request_path,'rt') as source:
        rows = [json.loads(line) for line in source if line.strip()]
    launched = []
    on_time = set()
    late = set()
    duplicates = 0
    historical = 0
    before=snapshot(root,'before')
    after=snapshot(root,'after')
    prior_products={pid for pid,row in (before or {}).get('products',{}).items() if row['sold'] or (before or {}).get('successRecords',{}).get(pid)}
    for row in rows:
        if not 0 <= row['startedOffset'] < duration:
            raise ValueError('request launched outside measurement window')
        expected = products[row['productId']]
        if row['productId'] not in selected:
            raise ValueError('request used unassigned product')
        body = row.get('response') or {}
        valid = all(body.get(field)==expected[field] for field in ['productId','sellerId','buyerId','priceCents','idempotencyKey'])
        row['_kind'] = 'success' if row['status']==200 and valid and body.get('success') is True and body.get('code')=='sold' else 'otherError'
        if row['status']==409 and body.get('code') in ['product_sold','product_expired','product_delisted','idempotency_conflict']:
            row['_kind']='conflict'
        elif row['status']==0:
            row['_kind']='transportError'
        elif row['status']>=500:
            row['_kind']='infrastructureError'
        elif row['status']==200 and row['_kind']!='success':
            row['_kind']='invalidResponse'
        if row['_kind']=='success':
            if row['productId'] in prior_products:
                historical+=1
            elif 0 <= row['receivedOffset'] <= duration:
                if row['productId'] in on_time: duplicates += 1
                on_time.add(row['productId'])
            else:
                late.add(row['productId'])
        launched.append(row)
    on_time_responses = [row for row in launched if 0 <= row['receivedOffset'] <= duration]
    result = {'schemaVersion':1,'formal':window['formal'],'window':window,'configuredCutoffSeconds':duration,'measuredWindowSeconds':measured,'client':{'onTimeUniqueSales':len(on_time),'onTimeSuccessProducts':sorted(on_time),'duplicateSuccessResponses':duplicates,'historicalSuccessResponses':historical,'lateSuccessProducts':sorted(late-on_time),'newSaleThroughput':len(on_time)/measured,'lastOnTimeSuccessSeconds':max((row['receivedOffset'] for row in rows if row['_kind']=='success' and row['productId'] in on_time and row['receivedOffset']<=duration),default=None),'conflicts':sum(row['_kind']=='conflict' for row in rows),'infrastructureErrors':sum(row['_kind']=='infrastructureError' for row in rows),'transportErrors':sum(row['_kind']=='transportError' for row in rows),'invalidResponses':sum(row['_kind']=='invalidResponse' for row in rows),'otherErrors':sum(row['_kind']=='otherError' for row in rows),'retries':sum(row['attempt']>1 for row in rows)},'metrics':{'allLaunchedRequests':metrics(launched,measured),'onTimeReceivedResponses':metrics(on_time_responses,measured),'successfulResponses':metrics([row for row in launched if row['_kind']=='success'],measured),'businessConflicts':metrics([row for row in launched if row['_kind']=='conflict'],measured),'otherFailures':metrics([row for row in launched if row['_kind'] not in ['success','conflict']],measured)},'method':'Nearest-rank P95/P99 over end-to-end elapsedMs. allLaunchedRequests includes requests completing after deadline; onTimeReceivedResponses throughput counts only receipts before configured cutoff. Throughput denominators use measuredWindowSeconds (pool completion or cutoff); configured duration remains the hard receipt deadline. New sale throughput counts unique valid on-time success products.'}

    if before is not None and after is not None:
        result['database']=reconcile(before,after,products,selected,on_time,rows)
    ready=window.get('ready',[])
    result['barrierValidated']=window.get('expectedUsers')==50 and window['readyUsers']==50 and len(ready)==50 and len({row.get('sellerId') for row in ready})==50 and all(row.get('readyUnixNs',float('inf'))<=window.get('startUnixNs',0) for row in ready)
    result['passed']=bool(result.get('database',{}).get('consistent')) and len(on_time)==len(selected) and result['barrierValidated'] and window.get('finalized',False) and not window.get('runtimeErrors') and not window.get('preparationError')
    result['formalPassed']=result['passed'] and window['formal'] and duration==600 and len(selected)==30000
    return result


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run-dir',type=Path,required=True)
    parser.add_argument('--manifest',type=Path,required=True)
    arguments=parser.parse_args()
    result=summarize(arguments.run_dir,load(arguments.manifest))
    text=json.dumps(result,indent=2,ensure_ascii=False)
    (arguments.run_dir/'summary.json').write_text(text+'\n')
    print(text)

if __name__=='__main__': main()
