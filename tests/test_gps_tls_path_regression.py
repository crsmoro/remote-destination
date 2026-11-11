import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

TOOL = Path(__file__).resolve().parents[1] / 'tools/gps_tls.py'
BUNDLE = '/etc/ssl/certs/ca-certificates.crt'


class PathSecurityTest(unittest.TestCase):
    def test_init_rejects_symlinked_pki_without_touching_target(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            victim = root / 'victim'
            victim.mkdir(mode=0o755)
            (root / 'pki').symlink_to(victim, target_is_directory=True)
            result = subprocess.run([sys.executable, str(TOOL), 'init', '--dir', str(root / 'pki'), '--hostname', 'gps.internal.test', '--certifi', BUNDLE], capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(victim.stat().st_mode & 0o777, 0o755)
            self.assertEqual(list(victim.iterdir()), [])

    def test_replacement_during_init_does_not_write_into_replacement(self):
        import importlib.util
        from unittest.mock import patch
        spec = importlib.util.spec_from_file_location('gps_tls_path_race', TOOL)
        assert spec and spec.loader
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        with tempfile.TemporaryDirectory() as d:
            base = Path(d)
            pki, moved, victim = base / 'pki', base / 'moved', base / 'victim'
            victim.mkdir()
            original_run = module.run
            attacked = False
            def race(*args, **kwargs):
                nonlocal attacked
                result = original_run(*args, **kwargs)
                if not attacked:
                    attacked = True
                    pki.rename(moved)
                    pki.symlink_to(victim, target_is_directory=True)
                    (victim / Path(args[args.index('-keyout') + 1]).parent.name).symlink_to(moved / Path(args[args.index('-keyout') + 1]).parent.name, target_is_directory=True)
                return result
            argv = [str(TOOL), 'init', '--dir', str(pki), '--hostname', 'gps.internal.test', '--certifi', BUNDLE]
            previous_umask = os.umask(0o077)
            try:
                with patch.object(sys, 'argv', argv), patch.object(module, 'run', race):
                    try:
                        module.main()
                    except (OSError, ValueError, subprocess.SubprocessError):
                        pass
            finally:
                os.umask(previous_umask)
            self.assertTrue(attacked)
            self.assertFalse((victim / 'ca.key').exists())
            self.assertFalse((victim / 'ca.pem').exists())


if __name__ == '__main__':
    unittest.main()
