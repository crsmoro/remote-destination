#!/usr/bin/env python3
"""Iris M2M client. Destination input is read from stdin, never arguments."""
import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path

CONFIG = Path(os.environ.get('WAZE_IRIS_CONFIG', Path.home() / '.config/waze-remoto/iris-m2m.json'))

def request(url, body=None, headers=None):
    req = urllib.request.Request(url, data=body, headers=headers or {}, method='POST' if body is not None else 'GET')
    with urllib.request.urlopen(req, timeout=20) as response:
        return json.load(response)

def main():
    if len(sys.argv) != 2 or sys.argv[1] not in ('status', 'send'):
        raise SystemExit('usage: iris_m2m.py status|send (send reads destination from stdin)')
    action = sys.argv[1]
    destination = sys.stdin.read().strip() if action == 'send' else None
    if action == 'send' and not destination:
        raise SystemExit('destination required on stdin')
    config = json.loads(CONFIG.read_text())
    base = config['base'].rstrip('/')
    credentials = {'grant_type': 'client_credentials', 'client_id': config['client_id'],
                   'client_secret': Path(config['secret_file']).read_text().strip(), 'audience': base + '/hermes-api',
                   'scope': 'destination:write status:read'}
    token = request(config['token_url'], json.dumps(credentials).encode(), {'Content-Type':'application/json'})['access_token']
    headers = {'Authorization':'Bearer '+token, 'User-Agent':'WazeRemoto/1.0 (Android)', 'Content-Type':'application/json'}
    body = json.dumps({'input':destination}).encode() if action == 'send' else None
    result = request(base + ('/agent/destinations' if action == 'send' else '/agent/status'), body, headers)
    print(json.dumps(result, ensure_ascii=False))

if __name__ == '__main__':
    try:
        main()
    except urllib.error.HTTPError as e:
        raise SystemExit('Request rejected: HTTP ' + str(e.code))
    except (urllib.error.URLError, OSError, KeyError, ValueError) as e:
        raise SystemExit('Request failed: ' + type(e).__name__)
