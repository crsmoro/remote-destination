import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

TOOL = Path(__file__).resolve().parents[1] / 'tools' / 'gps_tls.py'

class ProvisionTest(unittest.TestCase):
    def test_provision_and_hostname_verification(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            bundle = root / 'original.pem'
            bundle.write_bytes(Path('/etc/ssl/certs/ca-certificates.crt').read_bytes())
            result = subprocess.run([sys.executable, str(TOOL), 'init', '--dir', str(root / 'pki'), '--hostname', 'gps.internal.test', '--certifi', str(bundle)], capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            pki = root / 'pki'
            self.assertEqual((pki / 'ca.key').stat().st_mode & 0o777, 0o600)
            self.assertTrue((pki / 'trust.pem').read_bytes().startswith(bundle.read_bytes()))
            import ssl
            original_roots = set(ssl.create_default_context(cafile=str(bundle)).get_ca_certs(binary_form=True))
            combined_roots = set(ssl.create_default_context(cafile=str(pki/'trust.pem')).get_ca_certs(binary_form=True))
            self.assertTrue(original_roots <= combined_roots)
            self.assertGreater(len(combined_roots), len(original_roots))
            import importlib.util
            spec = importlib.util.spec_from_file_location('gps_tls_atomic_test', TOOL)
            assert spec and spec.loader
            mod = importlib.util.module_from_spec(spec); spec.loader.exec_module(mod)
            before = (pki/'current.pem').read_bytes()
            (pki/'ca.key').write_bytes(b'invalid')
            with self.assertRaises(subprocess.CalledProcessError):
                mod.issue(pki, 'gps.internal.test', 90)
            self.assertEqual((pki/'current.pem').read_bytes(), before)
            ok = subprocess.run(['openssl', 'verify', '-CAfile', str(pki / 'ca.pem'), '-verify_hostname', 'gps.internal.test', '-purpose', 'sslserver', str(pki / 'current.pem')], capture_output=True)
            self.assertEqual(ok.returncode, 0)
            wrong = subprocess.run(['openssl', 'verify', '-CAfile', str(pki / 'ca.pem'), '-verify_hostname', 'wrong.internal.test', str(pki / 'current.pem')], capture_output=True)
            self.assertNotEqual(wrong.returncode, 0)
            wrong_ca = subprocess.run(['openssl', 'verify', '-CAfile', str(bundle), '-verify_hostname', 'gps.internal.test', str(pki/'current.pem')], capture_output=True)
            self.assertNotEqual(wrong_ca.returncode, 0)
            self.assertNotIn(b'PRIVATE KEY', result.stdout + result.stderr)

    def test_separate_leaf_volume_and_renew_without_systemd(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d); private = p/'private'; leaf = p/'leaf'; leaf.mkdir()
            base = [sys.executable, str(TOOL)]
            init = subprocess.run(base + ['init', '--dir', str(private), '--output-dir', str(leaf), '--hostname', 'gps.internal.test', '--certifi', '/etc/ssl/certs/ca-certificates.crt'], capture_output=True)
            self.assertEqual(init.returncode, 0, init.stderr)
            self.assertEqual({x.name for x in leaf.iterdir()}, {'current.pem'})
            before = (leaf/'current.pem').read_bytes()
            renew = subprocess.run(base + ['renew', '--dir', str(private), '--output-dir', str(leaf)], capture_output=True)
            self.assertEqual(renew.returncode, 0, renew.stderr)
            self.assertEqual((leaf/'current.pem').read_bytes(), before)

    def test_periodic_renewer_once_reports_ca_expiry(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d); leaf = p/'leaf'; leaf.mkdir(); root = p/'pki'
            subprocess.run([sys.executable, str(TOOL), 'init', '--dir', str(root), '--output-dir', str(leaf), '--hostname', 'gps.internal.test', '--certifi', '/etc/ssl/certs/ca-certificates.crt'], check=True, capture_output=True)
            env = dict(__import__('os').environ, GPS_PKI_DIR=str(root), GPS_LEAF_DIR=str(leaf), GPS_CA_ALERT_DAYS='4000')
            result = subprocess.run([sys.executable, str(TOOL.parent/'gps_tls_scheduler.py'), '--once'], env=env, capture_output=True, timeout=30)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b'CA', result.stderr)

    def test_lock_and_timeout_fail_closed(self):
        import fcntl, importlib.util, os
        spec = importlib.util.spec_from_file_location('gps_tls_test', TOOL)
        assert spec and spec.loader
        module = importlib.util.module_from_spec(spec); spec.loader.exec_module(module)
        with self.assertRaises(subprocess.TimeoutExpired):
            module.run(sys.executable, '-c', 'import time; time.sleep(2)', timeout=0.1)
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            with (root/'.lock').open('w') as lock:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
                result = subprocess.run([sys.executable, str(TOOL), 'init', '--dir', str(root), '--hostname', 'gps.internal.test', '--certifi', '/etc/ssl/certs/ca-certificates.crt'], capture_output=True, timeout=5)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse((root/'ca.key').exists())
            self.assertNotIn(b'PRIVATE KEY', result.stderr)

    def test_receiver_reload_preserves_active_on_invalid_and_swaps_valid(self):
        import os, signal, socket, ssl, time, importlib.util
        with tempfile.TemporaryDirectory() as d:
            p = Path(d); root = p / 'pki'; original = '/etc/ssl/certs/ca-certificates.crt'
            subprocess.run([sys.executable, str(TOOL), 'init', '--dir', str(root), '--hostname', 'gps.internal.test', '--certifi', original, '--days', '1', '--renew-before-days', '0'], check=True, capture_output=True)
            (p / 'secret').write_text('x' * 43)
            with socket.socket() as s:
                s.bind(('127.0.0.1', 0)); port = s.getsockname()[1]
            env = dict(os.environ, GPS_TLS_RELOAD_SECONDS='1', GPS_TLS_CA_FILE=str(root/'ca.pem'), GPS_TLS_HOSTNAME='gps.internal.test', GPS_LISTEN_IP='127.0.0.1', GPS_PORT=str(port), GPS_SECRET_FILE=str(p/'secret'), GPS_TLS_CERT=str(root/'current.pem'), GPS_TLS_KEY=str(root/'current.pem'), GPS_SNAPSHOT_FILE=str(p/'snapshot'))
            proc = subprocess.Popen([sys.executable, str(TOOL.parent/'gps_push_receiver.py')], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            def peer():
                ctx = ssl.create_default_context(cafile=str(root/'trust.pem'))
                with socket.create_connection(('127.0.0.1', port), timeout=2) as sock:
                    with ctx.wrap_socket(sock, server_hostname='gps.internal.test') as tls:
                        return tls.getpeercert(binary_form=True)
            try:
                for i in range(50):
                    try: before = peer(); break
                    except OSError: time.sleep(.05)
                else: self.fail('receiver did not start')
                old = (root/'current.pem').read_bytes()
                (root/'current.pem').write_bytes(b'invalid')
                time.sleep(1.5)
                self.assertEqual(peer(), before)
                (root/'current.pem').write_bytes(old)
                other = p/'other'
                subprocess.run([sys.executable, str(TOOL), 'init', '--dir', str(other), '--hostname', 'gps.internal.test', '--certifi', original], check=True, capture_output=True)
                replacement = root/'replacement'
                replacement.write_bytes((other/'current.pem').read_bytes())
                os.replace(replacement, root/'current.pem')
                time.sleep(1.5)
                self.assertEqual(peer(), before, 'untrusted issuer replaced active context')
                (root/'current.pem').write_bytes(old)
                # Renewal must be detected without signaling the receiver.
                ca_before = (root/'ca.pem').read_bytes()
                result = subprocess.run([sys.executable, str(TOOL), 'renew', '--dir', str(root), '--renew-before-days', '2'], capture_output=True, timeout=30)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual((root/'ca.pem').read_bytes(), ca_before)
                for i in range(50):
                    if peer() != before: break
                    time.sleep(.05)
                else: self.fail('certificate did not reload')
                self.assertIsNone(proc.poll())
            finally:
                proc.terminate(); out, err = proc.communicate(timeout=5)
            self.assertNotIn(b'PRIVATE KEY', out + err)
            self.assertNotIn(b'x' * 43, out + err)
            self.assertIn(b'ALERT: GPS TLS reload rejected', err)
            self.assertIn(b'ALERT: GPS active TLS certificate expires', err)

if __name__ == '__main__': unittest.main()
