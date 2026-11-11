import gzip
import hashlib
import importlib.util
import json
import pathlib
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('municipios_generator', pathlib.Path(__file__).resolve().parents[1] / 'tools/gerar_municipios.py')
gen = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gen)


def fixture():
    prefixes = list(gen.UFS)
    return [{'id': int(prefixes[i % len(prefixes)] + f'{i:05d}'), 'nome': 'Cidade Á ' + str(i),
             'regiao-imediata': {'regiao-intermediaria': {'UF': {'sigla': gen.UFS[prefixes[i % len(prefixes)]]}}}}
            for i in range(5571)]


class ProvisionamentoTest(unittest.TestCase):
    def test_valid_and_idempotent(self):
        rows = fixture()
        result = gen.convert(gzip.compress(json.dumps(rows, ensure_ascii=False).encode()))
        with tempfile.TemporaryDirectory() as tmp:
            path = pathlib.Path(tmp) / 'index.json'
            gen.write_index(path, result)
            before = path.read_bytes()
            gen.write_index(path, result)
            self.assertEqual(before, path.read_bytes())
            envelope = json.loads(before)
            self.assertEqual(envelope['version'], 1)
            self.assertEqual(envelope['count'], 5571)
            rows_json = json.dumps(envelope['municipalities'], ensure_ascii=False, separators=(',', ':')).encode()
            self.assertEqual(envelope['sha256'], hashlib.sha256(rows_json).hexdigest())
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            with self.assertRaises(ValueError):
                gen.write_index(path, b'different')
            self.assertEqual(before, path.read_bytes())

    def test_reject_invalid(self):
        for change in ('short', 'duplicate', 'uf', 'name', 'utf8', 'oversize', 'missing_state'):
            rows = fixture()
            if change == 'short': rows.pop()
            if change == 'duplicate': rows[1]['id'] = rows[0]['id']
            if change == 'uf': rows[0]['regiao-imediata']['regiao-intermediaria']['UF']['sigla'] = 'ZZ'
            if change == 'name': rows[0]['nome'] = ''
            if change == 'missing_state': rows = [r for r in rows if str(r['id'])[:2] != '11']
            payload = json.dumps(rows, ensure_ascii=False).encode()
            if change == 'utf8': payload = b'\xff'
            if change == 'oversize': payload = b' ' * (gen.MAX_INPUT + 1)
            with self.subTest(change=change), self.assertRaises(ValueError):
                gen.convert(payload)

    def test_redirects_rejected(self):
        handler = gen.NoRedirect()
        with self.assertRaises(Exception):
            handler.redirect_request(None, None, 302, 'redirect', {}, 'https://example.org/')

if __name__ == '__main__': unittest.main()
