#!/usr/bin/env python3
"""Dedicated HTTPS push receiver. Run as a single process with private storage."""
import fcntl
import hmac
import ipaddress
import json
import math
import os
import re
import signal
import subprocess
import sys
import stat
from pathlib import Path
import ssl
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MAX_BODY = 512
MAX_AGE = 120
MAX_FUTURE = 10


def make_tls_context():
    ca = os.environ.get('GPS_TLS_CA_FILE')
    host = os.environ.get('GPS_TLS_HOSTNAME')
    cert = os.environ['GPS_TLS_CERT']
    key = os.environ['GPS_TLS_KEY']
    if not ca or not host or cert != key:
        raise ValueError('CA, hostname and a single atomic PEM are required')
    # A sealed anonymous file prevents pathname replacement or modification
    # between verification and OpenSSL's subsequent certificate/key load.
    fd = os.memfd_create('gps-tls', os.MFD_CLOEXEC | os.MFD_ALLOW_SEALING)
    try:
        data = Path(cert).read_bytes()
        with os.fdopen(os.dup(fd), 'wb') as out:
            out.write(data)
        # Linux fcntl.h constants; some Python builds omit the symbolic names.
        fcntl.fcntl(fd, 1033, 0x0008 | 0x0004 | 0x0002 | 0x0001)
        snapshot = '/proc/self/fd/' + str(fd)
        subprocess.run(['openssl', 'verify', '-CAfile', ca, '-verify_hostname', host,
                        '-purpose', 'sslserver', snapshot], pass_fds=(fd,),
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=10)
        candidate = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        candidate.minimum_version = ssl.TLSVersion.TLSv1_2
        candidate.load_cert_chain(snapshot, snapshot)
        candidate.gps_expiry = ssl.cert_time_to_seconds(ssl._ssl._test_decode_cert(snapshot)['notAfter'])
        return candidate
    finally:
        os.close(fd)


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('duplicate key')
        result[key] = value
    return result


def _number(value):
    return type(value) in (int, float) and math.isfinite(value)


def _validate_snapshot(obj):
    if not isinstance(obj, dict) or type(obj.get('available')) is not bool:
        raise ValueError('invalid snapshot')
    expected = {'sent_at', 'available'} | ({'latitude', 'longitude'} if obj['available'] else set())
    if set(obj) != expected or not _number(obj['sent_at']) or obj['sent_at'] < 0:
        raise ValueError('invalid snapshot')
    if obj['available'] and (not _number(obj['latitude']) or not _number(obj['longitude']) or abs(obj['latitude']) > 90 or abs(obj['longitude']) > 180):
        raise ValueError('invalid snapshot')


class Receiver:
    def __init__(self, path, secret):
        self.path = Path(path)
        if len(secret) < 43 or not secret.isascii() or any(c.isspace() for c in secret):
            raise ValueError('invalid dedicated secret')
        self.secret = secret
        self.lock = threading.Lock()
        self.last = -1
        if self.path.exists():
            # Fail closed on corrupt state: deleting it must be a deliberate operator action.
            state = json.loads(self.path.read_bytes(), object_pairs_hook=_unique)
            _validate_snapshot(state)
            self.last = state['sent_at']

    def process(self, authorization, body, now=None):
        if not hmac.compare_digest(authorization.encode('utf-8'), ('Bearer ' + self.secret).encode('ascii')):
            return 401
        now = time.time() if now is None else now
        if len(body) > MAX_BODY:
            return 400
        try:
            obj = json.loads(body, object_pairs_hook=_unique, parse_constant=lambda _: (_ for _ in ()).throw(ValueError()))
            _validate_snapshot(obj)
            if not now - MAX_AGE <= obj['sent_at'] <= now + MAX_FUTURE:
                raise ValueError()
        except (ValueError, TypeError, UnicodeError, RecursionError, OverflowError):
            return 400
        with self.lock:
            if obj['sent_at'] <= self.last:
                return 409
            # Atomic replace puts sequence and public snapshot in one durable transaction.
            fd, name = tempfile.mkstemp(prefix='.position-', dir=self.path.parent)
            try:
                with os.fdopen(fd, 'w', encoding='utf-8') as f:
                    json.dump(obj, f, separators=(',', ':'), allow_nan=False)
                    f.flush()
                    os.fsync(f.fileno())
                    os.replace(name, self.path)
                    self.last = obj['sent_at']
                    os.fchmod(f.fileno(), 0o640)
                    os.fsync(f.fileno())
                self.last = obj['sent_at']
                directory = os.open(self.path.parent, os.O_RDONLY | os.O_DIRECTORY)
                try:
                    os.fsync(directory)
                finally:
                    os.close(directory)
            finally:
                if os.path.exists(name):
                    os.unlink(name)
        return 204


