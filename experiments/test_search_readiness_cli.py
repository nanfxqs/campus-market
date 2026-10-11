"""Actual HTTP preparation seam shared by both load runners."""
import http.server,json,pathlib,subprocess,tempfile,threading,time,unittest
ROOT=pathlib.Path(__file__).resolve().parents[1]
class Readiness(unittest.TestCase):
 def test_retries_http_failure_and_incomplete_cohort_with_raw_observations(self):
  for form in ['sale','detail']:self.exercise([500,42,16667],True,form)
 def test_unready_cohort_fails_with_finite_observation_log(self):
  for form in ['sale','detail']:self.exercise([42],False,form)
 def exercise(self,responses,success,form):
  class Handler(http.server.BaseHTTPRequestHandler):
   calls=0
   def do_GET(self):
    value=responses[min(self.calls,len(responses)-1)];type(self).calls+=1
    self.send_response(500 if value==500 else 200);self.end_headers();self.wfile.write(json.dumps({'total':value}).encode())
   def log_message(self,*args):pass
  server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
  try:
   with tempfile.TemporaryDirectory() as td:
    p=pathlib.Path(td)/'observations.jsonl';begin=time.monotonic()
    r=subprocess.run(['python3',str(ROOT/'scripts/search_readiness.py'),'--api',f'http://127.0.0.1:{server.server_port}','--output',str(p),'--format',form,'--timeout','0.2','--interval','0.01'],capture_output=True,text=True)
    self.assertEqual(r.returncode==0,success,r.stderr);self.assertLess(time.monotonic()-begin,1)
    rs=[json.loads(x) for x in p.read_text().splitlines()]
    if success:self.assertIn('error',rs[0]);self.assertEqual(rs[1]['actualTotal' if form=='sale' else 'actual'],42);self.assertEqual(rs[2]['actualTotal' if form=='sale' else 'actual'],16667);self.assertTrue(rs[2]['ready'])
    else:self.assertTrue(rs);self.assertFalse(any(x.get('ready') for x in rs))
  finally:server.shutdown();server.server_close();thread.join()
if __name__=='__main__':unittest.main()
