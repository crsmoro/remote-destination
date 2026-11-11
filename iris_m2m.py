#!/usr/bin/env python3
"""Iris M2M client. Destination input is read from stdin, never arguments."""
import json
import fcntl
import hashlib
import hmac
import secrets
import tempfile
import os
import sys
import urllib.error
import urllib.request
import uuid
from pathlib import Path

CONFIG = Path(os.environ.get('WAZE_IRIS_CONFIG', Path.home() / '.config/waze-remoto/iris-m2m.json'))

def request(url, body=None, headers=None):
    req = urllib.request.Request(url, data=body, headers=headers or {}, method='POST' if body is not None else 'GET')
    with urllib.request.urlopen(req, timeout=20) as response:
        return json.load(response)

def durable_send(url, body, headers, deliver=None):
    # Serialize across processes, including the network request and recovery.
    directory = CONFIG.parent
    directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    state = directory / (CONFIG.name + '.pending')
    lock_path = directory / (CONFIG.name + '.lock')
    fd = os.open(lock_path, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    try:
        os.fchmod(fd, 0o600)
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            waited = False
        except BlockingIOError:
            waited = True
            fcntl.flock(fd, fcntl.LOCK_EX)
        try:
            sfd = os.open(state, os.O_RDONLY | os.O_NOFOLLOW)
        except FileNotFoundError:
            sfd = None
        saved = None
        if sfd is not None:
            with os.fdopen(sfd, 'r') as f:
                saved = json.load(f)
            if saved.get('acknowledged') and not waited:
                saved = None
        def save_state(value):
            tmpfd, tmp = tempfile.mkstemp(prefix='.iris-pending-', dir=directory)
            try:
                with os.fdopen(tmpfd, 'w') as f:
                    json.dump(value, f)
                    f.flush()
                    os.fsync(f.fileno())
                os.replace(tmp, state)
                dfd = os.open(directory, os.O_RDONLY | os.O_DIRECTORY)
                try: os.fsync(dfd)
                finally: os.close(dfd)
            finally:
                if os.path.exists(tmp): os.unlink(tmp)
        if saved is not None:
            digest = hmac.new(bytes.fromhex(saved['salt']), url.encode() + b'\0' + body, hashlib.sha256).hexdigest()
            if not hmac.compare_digest(digest, saved['digest']):
                raise ValueError('another send has an unresolved outcome')
            if saved.pop('acknowledged', False):
                save_state(saved)
        else:
            salt = secrets.token_bytes(32)
            saved = {'key': str(uuid.uuid4()), 'salt': salt.hex(),
                     'digest': hmac.new(salt, url.encode() + b'\0' + body, hashlib.sha256).hexdigest()}
            save_state(saved)
        headers['Idempotency-Key'] = saved['key']
        try:
            result = request(url, body, headers)
        except urllib.error.HTTPError as e:
            # Validation is rejected before any command can commit.
            if e.code == 422:
                os.unlink(state)
                dfd = os.open(directory, os.O_RDONLY | os.O_DIRECTORY)
                try: os.fsync(dfd)
                finally: os.close(dfd)
            raise
        if deliver is not None:
            deliver(result)
            saved['acknowledged'] = True
            save_state(saved)
        return result
    finally:
        os.close(fd)

def main():
    if len(sys.argv) != 2 or sys.argv[1] not in ('status', 'send', 'search', 'confirm', 'reset-send'):
        raise SystemExit('usage: iris_m2m.py status|send|search|confirm|reset-send (input on stdin)')
    action = sys.argv[1]
    if action == 'reset-send':
        directory = CONFIG.parent
        directory.mkdir(mode=0o700, parents=True, exist_ok=True)
        fd = os.open(directory / (CONFIG.name + '.lock'), os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
        try:
            os.fchmod(fd, 0o600)
            fcntl.flock(fd, fcntl.LOCK_EX)
            try: os.unlink(directory / (CONFIG.name + '.pending'))
            except FileNotFoundError: pass
            dfd = os.open(directory, os.O_RDONLY | os.O_DIRECTORY)
            try: os.fsync(dfd)
            finally: os.close(dfd)
        finally:
            os.close(fd)
        print('Send recovery reset; next send creates a new operation.')
        return
    destination = sys.stdin.read().strip() if action != 'status' else None
    if action != 'status' and not destination:
        raise SystemExit('input required on stdin')
    config = json.loads(CONFIG.read_text())
    base = config['base'].rstrip('/')
    credentials = {'grant_type': 'client_credentials', 'client_id': config['client_id'],
                   'client_secret': Path(config['secret_file']).read_text().strip(), 'audience': base + '/hermes-api',
                   'scope': 'destination:write status:read'}
    token = request(config['token_url'], json.dumps(credentials).encode(), {'Content-Type':'application/json'})['access_token']
    headers = {'Authorization':'Bearer '+token, 'User-Agent':'WazeRemoto/1.0 (Android)', 'Content-Type':'application/json'}
    paths = {'status':'/agent/status', 'send':'/agent/destinations', 'search':'/agent/geocode', 'confirm':'/agent/geocode/confirm'}
    payload = json.loads(destination) if action == 'confirm' else {'input':destination}
    body = None if action == 'status' else json.dumps(payload).encode()
    def deliver(result):
        if action == 'send' and isinstance(result, dict) and 'state' in result:
            result['state_at_creation'] = result.pop('state')
            result.setdefault('replayed', None)  # Older servers cannot identify a replay.
            result['state_notice'] = ('Idempotent replay of a historical response. ' if result.get('replayed') is True else '') + 'Historical creation state; current state may have changed. Check status and compare the command ID.'
        print(json.dumps(result, ensure_ascii=False), flush=True)
    if action == 'send':
        durable_send(base + paths[action], body, headers, deliver=deliver)
    else:
        deliver(request(base + paths[action], body, headers))

if __name__ == '__main__':
    try:
        main()
    except urllib.error.HTTPError as e:
        code = e.code
        e.close()
        raise SystemExit('Request rejected: HTTP ' + str(code))
    except (urllib.error.URLError, OSError, KeyError, ValueError) as e:
        raise SystemExit('Request failed: ' + type(e).__name__)
