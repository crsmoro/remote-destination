import os, pathlib, subprocess, tempfile, unittest

SCRIPT = pathlib.Path(__file__).resolve().parents[1] / "scripts" / "stage-mtls-leaf.sh"

def run(*args, **kw):
    return subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, **kw)

class StageTest(unittest.TestCase):
    def test_issue_strict_server_leaf_with_existing_key_and_ca(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = pathlib.Path(tmp)
            def openssl(*args):
                r = run("openssl", *map(str,args), cwd=p)
                self.assertEqual(r.returncode, 0, r.stderr)
            openssl("req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", "ca.key", "-out", "ca.crt", "-subj", "/CN=Test CA", "-days", "30", "-addext", "basicConstraints=critical,CA:TRUE", "-addext", "keyUsage=critical,keyCertSign,cRLSign")
            openssl("genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048", "-out", "leaf.key")
            r=run("bash",str(SCRIPT),"server",str(p/"ca.crt"),str(p/"ca.key"),str(p/"leaf.key"),str(p/"out"),"proxy","7")
            self.assertEqual(r.returncode, 0, r.stderr)
            self.assertEqual(run("openssl","verify","-x509_strict","-purpose","sslserver","-verify_hostname","proxy","-CAfile",str(p/"ca.crt"),str(p/"out/leaf.crt")).returncode, 0)
            self.assertEqual((p/"out/leaf.key").stat().st_mode & 0o777,0o600)
            self.assertNotEqual(run("openssl","verify","-x509_strict","-purpose","sslclient","-CAfile",str(p/"ca.crt"),str(p/"out/leaf.crt")).returncode, 0)

    def test_client_leaf_rejects_server_purpose(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = pathlib.Path(tmp)
            r = run('openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', str(p/'ca.key'), '-out', str(p/'ca.crt'), '-subj', '/CN=Test CA', '-days', '30', '-addext', 'basicConstraints=critical,CA:TRUE', '-addext', 'keyUsage=critical,keyCertSign,cRLSign')
            self.assertEqual(r.returncode, 0)
            self.assertEqual(run('openssl', 'genpkey', '-algorithm', 'RSA', '-pkeyopt', 'rsa_keygen_bits:2048', '-out', str(p/'leaf.key')).returncode, 0)
            r = run('bash', str(SCRIPT), 'client', str(p/'ca.crt'), str(p/'ca.key'), str(p/'leaf.key'), str(p/'out'), 'traefik-client', '7')
            self.assertEqual(r.returncode, 0, r.stderr)
            args = ('-x509_strict', '-CAfile', str(p/'ca.crt'), str(p/'out/leaf.crt'))
            self.assertEqual(run('openssl','verify','-purpose','sslclient',*args).returncode,0)
            self.assertNotEqual(run('openssl','verify','-purpose','sslserver',*args).returncode,0)

    def test_bad_ca_key_is_rejected_without_output(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = pathlib.Path(tmp)
            for cn in ('one', 'two'):
                r = run('openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', str(p/(cn+'.key')), '-out', str(p/(cn+'.crt')), '-subj', '/CN='+cn, '-days', '30', '-addext', 'basicConstraints=critical,CA:TRUE', '-addext', 'keyUsage=critical,keyCertSign,cRLSign')
                self.assertEqual(r.returncode, 0)
            r = run('bash', str(SCRIPT), 'server', str(p/'one.crt'), str(p/'two.key'), str(p/'one.key'), str(p/'out'), 'proxy', '7')
            self.assertNotEqual(r.returncode, 0)
            self.assertFalse((p/'out').exists())

    def test_existing_output_is_never_overwritten(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = pathlib.Path(tmp)
            (p/'out').mkdir()
            (p/'out'/'marker').write_text('unchanged')
            r = run('bash', str(SCRIPT), 'server', 'missing', 'missing', 'missing', str(p/'out'), 'proxy', '7')
            self.assertNotEqual(r.returncode, 0)
            self.assertEqual((p/'out'/'marker').read_text(), 'unchanged')

if __name__ == "__main__": unittest.main()
