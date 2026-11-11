import contextlib
import io
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
import iris_m2m

class IrisReplayTest(unittest.TestCase):
    def test_replay_exposes_historical_state_and_replay_flag(self):
        config={'base':'https://example.test','client_id':'client','secret_file':'/secret','token_url':'https://auth.test'}
        def request(url, body=None, headers=None):
            if url.endswith('/agent/destinations'):
                return {'id':'old','state':'pending','replayed':True}
            return {'access_token':'token'}
        with tempfile.TemporaryDirectory() as d, patch.object(iris_m2m,'CONFIG',Path(d)/'config'), patch.object(iris_m2m.sys,'argv',['iris_m2m.py','send']), patch.object(iris_m2m.sys,'stdin',io.StringIO('place')), patch.object(iris_m2m.Path,'read_text',side_effect=[json.dumps(config),'secret']), patch.object(iris_m2m,'request',side_effect=request), contextlib.redirect_stdout(io.StringIO()) as out:
            iris_m2m.main()
        result=json.loads(out.getvalue())
        self.assertEqual(result['state_at_creation'],'pending')
        self.assertTrue(result['replayed'])
        self.assertNotIn('current_state',result)
        self.assertIn('replay',result['state_notice'].lower())

    def test_legacy_response_reports_unknown_replay(self):
        config={'base':'https://example.test','client_id':'client','secret_file':'/secret','token_url':'https://auth.test'}
        def request(url, body=None, headers=None):
            return {'id':'old','state':'pending'} if url.endswith('/agent/destinations') else {'access_token':'token'}
        with tempfile.TemporaryDirectory() as d, patch.object(iris_m2m,'CONFIG',Path(d)/'config'), patch.object(iris_m2m.sys,'argv',['iris_m2m.py','send']), patch.object(iris_m2m.sys,'stdin',io.StringIO('place')), patch.object(iris_m2m.Path,'read_text',side_effect=[json.dumps(config),'secret']), patch.object(iris_m2m,'request',side_effect=request), contextlib.redirect_stdout(io.StringIO()) as out:
            iris_m2m.main()
        self.assertIsNone(json.loads(out.getvalue())['replayed'])
