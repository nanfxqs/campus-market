"""Public evidence CLI tests using delivered raw fixtures, never internal functions."""
import gzip,json,pathlib,shutil,subprocess,tempfile,unittest
ROOT=pathlib.Path(__file__).resolve().parents[1]
SCRIPT=ROOT/'scripts/validate-v2.py'
class Binding(unittest.TestCase):
 def test_one_sale_cannot_fill_nine_formal_slots(self):
  with tempfile.TemporaryDirectory() as td:
   p=pathlib.Path(td);idx=json.loads((ROOT/'docs/validation/v2-evidence.json').read_text())
   for e in idx['experiments']:
    if e['kind']=='sale':e['path']='docs/validation/v2/baseline/A-attempt-1/round-1'
   (p/'index.json').write_text(json.dumps(idx))
   r=subprocess.run(['python3',str(SCRIPT),'--index',str(p/'index.json'),'--output',str(p/'out')],capture_output=True,text=True)
   self.assertNotEqual(r.returncode,0,r.stdout)
class Competition(unittest.TestCase):
 def test_raw_seller_increment_cannot_be_hidden_by_summary(self):
  with tempfile.TemporaryDirectory() as td:
   p=pathlib.Path(td)/'race';shutil.copytree(ROOT/'docs/validation/v2-race25/formal-A',p)
   facts=[json.loads(x) for x in (p/'database-facts.jsonl').read_text().splitlines()]
   for r in facts:
    if r.get('group')==0 and 'stdout' in r:
     d=json.loads(r['stdout']);d['sellerSales']=0;r['stdout']=json.dumps(d)
   (p/'database-facts.jsonl').write_text(''.join(json.dumps(r)+'\n' for r in facts))
   r=subprocess.run(['python3',str(SCRIPT),'--check-stage','competition-A',str(p)],capture_output=True,text=True)
   self.assertNotEqual(r.returncode,0,r.stdout)
