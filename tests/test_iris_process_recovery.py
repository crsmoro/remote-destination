import json
import os
from pathlib import Path
import socket
import select
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

SCRIPT = str(Path(__file__).resolve().parents[1] / 'iris_m2m.py')

class RecoveryTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.commands = {}
        self.keys = []
        self.drop = False
        self.large_response = False
        parent = self
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, format, *args): pass
            def do_POST(self):
                body = self.rfile.read(int(self.headers['Content-Length']))
                if self.path == '/token':
                    result = {'access_token': 'test-token'}
                else:
                    key = self.headers['Idempotency-Key']
                    parent.keys.append(key)
                    if key in parent.commands and parent.commands[key][0] != body:
                        self.send_error(409); return
                    parent.commands.setdefault(key, (body, {'id': str(len(parent.commands)+1), 'state':'pending', 'padding': 'x' * (200000 if parent.large_response else 0)}))
                    result = parent.commands[key][1]
                    if parent.drop:
                        parent.drop = False
                        self.connection.shutdown(socket.SHUT_RDWR)
                        self.connection.close()
                        return
                data = json.dumps(result).encode()
                self.send_response(200); self.send_header('Content-Length',str(len(data))); self.end_headers(); self.wfile.write(data)
        self.server = ThreadingHTTPServer(('127.0.0.1',0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever); self.thread.start()
        d = Path(self.tmp.name); (d/'secret').write_text('test-secret')
        base = 'http://127.0.0.1:'+str(self.server.server_port)
        self.config = d/'config'; self.config.write_text(json.dumps({'base':base,'token_url':base+'/token','client_id':'test','secret_file':str(d/'secret')}))
        self.env = dict(os.environ, WAZE_IRIS_CONFIG=str(self.config))
    def tearDown(self):
        self.server.shutdown(); self.server.server_close(); self.thread.join(); self.tmp.cleanup()
    def run_cli(self, action='send', text='private destination'):
        return subprocess.run([sys.executable, SCRIPT, action], input=text, capture_output=True, text=True, env=self.env, timeout=10)
    def test_commit_then_lost_response_and_successful_replay_stays_recoverable(self):
        self.drop = True
        self.assertNotEqual(self.run_cli().returncode, 0)
        replay = self.run_cli()
        self.assertEqual(replay.returncode, 0)
        self.assertNotIn('state', json.loads(replay.stdout))
        self.assertEqual(json.loads(replay.stdout)['state_at_creation'], 'pending')
        self.assertEqual(len(self.commands), 1)
        self.assertEqual(len(set(self.keys)), 1)
        state = (self.config.parent/(self.config.name+'.pending')).read_text()
        self.assertNotIn('private destination', state)
        self.assertNotIn('test-secret', state)
        self.assertEqual(self.run_cli().returncode, 0)
        self.assertEqual(len(self.commands), 2)
    def test_sequential_successful_sends_create_distinct_commands(self):
        first = self.run_cli()
        second = self.run_cli()
        self.assertEqual(first.returncode, 0)
        self.assertEqual(second.returncode, 0)
        self.assertNotEqual(json.loads(first.stdout)['id'], json.loads(second.stdout)['id'])
        self.assertEqual(len(self.commands), 2)

    def test_concurrent_invocations_share_one_operation(self):
        self.large_response = True
        procs = [subprocess.Popen([sys.executable,SCRIPT,'send'],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=self.env) for _ in range(2)]
        procs[0].stdin.write(b'private destination'); procs[0].stdin.close(); procs[0].stdin = None
        self.assertTrue(select.select([procs[0].stdout], [], [], 10)[0])
        procs[1].stdin.write(b'private destination'); procs[1].stdin.close(); procs[1].stdin = None
        time.sleep(0.5)  # first process is blocked writing the large response
        for p in procs:
            p.communicate(timeout=10)
            self.assertEqual(p.returncode,0)
        self.assertEqual(len(self.commands),1)

    def test_kill_during_stdout_replays_and_then_allows_new_operation(self):
        self.large_response = True
        p = subprocess.Popen([sys.executable, SCRIPT, 'send'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=self.env)
        p.stdin.write(b'private destination'); p.stdin.close()
        self.assertTrue(select.select([p.stdout], [], [], 10)[0])
        p.kill(); p.wait(timeout=10)
        p.stdout.close(); p.stderr.close()
        self.assertNotEqual(self.run_cli(text='different destination').returncode, 0)
        self.assertEqual(self.run_cli().returncode, 0)
        self.assertEqual(len(self.commands), 1)
        self.assertEqual(self.run_cli(text='different destination').returncode, 0)
        self.assertEqual(len(self.commands), 2)

    def test_broken_stdout_replays_same_key(self):
        self.large_response = True
        p = subprocess.Popen([sys.executable, SCRIPT, 'send'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=self.env)
        p.stdout.close()
        p.communicate(b'private destination', timeout=10)
        self.assertNotEqual(p.returncode, 0)
        self.assertEqual(self.run_cli().returncode, 0)
        self.assertEqual(len(self.commands), 1)
        self.assertEqual(len(set(self.keys)), 1)
