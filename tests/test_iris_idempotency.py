import contextlib
import io
import json
import unittest
import tempfile
from pathlib import Path
from unittest.mock import patch
import iris_m2m

class IrisIdempotencyTest(unittest.TestCase):
    def test_successful_sends_use_distinct_keys(self):
        headers = []
        config = {'base':'https://example.test','client_id':'client','secret_file':'/secret','token_url':'https://auth.test'}
        def request(url, body=None, headers=None):
            if url.endswith('/agent/destinations'):
                captured.append(headers)
                return {'id':'1'}
            return {'access_token':'token'}
        captured = headers
        with tempfile.TemporaryDirectory() as d, patch.object(iris_m2m, 'CONFIG', Path(d) / 'config'):
            for _ in range(2):
                with patch.object(iris_m2m.sys,'argv',['iris_m2m.py','send']), patch.object(iris_m2m.sys,'stdin',io.StringIO('place')), patch.object(iris_m2m.Path,'read_text',side_effect=[json.dumps(config),'secret']), patch.object(iris_m2m,'request',side_effect=request), contextlib.redirect_stdout(io.StringIO()):
                    iris_m2m.main()
        self.assertTrue(headers[0].get('Idempotency-Key'))
        self.assertNotEqual(headers[0]['Idempotency-Key'], headers[1]['Idempotency-Key'])
