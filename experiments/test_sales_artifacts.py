"""Specification examples through the public saved-artifact statistics CLI."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

class SaleArtifactStatistics(unittest.TestCase):
    def run_artifacts(self, records, snapshots=None, window_fields=None):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            rows = [{'productId':p,'sellerId':'seller','buyerId':'buyer','priceCents':100,'idempotencyKey':'key-'+p} for p in ['p1','p2','p3']]
            (root/'manifest.json').write_text(json.dumps({'products':rows}))
            (root/'window.json').write_text(json.dumps({'durationSeconds':10,'readyUsers':50,'expectedUsers':50,'formal':False,'selectedProducts':['p1','p2','p3'],**(window_fields or {})}))
            (root/'requests.jsonl').write_text(''.join(json.dumps(row)+'\n' for row in records))
            if snapshots:
                for stage,data in snapshots.items(): (root/(stage+'.json')).write_text(json.dumps(data))
            process = subprocess.run([sys.executable,str(Path(__file__).with_name('summarize_sales.py')),'--run-dir',str(root),'--manifest',str(root/'manifest.json')],capture_output=True,text=True)
            self.assertEqual(process.returncode,0,process.stderr)
            return json.loads(process.stdout)
    @staticmethod
    def success(product, received, latency=10):
        return {'productId':product,'startedOffset':1,'receivedOffset':received,'elapsedMs':latency,'status':200,'attempt':1,'response':{'productId':product,'sellerId':'seller','buyerId':'buyer','priceCents':100,'idempotencyKey':'key-'+product,'success':True,'code':'sold'}}
    def test_deadline_inclusive_unique_new_success_excludes_duplicate_and_late(self):
        result=self.run_artifacts([self.success('p1',10,10),self.success('p1',10,20),self.success('p2',10.001,30)])
        self.assertEqual(result['client']['onTimeUniqueSales'],1)
        self.assertEqual(result['client']['duplicateSuccessResponses'],1)
        self.assertEqual(result['client']['lateSuccessProducts'],['p2'])
        self.assertEqual(result['client']['newSaleThroughput'],0.1)
        self.assertEqual(result['metrics']['allLaunchedRequests']['p95Ms'],30)

    def test_error_classes_and_retries_do_not_become_new_sales(self):
        result=self.run_artifacts([
            {'productId':'p1','startedOffset':1,'receivedOffset':2,'elapsedMs':10,'status':409,'attempt':1,'response':{'code':'product_sold'}},
            {'productId':'p2','startedOffset':1,'receivedOffset':3,'elapsedMs':20,'status':503,'attempt':1,'response':{'code':'unavailable'}},
            {'productId':'p3','startedOffset':1,'receivedOffset':4,'elapsedMs':30,'status':0,'attempt':1,'error':'read timeout'},
            {**self.success('p3',5,40),'attempt':2,'response':{'success':True,'code':'sold','idempotencyKey':'wrong'}}
        ])
        self.assertEqual(result['client']['onTimeUniqueSales'],0)
        self.assertEqual(result['client']['conflicts'],1)
        self.assertEqual(result['client']['infrastructureErrors'],1)
        self.assertEqual(result['client']['transportErrors'],1)
        self.assertEqual(result['client']['invalidResponses'],1)
        self.assertEqual(result['client']['retries'],1)
        self.assertEqual(result['metrics']['allLaunchedRequests']['errorRate'],1)
        self.assertEqual(result['metrics']['allLaunchedRequests']['p99Ms'],40)

    def test_timeout_commit_is_reconciled_separately_from_on_time_success(self):
        before={'counts':{'successfulTransactions':180000,'sold':180000,'listings':50000},'sellerCounts':{'seller':18},'products':{'p3':{'sellerId':'seller','sold':False,'listing':True,'sale':None}},'successRecords':{}}
        record={'productId':'p3','sellerId':'seller','buyerId':'buyer','priceCents':100,'idempotencyKey':'key-p3','success':True,'code':'sold','confirmedAt':'2026-10-11T10:00:00Z'}
        after={'counts':{'successfulTransactions':180001,'sold':180001,'listings':49999},'sellerCounts':{'seller':19},'products':{'p3':{'sellerId':'seller','sold':True,'listing':False,'sale':{'buyerId':'buyer','priceCents':100,'confirmedAt':'2026-10-11T10:00:00Z'}}},'successRecords':{'p3':[record]}}
        for product in ['p1','p2']:
            before['products'][product]={'sellerId':'seller','sold':False,'listing':True,'sale':None}
            after['products'][product]=before['products'][product].copy()
        result=self.run_artifacts([{'productId':'p3','startedOffset':1,'receivedOffset':5,'elapsedMs':4000,'status':0,'attempt':1,'error':'timeout'}],{'before':before,'after':after})
        self.assertEqual(result['client']['onTimeUniqueSales'],0)
        self.assertEqual(result['database']['newSuccessfulProducts'],['p3'])
        self.assertEqual(result['database']['committedWithoutOnTimeResponse'],['p3'])
        self.assertEqual(result['database']['timeoutCommittedProducts'],['p3'])
        self.assertTrue(result['database']['consistent'])

    def test_historical_idempotent_success_is_not_new(self):
        product={'sellerId':'seller','sold':True,'listing':False,'sale':{'buyerId':'buyer','priceCents':100,'confirmedAt':'earlier'}}
        previous={**self.success('p1',2)['response'],'confirmedAt':'earlier'}
        before={'counts':{'successfulTransactions':180000,'sold':180000,'listings':50000},'sellerCounts':{'seller':18},'products':{'p1':product},'successRecords':{'p1':[previous]}}
        for item in ['p2','p3']: before['products'][item]={'sellerId':'seller','sold':False,'listing':True,'sale':None}
        result=self.run_artifacts([self.success('p1',2)],{'before':before,'after':before})
        self.assertEqual(result['client']['onTimeUniqueSales'],0)
        self.assertEqual(result['client']['historicalSuccessResponses'],1)
        self.assertEqual(result['database']['newSuccessfulCount'],0)
        self.assertTrue(result['database']['consistent'])

    def test_http_success_with_partial_database_commit_cannot_pass(self):
        before={'counts':{'successfulTransactions':180000,'sold':180000,'listings':50000},'sellerCounts':{'seller':18},'products':{},'successRecords':{}}
        for item in ['p1','p2','p3']: before['products'][item]={'sellerId':'seller','sold':False,'listing':True,'sale':None}
        after=json.loads(json.dumps(before))
        after['products']['p1']['sold']=True
        after['products']['p1']['sale']={'buyerId':'buyer','priceCents':100,'confirmedAt':'now'}
        after['successRecords']['p1']=[{**self.success('p1',2)['response'],'confirmedAt':'now'}]
        after['counts']['successfulTransactions']+=1
        after['counts']['sold']+=1
        result=self.run_artifacts([self.success('p1',2)],{'before':before,'after':after})
        self.assertEqual(result['client']['onTimeUniqueSales'],1)
        self.assertFalse(result['database']['consistent'])
        self.assertFalse(result['passed'])
        self.assertFalse(result['formalPassed'])

    def test_recorded_barrier_requires_50_distinct_ready_users(self):
        records=[{'sellerId':'same','readyUnixNs':1} for _ in range(50)]
        result=self.run_artifacts([],window_fields={'ready':records,'startUnixNs':2})
        self.assertFalse(result['barrierValidated'])
        self.assertFalse(result['passed'])

if __name__=='__main__': unittest.main()
