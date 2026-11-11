import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

RECEIVER = Path(__file__).resolve().parents[1] / 'tools/gps_push_receiver.py'
spec = importlib.util.spec_from_file_location('gps_receiver_security', RECEIVER)
receiver = importlib.util.module_from_spec(spec)
spec.loader.exec_module(receiver)

class SnapshotSecurityTest(unittest.TestCase):
    def test_requires_ca_and_hostname(self):
        with patch.dict(os.environ, {'GPS_TLS_CERT':'x', 'GPS_TLS_KEY':'x'}, clear=True):
            with self.assertRaises(ValueError):
                receiver.make_tls_context()

    def test_snapshot_cannot_be_mutated_between_verification_and_load(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d)
            subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','1','-subj','/CN=gps.test','-addext','subjectAltName=DNS:gps.test','-keyout',str(p/'key'),'-out',str(p/'ca')], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            (p/'leaf').write_bytes((p/'ca').read_bytes() + (p/'key').read_bytes())
            env = {'GPS_TLS_CERT':str(p/'leaf'), 'GPS_TLS_KEY':str(p/'leaf'), 'GPS_TLS_CA_FILE':str(p/'ca'), 'GPS_TLS_HOSTNAME':'gps.test'}
            real_run = receiver.subprocess.run
            def attack(*args, **kwargs):
                result = real_run(*args, **kwargs)
                if args[0][1] == 'verify':
                    (p/'leaf').write_bytes(b'corrupted')
                    with self.assertRaises(OSError):
                        with open(args[0][-1], 'wb') as f:
                            f.write(b'corrupted')
                return result
            with patch.dict(os.environ, env, clear=True), patch.object(receiver.subprocess, 'run', side_effect=attack):
                self.assertIsNotNone(receiver.make_tls_context())
