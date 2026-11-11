#!/usr/bin/env python3
"""Offline GPS-only CA bootstrap and serialized server certificate rotation.

Linux /proc is required: directory descriptors remain pinned across OpenSSL calls.
"""
import argparse
from contextlib import ExitStack
import fcntl
import os
from pathlib import Path
import re
import secrets
import ssl
import stat
import subprocess
import sys
import tempfile


def run(*args, timeout=20, pass_fds=()):
    subprocess.run(args, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=timeout, pass_fds=pass_fds)


def directory(path, create=False, private=False, trusted_root=False):
    """Walk without symlinks, holding each parent until its child is open."""
    path = Path(os.path.abspath(path))
    fd = os.open('/', os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        parts = path.parts[1:]
        for i, part in enumerate(parts):
            parent = os.fstat(fd)
            # Root-owned sticky directories (e.g. /tmp) prevent other users from
            # replacing an entry owned by the provisioner.
            if parent.st_uid not in (0, os.geteuid()) or (parent.st_mode & 0o022 and not (parent.st_uid == 0 and parent.st_mode & stat.S_ISVTX)):
                raise ValueError('unsafe parent directory')
            try:
                child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
            except FileNotFoundError:
                if not create:
                    raise
                try:
                    os.mkdir(part, 0o700, dir_fd=fd)
                except FileExistsError:
                    pass
                child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
            os.close(fd)
            fd = child
        info = os.fstat(fd)
        if info.st_uid not in ((0, os.geteuid()) if trusted_root else (os.geteuid(),)) or info.st_mode & 0o022:
            raise ValueError('unsafe directory ownership or permissions')
        if private:
            os.fchmod(fd, 0o700)
        return fd
    except BaseException:
        os.close(fd)
        raise


def proc(fd, name=None):
    return '/proc/self/fd/' + str(fd) + (('/' + name) if name else '')


def open_file(fd, name):
    f = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
    info = os.fstat(f)
    if not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or info.st_nlink != 1 or info.st_mode & 0o022:
        os.close(f)
        raise ValueError('unsafe PKI file')
    return f


def open_private_key(fd, name):
    key = open_file(fd, name)
    mode = os.fstat(key).st_mode
    if mode & 0o077 or mode & 0o600 != 0o600 or mode & 0o7000:
        os.close(key)
        raise ValueError('unsafe CA private key permissions')
    return key


def read_file(fd, name):
    with os.fdopen(open_file(fd, name), 'rb') as f:
        return f.read()


def exists(fd, name):
    try:
        os.stat(name, dir_fd=fd, follow_symlinks=False)
        return True
    except FileNotFoundError:
        return False


def atomic(fd, name, data, mode):
    # Exclusive creation and dir_fd replacement never follow the destination.
    temp = '.new-' + secrets.token_hex(16)
    f = os.open(temp, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, mode, dir_fd=fd)
    try:
        with os.fdopen(f, 'wb') as stream:
            os.fchmod(stream.fileno(), mode)
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        if exists(fd, name):
            check = open_file(fd, name)
            os.close(check)
        os.replace(temp, name, src_dir_fd=fd, dst_dir_fd=fd)
        os.fsync(fd)
    finally:
        try:
            os.unlink(temp, dir_fd=fd)
        except FileNotFoundError:
            pass


def hostname(value):
    if len(value) > 253 or not re.fullmatch(r'[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+', value) or any(len(x) > 63 for x in value.split('.')):
        raise ValueError('invalid hostname')
    return value


def issue(root, host, days, output=None):
    # The public helper also accepts paths for callers; main passes held FDs.
    with ExitStack() as stack:
        if not isinstance(root, int):
            root = directory(root, private=True)
            stack.callback(os.close, root)
        if output is None:
            output = root
        elif not isinstance(output, int):
            output = directory(output)
            stack.callback(os.close, output)
        ca = open_file(root, 'ca.pem')
        stack.callback(os.close, ca)
        ca_key = open_private_key(root, 'ca.key')
        stack.callback(os.close, ca_key)
        with tempfile.TemporaryDirectory(prefix='.issue-', dir=proc(root)) as tmp:
            p = Path(tmp)
            key, csr, cert, ext = (p / n for n in ('key', 'csr', 'cert', 'ext'))
            ext.write_text(f'basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:{host}\n')
            fds = (root, ca, ca_key)
            run('openssl', 'req', '-new', '-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:P-256', '-nodes', '-keyout', str(key), '-out', str(csr), '-subj', '/CN=' + host, pass_fds=fds)
            os.chmod(key, 0o600)
            run('openssl', 'x509', '-req', '-in', str(csr), '-CA', proc(ca), '-CAkey', proc(ca_key), '-set_serial', '0x' + secrets.token_hex(20), '-out', str(cert), '-days', str(days), '-sha256', '-extfile', str(ext), pass_fds=fds)
            run('openssl', 'verify', '-CAfile', proc(ca), '-verify_hostname', host, '-purpose', 'sslserver', str(cert), pass_fds=fds)
            run('openssl', 'x509', '-in', str(cert), '-noout', '-checkend', '60', pass_fds=fds)
            ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            ctx.load_cert_chain(cert, key)
            atomic(output, 'current.pem', cert.read_bytes() + key.read_bytes(), 0o600 if output == root else 0o640)


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=['init', 'renew'])
    parser.add_argument('--dir', required=True, type=Path)
    parser.add_argument('--output-dir', type=Path)
    parser.add_argument('--hostname')
    parser.add_argument('--certifi', type=Path)
    parser.add_argument('--days', type=int, default=90)
    parser.add_argument('--renew-before-days', type=int, default=30)
    parser.add_argument('--reload-unit', help='systemd unit to signal SIGHUP after rotation')
    args = parser.parse_args()
    if not 1 <= args.days <= 397 or not 0 <= args.renew_before_days < args.days:
        raise ValueError('invalid validity or renewal threshold')
    with ExitStack() as stack:
        root = directory(args.dir, create=True, private=True)
        stack.callback(os.close, root)
        output = directory(args.output_dir) if args.output_dir else root
        if args.output_dir:
            stack.callback(os.close, output)
        lock = os.open('.lock', os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW | os.O_NONBLOCK, 0o600, dir_fd=root)
        stack.callback(os.close, lock)
        info = os.fstat(lock)
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or info.st_nlink != 1 or info.st_mode & 0o022:
            raise ValueError('unsafe lock')
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.action == 'init':
            host = hostname(args.hostname or '')
            if not args.certifi or exists(output, 'current.pem') or any(exists(root, x) for x in ('ca.key', 'ca.pem', 'hostname', 'current.pem', 'trust.pem')):
                raise ValueError('init requires original certifi bundle and empty PKI directory')
            bundle_dir = directory(args.certifi.parent, trusted_root=True)
            stack.callback(os.close, bundle_dir)
            bundle_fd = os.open(args.certifi.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=bundle_dir)
            with os.fdopen(bundle_fd, 'rb') as bundle:
                info = os.fstat(bundle.fileno())
                if not stat.S_ISREG(info.st_mode) or info.st_uid not in (0, os.geteuid()) or info.st_mode & 0o022 or info.st_nlink != 1:
                    raise ValueError('unsafe CA bundle')
                original = bundle.read()
            ssl.create_default_context(cadata=original.decode('ascii'))
            with tempfile.TemporaryDirectory(prefix='.ca-', dir=proc(root)) as tmp:
                key, cert = Path(tmp) / 'key', Path(tmp) / 'cert'
                run('openssl', 'req', '-x509', '-newkey', 'ec', '-pkeyopt', 'ec_paramgen_curve:P-256', '-nodes', '-keyout', str(key), '-out', str(cert), '-days', '3650', '-sha256', '-subj', '/CN=GPS Dedicated Root CA', '-addext', 'basicConstraints=critical,CA:TRUE,pathlen:0', '-addext', 'keyUsage=critical,keyCertSign,cRLSign', pass_fds=(root,))
                atomic(root, 'ca.key', key.read_bytes(), 0o600)
                atomic(root, 'ca.pem', cert.read_bytes(), 0o644)
            atomic(root, 'hostname', (host + '\n').encode(), 0o600)
            atomic(root, 'trust.pem', original + (b'' if original.endswith(b'\n') else b'\n') + read_file(root, 'ca.pem'), 0o644)
            issue(root, host, args.days, output)
            print('GPS PKI initialized')
        else:
            host = hostname(read_file(root, 'hostname').decode().strip())
            if args.hostname and args.hostname != host:
                raise ValueError('hostname differs from initialized hostname')
            if args.reload_unit and not re.fullmatch(r'[a-zA-Z0-9_.@-]+\.service', args.reload_unit):
                raise ValueError('invalid service')
            leaf = open_file(output, 'current.pem')
            stack.callback(os.close, leaf)
            due = subprocess.run(['openssl', 'x509', '-in', proc(leaf), '-noout', '-checkend', str(args.renew_before_days * 86400)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=20, pass_fds=(leaf,)).returncode != 0
            if due:
                issue(root, host, args.days, output)
            if args.reload_unit:
                run('systemctl', 'kill', '--kill-whom=main', '--signal=HUP', args.reload_unit, timeout=10)
            print('Certificate rotated' if due else 'Certificate not due')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError, ssl.SSLError):
        print('GPS TLS operation failed; inspect configuration and service state', file=sys.stderr)
        sys.exit(1)
