#!/usr/bin/env python3
"""Regression checks for the provisioned PKI; no private material is logged."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('provision.sh')

class ProvisionPKITest(unittest.TestCase):
    def test_strict_purposes_and_certificate_extensions(self):
        with tempfile.TemporaryDirectory(dir=os.environ.get('TMPDIR')) as tmp:
            root = Path(tmp) / 'pki'
            subprocess.run([str(SCRIPT), str(root)], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            for ca, leaf, purpose in [('server', 'proxy/server.crt', 'sslserver'), ('client', 'traefik/client.crt', 'sslclient')]:
                cert = root / 'ca' / (ca + '-ca.crt')
                result = subprocess.run(['openssl', 'verify', '-x509_strict', '-purpose', purpose, '-CAfile', str(cert), str(root / leaf)], text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, f'{purpose} strict verification failed: {result.stderr}')
                text = subprocess.check_output(['openssl', 'x509', '-in', str(cert), '-noout', '-text'], text=True)
                self.assertIn('X509v3 Basic Constraints: critical\n                CA:TRUE', text)
                self.assertIn('X509v3 Key Usage: critical\n                Certificate Sign, CRL Sign', text)
                leaftext = subprocess.check_output(['openssl', 'x509', '-in', str(root / leaf), '-noout', '-text'], text=True)
                self.assertIn('TLS Web Server Authentication' if ca == 'server' else 'TLS Web Client Authentication', leaftext)
                opposite = 'sslclient' if purpose == 'sslserver' else 'sslserver'
                wrong = subprocess.run(['openssl', 'verify', '-x509_strict', '-purpose', opposite, '-CAfile', str(cert), str(root / leaf)], capture_output=True)
                self.assertNotEqual(wrong.returncode, 0, f'{ca} certificate accepted for {opposite}')

if __name__ == '__main__':
    unittest.main()
