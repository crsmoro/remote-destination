import json
import tempfile
import time
import unittest
from pathlib import Path


class ReceiverTest(unittest.TestCase):
    def test_restart_rejects_malformed_complete_snapshot(self):
        import importlib.util
        spec = importlib.util.spec_from_file_location('gps_receiver_schema', Path(__file__).resolve().parents[1] / 'tools/gps_push_receiver.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        malformed = [
            {'sent_at': 123, 'available': 'yes'}, {'sent_at': True, 'available': False},
            {'sent_at': -1, 'available': False}, {'sent_at': 123, 'available': False, 'latitude': 0},
            {'sent_at': 123, 'available': True}, {'sent_at': 123, 'available': True, 'latitude': 91, 'longitude': 0},
            {'sent_at': 123, 'available': True, 'latitude': 0, 'longitude': False},
            {'sent_at': 123, 'available': False, 'version': 1},
            {'sent_at': 123}, [], {'sent_at': float('nan'), 'available': False},
        ]
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / 'position.json'
            for state in malformed:
                with self.subTest(state=state):
                    path.write_text(json.dumps(state))
                    with self.assertRaises((ValueError, TypeError)):
                        module.Receiver(path, 'z' * 43)
            path.write_text('{"sent_at":123,"sent_at":124,"available":false}')
            with self.assertRaises(ValueError):
                module.Receiver(path, 'z' * 43)

    def test_authenticated_snapshot_and_replay(self):
        import importlib.util
        spec = importlib.util.spec_from_file_location('gps_push_receiver', Path(__file__).resolve().parents[1] / 'tools/gps_push_receiver.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        Receiver = module.Receiver
        with tempfile.TemporaryDirectory() as d:
            r = Receiver(Path(d) / 'position.json', 'x' * 43)
            now = time.time()
            body = json.dumps({'sent_at': now, 'available': True, 'latitude': -26.3, 'longitude': -48.8}).encode()
            self.assertEqual(r.process('wrong', body, now), 401)
            self.assertEqual(r.process('Bearer ' + 'x' * 43, body, now), 204)
            self.assertEqual(json.loads((Path(d) / 'position.json').read_text())['latitude'], -26.3)
            self.assertEqual(r.process('Bearer ' + 'x' * 43, body, now), 409)

    def test_validation_restart_and_races(self):
        import importlib.util
        from concurrent.futures import ThreadPoolExecutor
        spec = importlib.util.spec_from_file_location('gps_receiver_test', Path(__file__).resolve().parents[1] / 'tools/gps_push_receiver.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        auth = 'Bearer ' + 'z' * 43
        now = time.time()
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / 'position.json'
            r = module.Receiver(path, 'z' * 43)
            invalid = [b'{}', b'{"available":true,"available":false}', b'[]', b'NaN', b'x' * 513,
                       json.dumps({'sent_at': now - 121, 'available': False}).encode(),
                       json.dumps({'sent_at': now + 11, 'available': False}).encode(),
                       json.dumps({'sent_at': now, 'available': True, 'latitude': 91, 'longitude': 0}).encode(),
                       json.dumps({'sent_at': now, 'available': False, 'latitude': 0}).encode()]
            for body in invalid:
                self.assertEqual(r.process(auth, body, now), 400)
            self.assertFalse(path.exists())
            payloads = [json.dumps({'sent_at': now + i / 100, 'available': False}).encode() for i in range(30)]
            with ThreadPoolExecutor(max_workers=12) as pool:
                results = list(pool.map(lambda b: r.process(auth, b, now), reversed(payloads)))
            self.assertIn(204, results)
            self.assertEqual(json.loads(path.read_text())['sent_at'], now + .29)
            restarted = module.Receiver(path, 'z' * 43)
            self.assertEqual(restarted.process(auth, payloads[-1], now), 409)
            self.assertEqual(json.loads(path.read_text())['available'], False)
            self.assertNotIn('z' * 43, path.read_text())
            path.write_text('bad')
            with self.assertRaises(ValueError):
                module.Receiver(path, 'z' * 43)

    def test_tls_http_and_no_sensitive_logs(self):
        import http.client
        import os
        import socket
        import ssl
        import subprocess
        import sys
        with tempfile.TemporaryDirectory() as d:
            p = Path(d)
            subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1', '-subj', '/CN=localhost', '-addext', 'subjectAltName=DNS:localhost', '-keyout', str(p/'key'), '-out', str(p/'cert')], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            (p/'secret').write_text('q' * 43)
            with socket.socket() as sock:
                sock.bind(('127.0.0.1', 0))
                port = sock.getsockname()[1]
            (p/'leaf').write_bytes((p/'cert').read_bytes() + (p/'key').read_bytes())
            env = dict(os.environ, GPS_LISTEN_IP='127.0.0.1', GPS_PORT=str(port), GPS_SECRET_FILE=str(p/'secret'), GPS_TLS_CERT=str(p/'leaf'), GPS_TLS_KEY=str(p/'leaf'), GPS_TLS_CA_FILE=str(p/'cert'), GPS_TLS_HOSTNAME='localhost', GPS_SNAPSHOT_FILE=str(p/'snapshot'))
            code = '''import importlib.util, json, os, signal, sys, time
s = importlib.util.spec_from_file_location("receiver", sys.argv[1]); m = importlib.util.module_from_spec(s); s.loader.exec_module(m)
r = m.Receiver(sys.argv[2], "q" * 43)
def crash(*args): os.kill(os.getpid(), signal.SIGKILL)
m.os.replace = crash
r.process("Bearer " + "q" * 43, json.dumps({"sent_at": time.time(), "available": True, "latitude": 1, "longitude": 2}).encode())'''
            child = subprocess.run([sys.executable, '-c', code, str(Path(__file__).resolve().parents[1]/'tools/gps_push_receiver.py'), str(p/'snapshot')])
            self.assertNotEqual(child.returncode, 0)
            leftovers = list(p.glob('.position-*'))
            self.assertEqual(len(leftovers), 1)
            self.assertEqual(leftovers[0].stat().st_mode & 0o777, 0o600)
            (p/'.position-not-ours').write_text('keep')
            (p/'.position-abcdefgh').symlink_to(p/'secret')
            proc = subprocess.Popen([sys.executable, str(Path(__file__).resolve().parents[1]/'tools/gps_push_receiver.py')], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            try:
                ctx = ssl.create_default_context(cafile=str(p/'cert'))
                for attempt in range(100):
                    try:
                        conn = http.client.HTTPSConnection('localhost', port, context=ctx, timeout=1)
                        conn.connect()
                        break
                    except OSError:
                        time.sleep(.03)
                else:
                    self.fail('receiver did not start')
                self.assertFalse(leftovers[0].exists())
                self.assertTrue((p/'.position-abcdefgh').is_symlink())
                self.assertEqual((p/'.position-not-ours').read_text(), 'keep')
                self.assertEqual((p/'secret').read_text(), 'q' * 43)
                conn.close()
                idle = socket.create_connection(('127.0.0.1', port), timeout=1)
                time.sleep(.1)
                conn = http.client.HTTPSConnection('localhost', port, context=ctx, timeout=1)
                body = json.dumps({'sent_at': time.time(), 'available': True, 'latitude': -23.1234, 'longitude': -45.1234})
                conn.request('POST', '/position', body, {'Authorization': 'Bearer ' + 'q' * 43})
                response = conn.getresponse()
                self.assertEqual(response.status, 204)
                response.read()
                conn.close()
                idle.close()
                self.assertTrue((p/'snapshot').exists())
                env2 = dict(env, GPS_PORT='0')
                second = subprocess.run([sys.executable, str(Path(__file__).resolve().parents[1]/'tools/gps_push_receiver.py')], env=env2, capture_output=True, timeout=3)
                self.assertNotEqual(second.returncode, 0)
                stalled = []
                for _ in range(16):
                    stalled.append(socket.create_connection(('127.0.0.1', port), timeout=1))
                    time.sleep(.03)
                time.sleep(.3)
                extra = socket.create_connection(('127.0.0.1', port), timeout=1)
                extra.settimeout(2)
                self.assertEqual(extra.recv(1), b'')
                extra.close()
                time.sleep(5.2)
                for s in stalled:
                    s.settimeout(1)
                    self.assertEqual(s.recv(1), b'')
                    s.close()
                conn = http.client.HTTPSConnection('localhost', port, context=ctx, timeout=2)
                conn.request('POST', '/position', json.dumps({'sent_at': time.time(), 'available': False}), {'Authorization': 'Bearer ' + 'q' * 43})
                self.assertEqual(conn.getresponse().status, 204)
                conn.close()
                slow = ctx.wrap_socket(socket.create_connection(('127.0.0.1', port), timeout=2), server_hostname='localhost')
                slow.settimeout(2)
                slow.sendall(b'POST /position HTTP/1.1\r\nX-Slow: ')
                start = time.monotonic()
                closed = False
                for _ in range(13):
                    time.sleep(1)
                    try:
                        slow.sendall(b'x')
                    except OSError:
                        closed = True
                        break
                slow.close()
                self.assertTrue(closed, 'drip-fed headers exceeded the absolute deadline')
                self.assertLess(time.monotonic() - start, 13)
            finally:
                proc.terminate()
                out, err = proc.communicate(timeout=5)
            for sensitive in (b'q' * 43, b'-23.1234', b'-45.1234'):
                self.assertNotIn(sensitive, out + err)

if __name__ == '__main__':
    unittest.main()
