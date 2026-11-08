#!/usr/bin/env python3
"""Exercise the live image/rule command copy, never mutate live containers.

Requires PORTAINER_API_URL, PORTAINER_API_TOKEN; endpoint id defaults to 2.
Creates network=none throwaway auth containers and Python probes sharing only
those isolated namespaces; removes all created containers even on failure.
"""
import http.client
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from rule_overlay import HOST, transform

base = os.environ['PORTAINER_API_URL'].rstrip('/')
if not base.endswith('/api'):
    base += '/api'
endpoint = os.environ.get('PORTAINER_ENDPOINT_ID', '2')
base += '/endpoints/' + endpoint + '/docker'
token = os.environ['PORTAINER_API_TOKEN']
created = []

def api(method, path, data=None):
    body = None if data is None else json.dumps(data).encode()
    req = urllib.request.Request(base + path, data=body, method=method,
        headers={'X-API-Key': token, 'Content-Type': 'application/json'})
    try:
        with urllib.request.urlopen(req, timeout=30) as res:
            raw = res.read()
            return json.loads(raw) if raw else None
    except urllib.error.HTTPError as exc:
        raise RuntimeError(f'Docker API {method} {path}: {exc.code} {exc.read().decode(errors="replace")[:500]}') from None

def create(config):
    cid = api('POST', '/containers/create', config)['Id']
    created.append(cid)
    api('POST', '/containers/' + cid + '/start')
    return cid

probe = r'''
import http.client,json,sys
h=__HOST__
def req(method,path,host=h,extra=None):
 c=http.client.HTTPConnection('127.0.0.1',4181,timeout=4)
 headers={'X-Forwarded-Host':host,'X-Forwarded-Uri':path,'X-Forwarded-Method':method,'X-Forwarded-Proto':'https'}
 if extra: headers.update(extra)
 c.request('GET','/',headers=headers)
 r=c.getresponse(); status=r.status; c.close(); return status
cases=[]
for path in ['/device/poll','/device/claim','/device/ack']:
 for method in ['POST','GET','PUT','DELETE','HEAD','OPTIONS','post']:
  cases.append([method,path,h,req(method,path)])
for path in ['/device/poll/','/device/polls','/device/Poll','/device/%70oll','/device/poll%2f','/device//poll','/device/../device/poll','/device/poll?x=1','/device/claim%00','/.well-known/acme-challenge/x']:
 cases.append(['POST',path,h,req('POST',path)])
for host in ['evil.example',h+'.evil.example',h.upper(),h+':443','other.example']:
 cases.append(['POST','/device/poll',host,req('POST','/device/poll',host)])
# Typical client-controlled forwarding headers not used as the source of the
# overwritten X-Forwarded-* fields; direct access remains a separate concern.
for extra in [{'X-Original-URL':'/device/poll'}, {'X-Rewrite-URL':'/device/poll'}, {'Forwarded':'host='+h+';proto=https'}, {'X-Forwarded-Method':'POST','X-Forwarded-Uri':'/device/poll','X-Forwarded-Host':h}]:
 cases.append(['GET','/other','evil.example',req('GET','/other','evil.example',extra)])
print(json.dumps(cases))
'''.replace('__HOST__', json.dumps(HOST))

def run_probe(auth):
    p = create({'Image':'python:3.14-slim', 'Cmd':['python','-c',probe],
                'HostConfig':{'NetworkMode':'container:'+auth,'AutoRemove':False},
                'NetworkDisabled':False})
    api('POST','/containers/'+p+'/wait',{})
    info=api('GET','/containers/'+p+'/json')
    if info['State']['ExitCode'] != 0:
        raise RuntimeError('probe exited '+str(info['State']['ExitCode']))
    req=urllib.request.Request(base+'/containers/'+p+'/logs?stdout=1&stderr=1',headers={'X-API-Key':token})
    with urllib.request.urlopen(req,timeout=15) as r: raw=r.read()
    # Docker non-TTY multiplexed stream.
    out=b''
    while raw:
        n=int.from_bytes(raw[4:8],'big');out+=raw[8:8+n];raw=raw[8+n:]
    return json.loads(out)

try:
    live=api('GET','/containers/traefik-forward-auth/json')
    original=live['Config']['Cmd']
    assert transform(transform(original,'add'),'remove') == original
    print('Copied live image and',len(original),'command arguments; rollback exact.',flush=True)
    result=[]
    for label,cmd in [('baseline',original),('candidate',transform(original,'add'))]:
        auth=create({'Image':live['Image'],'Cmd':cmd,'Env':[
          'SECRET=isolated-test-secret-not-production','PROVIDERS_GOOGLE_CLIENT_ID=dummy',
          'PROVIDERS_GOOGLE_CLIENT_SECRET=dummy','LOG_LEVEL=error'],
          'HostConfig':{'NetworkMode':'none','AutoRemove':False}})
        for _ in range(30):
            state=api('GET','/containers/'+auth+'/json')['State']
            if not state['Running']:
                raise RuntimeError(label+' auth exited: '+str(state['ExitCode']))
            try:
                result.append(run_probe(auth));break
            except RuntimeError as e:
                if 'probe exited' not in str(e): raise
                time.sleep(.2)
        else: raise RuntimeError('auth did not become ready')
        print(label,'probe complete',flush=True)
    before,after=result
    for b,a in zip(before,after):
        print(json.dumps({'method':a[0],'path':a[1],'host':a[2],'before':b[3],'after':a[3]}))
    allowed={('/device/poll','POST'),('/device/claim','POST'),('/device/ack','POST')}
    for b,a in zip(before,after):
        if a[2]==HOST and (a[1],a[0]) in allowed:
            assert a[3]==200 and b[3]!=200, ('intended case failed',b,a)
        elif b[3]!=a[3]:
            raise AssertionError(('unintended behavioral change',b,a))
    print('PASS: target cases changed and all other tested cases preserved')
finally:
    for cid in reversed(created):
        try: api('DELETE','/containers/'+cid+'?force=1')
        except Exception as e: print('cleanup failure',cid[:12],str(e),file=sys.stderr)
