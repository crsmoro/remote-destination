import importlib.util
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

TOOL = Path(__file__).resolve().parents[1] / 'tools/gps_tls.py'
BUNDLE = Path('/etc/ssl/certs/ca-certificates.crt')


def module():
    spec = importlib.util.spec_from_file_location('gps_tls_audit', TOOL)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


class AuditRegression(unittest.TestCase):
    def test_scheduler_alerts_when_ca_key_missing_even_if_not_due(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)/'pki'; leaf = Path(d)/'leaf'; leaf.mkdir()
            subprocess.run([sys.executable, str(TOOL), 'init', '--dir', str(root), '--output-dir', str(leaf), '--hostname', 'gps.internal.test', '--certifi', str(BUNDLE)], check=True, capture_output=True)
            (root/'ca.key').unlink()
            env = dict(os.environ, GPS_PKI_DIR=str(root), GPS_LEAF_DIR=str(leaf))
            result = subprocess.run([sys.executable, str(TOOL.parent/'gps_tls_scheduler.py'), '--once'], env=env, capture_output=True, timeout=30)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b'CA private key', result.stderr)

    def test_issue_rejects_readable_ca_private_key(self):
        mod = module()
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)/'pki'
            subprocess.run([sys.executable, str(TOOL), 'init', '--dir', str(root), '--hostname', 'gps.internal.test', '--certifi', str(BUNDLE)], check=True, capture_output=True)
            before = (root/'current.pem').read_bytes()
            (root/'ca.key').chmod(0o644)
            with self.assertRaises(ValueError):
                mod.issue(root, 'gps.internal.test', 90)
            self.assertEqual((root/'current.pem').read_bytes(), before)

    def test_bundle_symlink_is_rejected(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d); source = root/'bundle'; source.symlink_to(BUNDLE)
            result = subprocess.run([sys.executable, str(TOOL), 'init', '--dir', str(root/'pki'), '--hostname', 'gps.internal.test', '--certifi', str(source)], capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse((root/'pki'/'trust.pem').exists())

    def test_bundle_swap_after_validation_preserves_exact_original(self):
        mod = module()
        with tempfile.TemporaryDirectory() as d:
            root = Path(d); source = root/'original.pem'; pki = root/'pki'
            original = BUNDLE.read_bytes(); source.write_bytes(original)
            validate = mod.ssl.create_default_context
            def swap(*args, **kwargs):
                result = validate(*args, **kwargs)
                replacement = root/'replacement'; replacement.write_bytes(b'INJECTED\n')
                os.replace(replacement, source)
                return result
            argv = [str(TOOL), 'init', '--dir', str(pki), '--hostname', 'gps.internal.test', '--certifi', str(source)]
            previous_umask = os.umask(0o077)
            os.umask(previous_umask)
            try:
                with patch.object(sys, 'argv', argv), patch.object(mod.ssl, 'create_default_context', side_effect=swap):
                    mod.main()
            finally:
                os.umask(previous_umask)
            self.assertEqual((pki/'trust.pem').read_bytes(), original + (b'' if original.endswith(b'\n') else b'\n') + (pki/'ca.pem').read_bytes())
