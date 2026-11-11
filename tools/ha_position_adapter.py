"""Read one cached HA entity; write only a short-lived local position snapshot.

Run as a separate unprivileged process, never inside the backend. No vehicle calls.
"""
import json
import math
import os
import re
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request


def snapshot(data):
    if not isinstance(data, dict) or data.get('state') in (None, 'unknown', 'unavailable'):
        return {'available': False}
    attrs = data.get('attributes')
    if not isinstance(attrs, dict):
        return {'available': False}
    lat, lon = attrs.get('latitude'), attrs.get('longitude')
    if any(isinstance(n, bool) or not isinstance(n, (int, float)) or not math.isfinite(n) for n in (lat, lon)) or abs(lat) > 90 or abs(lon) > 180:
        return {'available': False}
    return {'available': True, 'latitude': lat, 'longitude': lon}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def main():
    base = os.environ['HA_URL'].rstrip('/')
    parsed = urllib.parse.urlsplit(base)
    entity = os.environ['HA_TRACKER_ENTITY']
    if parsed.scheme != 'https' or not parsed.hostname or parsed.username or parsed.password or parsed.path or parsed.query or parsed.fragment or not re.fullmatch(r'device_tracker\.[a-z0-9_]+', entity):
        raise ValueError('invalid adapter configuration')
    target = os.environ['POSITION_OUTPUT_FILE']
    if not os.path.isabs(target):
        raise ValueError('invalid adapter configuration')
    with open(os.environ['HA_TOKEN_FILE'], encoding='utf-8') as f:
        token = f.read(16385).strip()
    if not token or len(token) > 16384 or '\n' in token or '\r' in token:
        raise ValueError('invalid adapter configuration')
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    while True:
        value = {'available': False}
        try:
            req = urllib.request.Request(base + '/api/states/' + entity, headers={'Authorization': 'Bearer ' + token, 'Accept': 'application/json'})
            with opener.open(req, timeout=5) as response:
                raw = response.read(65537)
            if len(raw) <= 65536:
                value = snapshot(json.loads(raw))
        except (OSError, ValueError, urllib.error.URLError):
            pass
        # Atomic replacement; backend must mount the directory, not a single inode.
        fd, tmp = tempfile.mkstemp(prefix='.position-', dir=os.path.dirname(target))
        try:
            os.fchmod(fd, 0o640)
            with os.fdopen(fd, 'w', encoding='utf-8') as f:
                json.dump(value, f, separators=(',', ':'))
            os.replace(tmp, target)
        finally:
            if os.path.exists(tmp):
                os.unlink(tmp)
        time.sleep(60)


if __name__ == '__main__':
    main()
