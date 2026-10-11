"""Public offline artifact boundary: receipts, not pass flags, decide acceptance."""
import json,pathlib,subprocess,tempfile,unittest
ROOT=pathlib.Path(__file__).resolve().parents[1]
class Evidence(unittest.TestCase):
 def fixture(self,p,duplicate=False,error=False):
  names=['publish','browse','search','detail','seller','sale','expiry','stats'];records=[]
  for name in names:
   for i in range(100):records.append({'phase':'measurement','feature':name,'sample':0 if duplicate else i,'status':500 if error and name=='sale' and i==99 else 200,'expected_status':200,'elapsed_ms':i+1})
  (p/'http.jsonl').write_text(''.join(json.dumps(r)+'\n' for r in records));(p/'summary.json').write_text('{"passed":true}')
  (p/'ttl-observations.json').write_text(json.dumps({'result':'PASS','observations':[{'product_id':str(i),'deletion_delay_lower_seconds':1,'deletion_delay_upper_seconds':3} for i in range(10)]}))
  for name in ['instant-expiry.json','ttl-polls.jsonl','post-ttl-retention.txt']:(p/name).write_text('retained test artifact\n')
 def check(self,p):return subprocess.run(['python3',str(ROOT/'scripts/validate-v2.py'),'--check-functions',str(p)],capture_output=True,text=True)
 def test_measured_error_rejects_fabricated_pass_summary(self):
  with tempfile.TemporaryDirectory() as td:
   p=pathlib.Path(td);self.fixture(p,error=True);r=self.check(p);self.assertNotEqual(r.returncode,0);self.assertIn('raw',r.stdout+r.stderr)
 def test_duplicate_sample_receipts_do_not_satisfy_100_measurements(self):
  with tempfile.TemporaryDirectory() as td:
   p=pathlib.Path(td);self.fixture(p,duplicate=True);self.assertNotEqual(self.check(p).returncode,0)
 def test_literal_receipts_have_known_percentiles(self):
  with tempfile.TemporaryDirectory() as td:
   p=pathlib.Path(td);self.fixture(p);r=self.check(p);self.assertEqual(r.returncode,0,r.stdout);m=json.loads(r.stdout)['metrics']['sale'];self.assertEqual(m,{'medianMs':50.5,'p95Ms':95,'p99Ms':99})
if __name__=='__main__':unittest.main()