def serve():
    addr = os.environ['GPS_LISTEN_IP']
    ip = ipaddress.ip_address(addr)
    if ip.version != 4 or not (ip.is_loopback or ip.is_private) or ip.is_unspecified or ip.is_multicast or ip.is_link_local:
        raise ValueError('explicit private or loopback address required')
    lock_path = Path(os.environ['GPS_SNAPSHOT_FILE']).parent / '.gps-push.lock'
    lock_fd = os.open(lock_path, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600)
    fcntl.flock(lock_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
    # Dedicated directory, held exclusively: only our exact mkstemp names.
    for entry in os.scandir(lock_path.parent):
        if entry.name != Path(os.environ['GPS_SNAPSHOT_FILE']).name and re.fullmatch(r'\.position-[a-z0-9_]{8}', entry.name) and stat.S_ISREG(entry.stat(follow_symlinks=False).st_mode):
            os.unlink(entry.path)
    receiver = Receiver(os.environ['GPS_SNAPSHOT_FILE'], Path(os.environ['GPS_SECRET_FILE']).read_text().strip())

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            if self.path != '/position' or self.headers.get('Transfer-Encoding') or len(self.headers.get_all('Content-Length', [])) != 1 or len(self.headers.get_all('Authorization', [])) != 1:
                self.send_error(400)
                return
            try:
                length = int(self.headers['Content-Length'])
                if not 0 <= length <= MAX_BODY:
                    raise ValueError()
                self.connection.settimeout(5)
                body = self.rfile.read(length)
                if len(body) != length:
                    raise ValueError()
                status = receiver.process(self.headers['Authorization'], body)
            except (ValueError, OSError):
                status = 400
            self.send_response(status)
            self.send_header('Content-Length', '0')
            self.send_header('Connection', 'close')
            self.end_headers()
            self.close_connection = True

    context = make_tls_context()

    def reload_tls(_signum, _frame):
        nonlocal context
        try:
            candidate = make_tls_context()
        except (OSError, ssl.SSLError, ValueError, subprocess.SubprocessError):
            print('ALERT: GPS TLS reload rejected; retaining active context', file=sys.stderr, flush=True)
            return False
        context = candidate
        return True

    signal.signal(signal.SIGHUP, reload_tls)

    interval = int(os.environ.get('GPS_TLS_RELOAD_SECONDS', '30'))
    if not 1 <= interval <= 3600:
        raise ValueError('invalid reload interval')
    def poll_tls():
        previous = None
        while True:
            if context.gps_expiry - time.time() < 30 * 86400:
                print('ALERT: GPS active TLS certificate expires in less than 30 days', file=sys.stderr, flush=True)
            try:
                st = os.stat(os.environ['GPS_TLS_CERT'])
                current = (st.st_dev, st.st_ino, st.st_size, st.st_mtime_ns)
                if current != previous:
                    if reload_tls(None, None):
                        previous = current
            except OSError:
                print('ALERT: GPS TLS certificate unavailable; keeping active context', file=sys.stderr, flush=True)
            time.sleep(interval)
    threading.Thread(target=poll_tls, daemon=True).start()

    class Server(ThreadingHTTPServer):
        daemon_threads = True
        slots = threading.BoundedSemaphore(16)

        def process_request(self, request, client_address):
            if not self.slots.acquire(blocking=False):
                self.shutdown_request(request)
                return
            try:
                super().process_request(request, client_address)
            except BaseException:
                self.slots.release()
                raise

        def process_request_thread(self, request, client_address):
            # Absolute lifetime also bounds peers that drip-feed headers/body.
            connection = [request]
            def expire():
                try:
                    connection[0].shutdown(2)
                except OSError:
                    pass
            timer = threading.Timer(10, expire)
            timer.daemon = True
            timer.start()
            try:
                request.settimeout(5)
                with context.wrap_socket(request, server_side=True, do_handshake_on_connect=False) as secure:
                    connection[0] = secure
                    secure.do_handshake()
                    self.finish_request(secure, client_address)
            except (OSError, ssl.SSLError):
                pass
            except Exception:
                self.handle_error(request, client_address)
            finally:
                timer.cancel()
                self.shutdown_request(request)
                self.slots.release()

        def handle_error(self, request, client_address):
            pass  # never print request, credentials, location or tracebacks

    server = Server((addr, int(os.environ.get('GPS_PORT', '8443'))), Handler)
    server.serve_forever()


if __name__ == '__main__':
    serve()
