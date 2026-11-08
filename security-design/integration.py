#!/usr/bin/env python3
"""Isolated, disposable Portainer/Docker mTLS boundary test; never touches live networks."""
import io, json, os, pathlib, secrets, subprocess, tarfile, tempfile, time, urllib.parse, urllib.request

BASE = os.environ['PORTAINER_API_URL'].rstrip('/')
TOKEN = os.environ['PORTAINER_API_TOKEN']
ENDPOINT = os.environ.get('PORTAINER_TEST_ENDPOINT', '2')
ROOT = pathlib.Path(__file__).resolve().parent
suffix = secrets.token_hex(6)
print('test label: waze.test=' + suffix, flush=True)
containers, networks = [], []

def request(path, method='GET', obj=None, raw=None):
    data = json.dumps(obj).encode() if obj is not None else raw
    headers = {'X-API-Key': TOKEN}
    if obj is not None: headers['Content-Type'] = 'application/json'
    if raw is not None: headers['Content-Type'] = 'application/x-tar'
    req = urllib.request.Request(BASE + '/api/endpoints/' + ENDPOINT + '/docker' + path, data=data, method=method, headers=headers)
    with urllib.request.urlopen(req, timeout=30) as response:
        body = response.read()
        return json.loads(body) if body and 'application/json' in response.headers.get('Content-Type', '') else body

