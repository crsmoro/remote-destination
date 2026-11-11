import contextlib
import io
import json
import unittest
from unittest.mock import patch
import iris_m2m

class IrisSearchTest(unittest.TestCase):
    def test_search_never_sends_destination(self):
        calls = []
        def request(url, body=None, headers=None):
            calls.append((url, body))
            return {'access_token':'test'} if url == 'https://auth/token' else {'token':'opaque','candidates':[{'label':'Place'}]}
        config = {'base':'https://example.test','client_id':'client','secret_file':'secret','token_url':'https://auth/token'}
        with patch.object(iris_m2m.sys,'argv',['iris_m2m.py','search']), patch.object(iris_m2m.sys,'stdin',io.StringIO('Place')), patch.object(iris_m2m.Path,'read_text',side_effect=[json.dumps(config),'secret']), patch.object(iris_m2m,'request',side_effect=request), contextlib.redirect_stdout(io.StringIO()):
            iris_m2m.main()
        self.assertEqual(calls[-1][0], 'https://example.test/agent/geocode')
        self.assertEqual(json.loads(calls[-1][1]), {'input':'Place'})

    def test_confirm_requires_explicit_token_and_index(self):
        calls = []
        def request(url, body=None, headers=None):
            calls.append((url, body))
            return {'access_token':'test'} if url == 'https://auth/token' else {'state':'pending'}
        config = {'base':'https://example.test','client_id':'client','secret_file':'secret','token_url':'https://auth/token'}
        with patch.object(iris_m2m.sys,'argv',['iris_m2m.py','confirm']), patch.object(iris_m2m.sys,'stdin',io.StringIO('{"token":"opaque","index":0}')), patch.object(iris_m2m.Path,'read_text',side_effect=[json.dumps(config),'secret']), patch.object(iris_m2m,'request',side_effect=request), contextlib.redirect_stdout(io.StringIO()):
            iris_m2m.main()
        self.assertEqual(calls[-1][0], 'https://example.test/agent/geocode/confirm')
        self.assertEqual(json.loads(calls[-1][1]), {'token':'opaque','index':0})

if __name__ == '__main__': unittest.main()
