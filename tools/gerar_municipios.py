#!/usr/bin/env python3
"""Gera índice privado, offline em uso, da API de localidades do IBGE."""
import gzip
import hashlib
import unicodedata
import io
import json
import os
import pathlib
import sys
import tempfile
import urllib.request
import urllib.error

URL = 'https://servicodados.ibge.gov.br/api/v1/localidades/municipios'
MAX_INPUT = 8 * 1024 * 1024
EXPECTED_COUNT = 5571  # Revisar deliberadamente quando o IBGE mudar a relação.
UFS = dict(zip(('11 12 13 14 15 16 17 21 22 23 24 25 26 27 28 29 '
                '31 32 33 35 41 42 43 50 51 52 53').split(),
               ('RO AC AM RR PA AP TO MA PI CE RN PB PE AL SE BA MG ES RJ SP '
                'PR SC RS MS MT GO DF').split()))


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError('redirecionamento recusado')


def convert(payload):
    try:
        if len(payload) > MAX_INPUT:
            raise ValueError('resposta grande demais')
        if payload.startswith(b'\x1f\x8b'):
            with gzip.GzipFile(fileobj=io.BytesIO(payload)) as stream:
                payload = stream.read(MAX_INPUT + 1)
        if len(payload) > MAX_INPUT:
            raise ValueError('resposta descomprimida grande demais')
        data = json.loads(payload.decode('utf-8', errors='strict'))
        if not isinstance(data, list) or len(data) != EXPECTED_COUNT:
            raise ValueError('quantidade inesperada de municípios')
        rows, seen, states = [], set(), set()
        for m in data:
            ident, name = m['id'], m['nome']
            immediate = m.get('regiao-imediata') or {}
            intermediate = immediate.get('regiao-intermediaria') or {}
            micro = m.get('microrregiao') or {}
            meso = micro.get('mesorregiao') or {}
            uf = (intermediate.get('UF') or meso.get('UF') or {}).get('sigla')
            if (type(ident) is not int or not 1000000 <= ident <= 9999999 or ident in seen
                    or not isinstance(name, str) or not name.strip() or len(name) > 120
                    or name != name.strip()
                    or any(unicodedata.category(c) in ('Cc', 'Cs') or c == '\ufffd' for c in name)
                    or uf != UFS.get(str(ident)[:2])):
                raise ValueError('registro inválido')
            seen.add(ident)
            states.add(uf)
            rows.append({'id': ident, 'nome': name, 'uf': uf})
        if states != set(UFS.values()):
            raise ValueError('UF ausente')
        rows.sort(key=lambda row: row['id'])
        rows_json = json.dumps(rows, ensure_ascii=False, separators=(',', ':')).encode('utf-8')
        result = (b'{"version":1,"count":' + str(len(rows)).encode()
                  + b',"sha256":"' + hashlib.sha256(rows_json).hexdigest().encode()
                  + b'","municipalities":' + rows_json + b'}\n')
        if len(result) >= 2 << 20:
            raise ValueError('índice excede limite do servidor')
        return result
    except (KeyError, TypeError, AttributeError, UnicodeError, OSError, EOFError) as exc:
        raise ValueError('resposta IBGE inválida') from exc


def write_index(path, content):
    path = pathlib.Path(path)
    fd, tmp = tempfile.mkstemp(prefix='.municipios-', dir=path.parent)
    try:
        with os.fdopen(fd, 'wb') as out:
            out.write(content)
            out.flush()
            os.fsync(out.fileno())
        try:
            os.link(tmp, path, follow_symlinks=False)
        except FileExistsError:
            if path.is_symlink() or not path.is_file() or path.read_bytes() != content:
                raise ValueError('arquivo existente diferente; escolha novo caminho')
    finally:
        os.unlink(tmp)


def main():
    if len(sys.argv) != 2:
        raise SystemExit('uso: python3 tools/gerar_municipios.py /caminho/municipios.json')
    request = urllib.request.Request(URL, headers={'User-Agent': 'waze-remoto-municipios/1.0', 'Accept-Encoding': 'gzip'})
    opener = urllib.request.build_opener(NoRedirect())
    with opener.open(request, timeout=30) as response:
        if response.geturl() != URL:
            raise ValueError('URL inesperada')
        payload = response.read(MAX_INPUT + 1)
    write_index(sys.argv[1], convert(payload))
    print(f'{EXPECTED_COUNT} municípios; fonte IBGE: {URL}')


if __name__ == '__main__':
    main()