class CorruptedArtifacts(unittest.TestCase):
 def stage(self,stage,p):
  return subprocess.run(['python3',str(SCRIPT),'--check-stage',stage,str(p)],capture_output=True,text=True)
 def copied(self,source,p):
  # Link immutable bulk receipts/resources; copy mutation targets before writing.
  shutil.copytree(ROOT/source,p,copy_function=lambda src,dst: pathlib.Path(dst).symlink_to(pathlib.Path(src).resolve()))
 def rewrite(self,p,name,data):
  f=p/name
  if f.is_symlink():f.unlink()
  if name.endswith('.gz'):
   with gzip.open(f,'wt') as out:json.dump(data,out)
  else:f.write_text(json.dumps(data))
 def test_sale_configuration_and_round_substitution(self):
  source=ROOT/'docs/validation/v2/baseline/A-attempt-1/round-1'
  for stage in ['A2','B1','C1']:
   with self.subTest(stage=stage):self.assertNotEqual(self.stage(stage,source).returncode,0)
 def test_duplicate_round_identity_even_with_relabelled_round_metadata(self):
  with tempfile.TemporaryDirectory() as td:
   p=pathlib.Path(td);copied=p/'copy';self.copied('docs/validation/v2/baseline/A-attempt-1/round-1',copied)
   result=json.loads((copied/'result.json').read_text());result['round']=2;self.rewrite(copied,'result.json',result)
   idx=json.loads((ROOT/'docs/validation/v2-evidence.json').read_text())
   next(e for e in idx['experiments'] if e['id']=='A2')['path']=str(copied)
   (p/'index.json').write_text(json.dumps(idx))
   r=subprocess.run(['python3',str(SCRIPT),'--index',str(p/'index.json'),'--output',str(p/'out')],capture_output=True,text=True)
   self.assertNotEqual(r.returncode,0,r.stdout)
   matrix=json.loads((p/'out/verification.json').read_text());row=next(x for x in matrix['experiments'] if x['id']=='A2');self.assertIn('duplicate measured',row['error'])
 def test_sale_initial_scale_corruption(self):
  with tempfile.TemporaryDirectory() as td:
   p=pathlib.Path(td)/'sale';self.copied('docs/validation/v2/baseline/A-attempt-1/round-1',p)
   with gzip.open(p/'sale/before.json.gz','rt') as f:d=json.load(f)
   d['counts']['products']=200000;self.rewrite(p,'sale/before.json.gz',d)
   self.assertNotEqual(self.stage('A1',p).returncode,0)
 def test_detail_configuration_state_pattern_substitution(self):
  source=ROOT/'docs/validation/v2-details23/formal/A-hotspot'
  for stage in ['detail-A-dispersed','detail-B-cold-hotspot','detail-B-hot-dispersed','detail-C-hot']:
   with self.subTest(stage=stage):self.assertNotEqual(self.stage(stage,source).returncode,0)
 def test_detail_scale_pool_and_cold_preparation_corruption(self):
  for corruption in ['scale','pool','cold']:
   with self.subTest(corruption=corruption),tempfile.TemporaryDirectory() as td:
    p=pathlib.Path(td)/'detail';self.copied('docs/validation/v2-details23/formal/B-cold-hotspot',p)
    if corruption=='scale':
     d=json.loads((p/'dataset-before.json').read_text());d['counts']['users']=9999;self.rewrite(p,'dataset-before.json',d)
    elif corruption=='pool':
     import hashlib
     with gzip.open(p/'pool.json.gz','rt') as f:d=json.load(f)
     d['ids'][0],d['ids'][1]=d['ids'][1],d['ids'][0];d['sha256']=hashlib.sha256(json.dumps(d['ids'],separators=(',',':')).encode()).hexdigest();self.rewrite(p,'pool.json.gz',d)
    else:
     d=json.loads((p/'preparation.json').read_text());d['cacheDbsize']=1;self.rewrite(p,'preparation.json',d)
    self.assertNotEqual(self.stage('detail-B-cold-hotspot',p).returncode,0)
 def test_duplicate_detail_sequence_receipt_is_rejected(self):
  with tempfile.TemporaryDirectory() as td:
   p=pathlib.Path(td)/'detail';self.copied('docs/validation/v2-details23/formal/A-hotspot',p)
   f=p/'requests.jsonl.gz'
   with gzip.open(f,'rt') as raw:lines=raw.readlines()
   f.unlink()
   with gzip.open(f,'wt') as raw:raw.writelines(lines+[lines[0]])
   r=self.stage('detail-A-hotspot',p);self.assertNotEqual(r.returncode,0,r.stdout)
 def test_hot_requires_every_prewarm_receipt(self):
  with tempfile.TemporaryDirectory() as td:
   p=pathlib.Path(td)/'detail';self.copied('docs/validation/v2-details23/formal/B-hot-hotspot',p)
   f=p/'prewarm.jsonl.gz'
   with gzip.open(f,'rt') as raw:lines=raw.readlines()
   f.unlink()
   with gzip.open(f,'wt') as raw:raw.writelines(lines[:-1])
   self.assertNotEqual(self.stage('detail-B-hot-hotspot',p).returncode,0)
 def test_hot_cache_clear_after_preparation_is_rejected(self):
  with tempfile.TemporaryDirectory() as td:
   p=pathlib.Path(td)/'detail';self.copied('docs/validation/v2-details23/formal/B-hot-hotspot',p)
   name='commands.jsonl.gz' if (p/'commands.jsonl.gz').exists() else 'commands.jsonl';f=p/name
   opener=gzip.open if name.endswith('.gz') else open
   with opener(f,'rt') as source:lines=source.readlines()
   empty=json.loads((p/'cache-empty.json').read_text());lines.append(json.dumps({'argv':['redis-cli','FLUSHDB'],'startedUnixNs':empty['unixNs']+1})+'\n')
   f.unlink()
   with opener(f,'wt') as dest:dest.writelines(lines)
   self.assertNotEqual(self.stage('detail-B-hot-hotspot',p).returncode,0)
 def test_competition_raw_receipt_and_database_corruption(self):
  for corruption in ['buyer','transaction','code','same-key-missing','same-key-duplicate','changed-content','upstream','summary']:
   with self.subTest(corruption=corruption),tempfile.TemporaryDirectory() as td:
    p=pathlib.Path(td)/'race';self.copied('docs/validation/v2-race25/formal-C',p)
    name='database-facts.jsonl' if corruption in ['buyer','transaction'] else 'responses.jsonl'
    rs=[json.loads(x) for x in (p/name).read_text().splitlines()]
    if corruption in ['buyer','transaction']:
     r=next(r for r in rs if r['group']==0 and 'stdout' in r);d=json.loads(r['stdout']);d['successfulTransactions'][0]['buyerId' if corruption=='buyer' else 'id']='wrong';r['stdout']=json.dumps(d)
    elif corruption=='code':next(r for r in rs if r.get('group')==0 and r['status']==409)['response']['code']='idempotency_conflict'
    elif corruption=='same-key-missing':rs.remove(next(r for r in rs if r.get('phase')=='same-key'))
    elif corruption=='same-key-duplicate':
     same=[r for r in rs if r.get('phase')=='same-key'];same[1]['index']=same[0]['index']
    elif corruption=='changed-content':next(r for r in rs if r.get('phase')=='changed-content')['response']['error']['code']='product_sold'
    elif corruption=='upstream':
     for r in rs:
      if r.get('group')==0:r['upstream']='single:8080'
    else:
     d=json.loads((p/'summary.json').read_text());d['sameKey']['counts']['success']=55;self.rewrite(p,'summary.json',d)
    f=p/name;f.unlink();f.write_text(''.join(json.dumps(r)+'\n' for r in rs))
    r=self.stage('competition-C',p);self.assertNotEqual(r.returncode,0,r.stdout)
 def test_valid_raw_competition_passes(self):
  r=self.stage('competition-C',ROOT/'docs/validation/v2-race25/formal-C');self.assertEqual(r.returncode,0,r.stdout);d=json.loads(r.stdout);self.assertEqual(d['sameKey']['counts']['success'],54);self.assertEqual(d['sameKey']['counts']['business_conflict'],4)
if __name__=='__main__':unittest.main()
