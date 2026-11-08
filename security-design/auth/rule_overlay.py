#!/usr/bin/env python3
"""Add/remove a narrowly scoped candidate rule to a JSON command array.

This only transforms stdin to stdout; it never changes Docker or production.
Do not supply command arrays containing credentials to a tracked file.
"""
import json
import os
import sys

NAME = 'remote-destination-device-v1'
ACTION = '--rule.' + NAME + '.action=allow'
HOST = os.environ['WAZE_PUBLIC_HOST']
if not HOST or any(c not in 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-' for c in HOST):
    raise ValueError('WAZE_PUBLIC_HOST must be a DNS hostname')
RULE = ('--rule.' + NAME + '.rule=Host(`' + HOST + '`) && '
        'Method(`POST`) && (Path(`/device/poll`) || Path(`/device/claim`) || Path(`/device/ack`))')

def transform(cmd, operation):
    if not isinstance(cmd, list) or not all(isinstance(v, str) for v in cmd):
        raise ValueError('expected JSON array of strings')
    prefix = '--rule.' + NAME + '.'
    existing = [x for x in cmd if x.startswith(prefix)]
    if operation == 'add':
        if existing:
            raise ValueError('candidate rule name already present; refusing to overwrite')
        return cmd + [ACTION, RULE]
    if operation == 'remove':
        if existing != [ACTION, RULE]:
            raise ValueError('candidate rule differs or is absent; refusing rollback')
        return [x for x in cmd if not x.startswith(prefix)]
    raise ValueError('operation must be add or remove')

if __name__ == '__main__':
    try:
        if len(sys.argv) != 2:
            raise ValueError('usage: rule_overlay.py add|remove < command.json')
        json.dump(transform(json.load(sys.stdin), sys.argv[1]), sys.stdout, indent=2)
        print()
    except (ValueError, json.JSONDecodeError) as exc:
        print('error:', exc, file=sys.stderr)
        sys.exit(1)