def run(*args):
    subprocess.run(args, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

def make_cert(dir, name, subject, ca=None, extensions=None):
    key, crt = str(dir / (name + '.key')), str(dir / (name + '.crt'))
    if ca is None:
        run('openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','1','-subj',subject,'-keyout',key,'-out',crt)
    else:
        csr = str(dir / (name + '.csr'))
        run('openssl','req','-new','-newkey','rsa:2048','-nodes','-subj',subject,'-keyout',key,'-out',csr)
        ext = dir / (name + '.ext'); ext.write_text(extensions)
        run('openssl','x509','-req','-in',csr,'-CA',str(dir / (ca + '.crt')),'-CAkey',str(dir / (ca + '.key')),'-CAcreateserial','-days','1','-extfile',str(ext),'-out',crt)

def network(name):
    n = request('/networks/create', 'POST', {'Name':'waze-test-'+suffix+'-'+name, 'Driver':'bridge','Internal':True, 'Labels':{'waze.test':suffix}})['Id']
    networks.append(n); return n

def container(name, image, net, cmd=None):
    cfg = {'Image':image,'HostConfig':{'NetworkMode':net},'NetworkingConfig':{'EndpointsConfig':{net:{'Aliases':[name]}}},'Labels':{'waze.test':suffix}}
    if cmd: cfg['Cmd']=cmd
    c=request('/containers/create?name='+urllib.parse.quote('waze-test-'+suffix+'-'+name),'POST',cfg)['Id']; containers.append(c); return c

def upload(c, files):
    data=io.BytesIO()
    with tarfile.open(fileobj=data,mode='w') as t:
        for name, content in files.items():
            if isinstance(content, str): content=content.encode()
            item=tarfile.TarInfo(name); item.size=len(content); item.mode=0o644
            t.addfile(item,io.BytesIO(content))
    request('/containers/'+c+'/archive?path=/', 'PUT', raw=data.getvalue())

def start(c): request('/containers/'+c+'/start','POST')
def wait(c): return request('/containers/'+c+'/wait','POST')['StatusCode']

try:
    info=request('/info')
    if info.get('Name') != 'jarvis': raise RuntimeError('Refusing unexpected Docker host')
    with tempfile.TemporaryDirectory(prefix='waze-test-',dir=os.environ.get('TMPDIR')) as temp:
        d=pathlib.Path(temp)
        make_cert(d,'client-ca','/CN=waze-test-client-ca')
        make_cert(d,'server-ca','/CN=waze-test-server-ca')
        make_cert(d,'server','/CN=proxy','server-ca','subjectAltName=DNS:proxy\nextendedKeyUsage=serverAuth\n')
        make_cert(d,'client','/CN=traefik','client-ca','extendedKeyUsage=clientAuth\n')
        make_cert(d,'bad','/CN=untrusted')
        shared, private=network('shared'),network('private')
        backend=container('backend','node:alpine',private,['node','-e',"require('http').createServer((q,s)=>s.end(JSON.stringify(q.headers))).listen(8080,'0.0.0.0')"]); start(backend)
        backend_ip=next(iter(request('/containers/'+backend+'/json')['NetworkSettings']['Networks'].values()))['IPAddress']
        proxy=container('proxy','nginx:latest',shared)
        request('/networks/'+private+'/connect','POST',{'Container':proxy})
        upload(proxy,{'etc/nginx/nginx.conf':(ROOT/'proxy.conf').read_bytes(), **{'etc/waze-pki/'+n:(d/n).read_bytes() for n in ['server.crt','server.key','client-ca.crt']}}); start(proxy)
        js=r'''const https=require('https'), http=require('http'), fs=require('fs'), dns=require('dns');
const read=n=>fs.readFileSync('/test/'+n);
const delay=ms=>new Promise(r=>setTimeout(r,ms));
async function req(opts){return await new Promise(resolve=>{let r=https.get({host:'proxy',port:8443,servername:opts.name||'proxy',ca:read('server-ca.crt'),headers:{'X-Forwarded-User':'spoof','X-Forwarded-For':'spoof','Forwarded':'spoof'},...(opts.cert?{cert:read(opts.cert+'.crt'),key:read(opts.cert+'.key')}:{}),timeout:3000},s=>{let b='';s.on('data',x=>b+=x);s.on('end',()=>resolve({status:s.statusCode,body:b}))});r.on('error',()=>resolve({status:0}));r.on('timeout',()=>r.destroy())})}
(async()=>{let good={};for(let i=0;i<20;i++){good=await req({cert:'client'});if(good.status===200)break;await delay(500)}
let missing=(await req({})).status,bad=(await req({cert:'bad'})).status,wrong=(await req({cert:'client',name:'wrong'})).status;
let direct=await new Promise(r=>{let q=http.get({host:read('backend-ip').toString(),port:8080,timeout:2000},()=>r(true));q.on('error',()=>r(false));q.on('timeout',()=>{q.destroy();r(false)})});
let headers=JSON.parse(good.body||'{}'), clean=!headers['x-forwarded-user']&&!headers['x-forwarded-for']&&!headers.forwarded;
console.log(JSON.stringify({good:good.status,missing,bad,wrong,direct,clean}));
process.exit(good.status===200 && missing!==200 && bad!==200 && wrong===0 && !direct && clean?0:1)})().catch(()=>process.exit(2));'''
        probe=container('probe','node:alpine',shared,['node','/test/probe.js'])
        upload(probe,{'test/probe.js':js,'test/backend-ip':backend_ip,**{'test/'+n:(d/n).read_bytes() for n in ['server-ca.crt','client.crt','client.key','bad.crt','bad.key']}}); start(probe)
        code=wait(probe)
        logs=request('/containers/'+probe+'/logs?stdout=1&stderr=1')
        print('probe exit:',code,'results:',logs[8:].decode(errors='replace').strip())
        if code: raise RuntimeError('mTLS integration test failed')
finally:
    for c in reversed(containers):
        try: request('/containers/'+c+'?force=1','DELETE')
        except Exception as e: print('CLEANUP FAILED container',c[:12],type(e).__name__)
    for n in reversed(networks):
        try: request('/networks/'+n,'DELETE')
        except Exception as e: print('CLEANUP FAILED network',n[:12],type(e).__name__)
    remaining_c=request('/containers/json?all=1&filters='+urllib.parse.quote(json.dumps({'label':['waze.test='+suffix]})))
    remaining_n=request('/networks?filters='+urllib.parse.quote(json.dumps({'label':['waze.test='+suffix]})))
    print('remaining test containers/networks:',len(remaining_c),len(remaining_n))
    if remaining_c or remaining_n: raise RuntimeError('cleanup incomplete')
