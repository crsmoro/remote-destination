#!/usr/bin/env python3
"""Run the actual Alpine nginx binary and project proxy config on loopback.

Set NGINX_IMAGE_ROOT to an extracted nginx:1.27-alpine runtime and PKI_DIR
 to a private provisioned PKI directory. Does not start or alter production.
"""
import http.server
import os
from pathlib import Path
import re
import socket
import ssl
import subprocess
import tempfile
import threading
import time
import unittest

ROOT = Path(os.environ.get('NGINX_IMAGE_ROOT', '/nonexistent'))
PKI = Path(os.environ.get('PKI_DIR', '/nonexistent'))
PROXY = Path(__file__).resolve().parents[1] / 'proxy.conf'

class Backend(http.server.BaseHTTPRequestHandler):
    count = 0
    def do_GET(self):
        type(self).count += 1
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'isolated backend')
    def log_message(self, format, *args):
        pass

class NginxMTLS(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        if not (ROOT / 'usr/sbin/nginx').is_file() or not (PKI / 'proxy/server.key').is_file():
            raise unittest.SkipTest('Set NGINX_IMAGE_ROOT and PKI_DIR to the nginx runtime and provisioned PKI')
        cls.tmp = tempfile.TemporaryDirectory(dir=os.environ.get('TMPDIR'))
        t = Path(cls.tmp.name)
        cls.backend = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Backend)
        threading.Thread(target=cls.backend.serve_forever, daemon=True).start()
        sock = socket.socket()
        sock.bind(('127.0.0.1', 0))
        cls.port = sock.getsockname()[1]
        sock.close()
        config = PROXY.read_text()
        config = config.replace('listen 8443 ssl;', f'listen 127.0.0.1:{cls.port} ssl;')
        config = config.replace('http://backend:8080', f'http://127.0.0.1:{cls.backend.server_port}')
        config = config.replace('/etc/waze-pki/', str(PKI / 'proxy') + '/')
        for directive in ('pid', 'client_body_temp_path', 'proxy_temp_path', 'fastcgi_temp_path', 'uwsgi_temp_path', 'scgi_temp_path'):
            config, n = re.subn(r'(?m)^(\s*' + directive + r')\s+[^;]+;', lambda m: m.group(1) + ' ' + str(t / directive) + ';', config)
            if n != 1:
                raise RuntimeError('unexpected proxy config directive: ' + directive)
        conf = t / 'nginx.conf'
        conf.write_text(config)
        cls.cmd = [str(ROOT / 'lib/ld-musl-x86_64.so.1'), '--library-path', str(ROOT / 'usr/lib') + ':' + str(ROOT / 'lib'), str(ROOT / 'usr/sbin/nginx'), '-p', str(t), '-c', str(conf), '-g', 'daemon off;']
        subprocess.run(cls.cmd[:-2] + ['-t'], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        cls.nginx = subprocess.Popen(cls.cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        for _ in range(100):
            if cls.nginx.poll() is not None:
                raise RuntimeError('nginx exited before readiness')
            try:
                with socket.create_connection(('127.0.0.1', cls.port), timeout=.1):
                    break
            except OSError:
                time.sleep(.05)
        else:
            raise RuntimeError('nginx failed to listen')
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-noenc', '-days', '1', '-subj', '/CN=untrusted', '-keyout', str(t/'bad.key'), '-out', str(t/'bad.crt')], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        cls.bad = t

    @classmethod
    def tearDownClass(cls):
        cls.nginx.terminate()
        cls.nginx.wait(timeout=5)
        cls.backend.shutdown()
        cls.backend.server_close()
        cls.tmp.cleanup()

    def request(self, host='proxy', ca='server-ca.crt', cert: str | None = 'good'):
        ctx = ssl.create_default_context(cafile=str(PKI/'ca'/ca))
        if cert == 'good':
            ctx.load_cert_chain(str(PKI/'traefik/client.crt'), str(PKI/'traefik/client.key'))
        elif cert == 'bad':
            ctx.load_cert_chain(str(self.bad/'bad.crt'), str(self.bad/'bad.key'))
        with socket.create_connection(('127.0.0.1', self.port), timeout=3) as sock:
            with ctx.wrap_socket(sock, server_hostname=host) as tls:
                tls.sendall(b'GET / HTTP/1.0\r\nHost: proxy\r\n\r\n')
                return tls.recv(4096).split(b'\r\n', 1)[0]

    def test_actual_nginx_uid(self):
        status = Path(f'/proc/{self.nginx.pid}/status').read_text()
        uid_line = next(line for line in status.splitlines() if line.startswith('Uid:'))
        self.assertEqual(int(uid_line.split()[1]), 10000)
        self.assertEqual(os.getuid(), 10000)

    def test_valid_client_reaches_backend(self):
        before = Backend.count
        self.assertIn(b' 200 ', self.request())
        self.assertEqual(Backend.count, before + 1)

    def test_wrong_hostname_rejected(self):
        before = Backend.count
        with self.assertRaises(ssl.SSLCertVerificationError):
            self.request(host='wrong.invalid')
        self.assertEqual(Backend.count, before)

    def test_untrusted_server_rejected(self):
        before = Backend.count
        with self.assertRaises(ssl.SSLCertVerificationError):
            self.request(ca='client-ca.crt')
        self.assertEqual(Backend.count, before)

    def test_no_client_cert_rejected(self):
        before = Backend.count
        self.assertIn(b' 400 ', self.request(cert=None))
        self.assertEqual(Backend.count, before)

    def test_untrusted_client_rejected(self):
        before = Backend.count
        self.assertIn(b' 400 ', self.request(cert='bad'))
        self.assertEqual(Backend.count, before)

if __name__ == '__main__':
    unittest.main()
