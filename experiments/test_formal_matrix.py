"""Public artifact summary must never average away a failed formal round."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

class FormalMatrix(unittest.TestCase):
    def test_one_failed_round_fails_the_configuration(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp)
            (root/'configuration.json').write_text(json.dumps({'configuration':'A','rounds':3,'baseTime':'2026-10-11T00:00:00Z','randomSeed':42}))
            for number,count in [(1,30000),(2,29999),(3,30001)]:
                directory=root/f'round-{number}';directory.mkdir()
                (directory/'result.json').write_text(json.dumps({'round':number,'passed':count>=30000,'onTimeUniqueSales':count,'consistent':True,'resourceCoverage':True}))
            process=subprocess.run([sys.executable,str(Path(__file__).resolve().parents[1]/'scripts/run-formal-sales.py'),'--summarize','--output',str(root)],text=True,capture_output=True)
            self.assertEqual(process.returncode,1,process.stderr)
            self.assertFalse(json.loads(process.stdout)['passed'])
            self.assertEqual(json.loads(process.stdout)['failedRounds'],[2])

if __name__=='__main__': unittest.main()
