import contextlib
import io
import json
import tempfile
import unittest
import urllib.error
from pathlib import Path
from unittest.mock import patch
import iris_m2m

class IrisRecoveryTest(unittest.TestCase):
    def test_network_loss_reuses_key_then_success_allows_new_send(self):
        with tempfile.TemporaryDirectory() as d:
            config = Path(d)/'config.json'
            secret = Path(d)/'secret'
            secret.write_text('secret')
            config.write_text(json.dumps({'base':'https://example.test','client_id':'c','secret_file':str(secret),'token_url':'https://auth.test'}))
            keys=[]
            def request(url, body=None, headers=None):
                if url.endswith('/agent/destinations'):
                    keys.append(headers['Idempotency-Key'])
                    if len(keys)==1: raise urllib.error.URLError('lost response')
                    return {'id':'1'}
                return {'access_token':'token'}
            def run():
                with patch.object(iris_m2m, 'CONFIG', config), patch.object(iris_m2m.sys,'argv',['iris_m2m.py','send']), patch.object(iris_m2m.sys,'stdin',io.StringIO('place')), patch.object(iris_m2m,'request',side_effect=request), contextlib.redirect_stdout(io.StringIO()):
                    iris_m2m.main()
            with self.assertRaises(urllib.error.URLError): run()
            run(); run()
            self.assertEqual(keys[0],keys[1])
            self.assertNotEqual(keys[1],keys[2])

    def test_definite_validation_error_allows_new_input(self):
        with tempfile.TemporaryDirectory() as d:
            with patch.object(iris_m2m, 'CONFIG', Path(d)/'config'):
                def rejected(*args):
                    raise urllib.error.HTTPError('https://example.test', 422, 'invalid', None, io.BytesIO())
                with patch.object(iris_m2m, 'request', side_effect=rejected):
                    with self.assertRaises(urllib.error.HTTPError) as caught:
                        iris_m2m.durable_send('https://example.test', b'first', {})
                    caught.exception.close()
                with patch.object(iris_m2m, 'request', return_value={'id':'ok'}):
                    self.assertEqual(iris_m2m.durable_send('https://example.test', b'second', {})['id'], 'ok')
