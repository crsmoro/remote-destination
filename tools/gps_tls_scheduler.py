#!/usr/bin/env python3
"""Unprivileged periodic leaf renewal. Failures are reported, then retried."""
import os
import subprocess
import sys
import time
from pathlib import Path

import gps_tls


def cycle():
    root = Path(os.environ['GPS_PKI_DIR'])
    leaf = Path(os.environ['GPS_LEAF_DIR'])
    alert = int(os.environ.get('GPS_CA_ALERT_DAYS', '180'))
    if not 1 <= alert <= 10000:
        raise ValueError('invalid CA alert threshold')
    key_bad = False
    try:
        root_fd = gps_tls.directory(root)
        try:
            key_fd = gps_tls.open_private_key(root_fd, 'ca.key')
            os.close(key_fd)
        finally:
            os.close(root_fd)
    except (OSError, ValueError):
        key_bad = True
        print('ALERT: GPS CA private key missing or unsafe', file=sys.stderr, flush=True)
    ca = subprocess.run(['openssl', 'x509', '-in', str(root/'ca.pem'), '-noout', '-checkend', str(alert*86400)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=20)
    if ca.returncode:
        print('ALERT: GPS CA expires within configured threshold or is invalid', file=sys.stderr, flush=True)
    cmd = [sys.executable, str(Path(__file__).with_name('gps_tls.py')), 'renew', '--dir', str(root), '--output-dir', str(leaf)]
    renewed = subprocess.run(cmd, timeout=120)
    if renewed.returncode:
        print('ALERT: GPS certificate renewal failed', file=sys.stderr, flush=True)
    return bool(key_bad or ca.returncode or renewed.returncode)


def main():
    interval = int(os.environ.get('GPS_RENEW_INTERVAL_SECONDS', '86400'))
    if not 60 <= interval <= 604800:
        raise ValueError('invalid renewal interval')
    while True:
        try:
            failed = cycle()
        except (OSError, ValueError, subprocess.SubprocessError, KeyError):
            print('ALERT: GPS certificate maintenance failed', file=sys.stderr, flush=True)
            failed = True
        if '--once' in sys.argv:
            return int(failed)
        time.sleep(interval)


if __name__ == '__main__':
    sys.exit(main())
