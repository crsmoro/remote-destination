# pyright: reportUndefinedVariable=false
# Run inside Hermes browser_exec: exec(open('/opt/data/waze-remoto/server/internal/api/frontend_mobile_browser.py').read())
# Browser helpers are injected by browser_exec.
# Uses a synthetic local HTTP server only; no geocoder, M2M, or vehicle is contacted.
import subprocess, time, urllib.request, json
from pathlib import Path
fixture = Path('/opt/data/waze-remoto/server/internal/api/frontend_browser_fixture.cjs')
server = subprocess.Popen(['node', str(fixture)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
try:
    for attempt in range(50):
        try:
            urllib.request.urlopen('http://127.0.0.1:18766/', timeout=1).close()
            break
        except OSError:
            time.sleep(.1)
    else:
        raise AssertionError('local fixture did not start')
    assert server.poll() is None, 'fixture process exited'
    new_tab('http://127.0.0.1:18766/')
    wait_for_load()
    cdp('Emulation.setDeviceMetricsOverride', width=390, height=700, deviceScaleFactor=1, mobile=True)
    js("window.testErrors=[]; window.addEventListener('error', e=>testErrors.push(e.message)); window.addEventListener('unhandledrejection', e=>testErrors.push(String(e.reason))); console.error=(...x)=>testErrors.push(x.join(' '));")
    assert js('[innerWidth,innerHeight]') == [390, 700]
    assert js("document.querySelectorAll('#city').length") == 0
    def submit(value):
        fill_input('#destination', value)
        js("document.querySelector('#submit').scrollIntoView({block:'center'})")
        rect = js("(()=>{const r=document.querySelector('#submit').getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()")
        click_at_xy(rect['x'], rect['y'])
        time.sleep(.35)
    def visible(selector):
        return js("(()=>{const r=document.querySelector(" + json.dumps(selector) + ").getBoundingClientRect();return r.height>0&&r.top>=0&&r.bottom<=innerHeight})()")
    submit('Rua Teste, Cidade Exemplo')
    assert 'Rua Teste, Cidade Exemplo' in js("document.querySelector('#action-status').textContent")
    assert js("document.querySelector('#state').textContent") == 'Pendente'
    assert 'Rua Teste' in js("document.querySelector('#pending-display').textContent")
    assert visible('#action-status')
    assert js("fetch('/test-posts').then(r=>r.json())") == [{'input':'Rua Teste, Cidade Exemplo'}]
    maps = 'https://www.google.com/maps/search/?api=1&query=1%2C2'
    submit(maps)
    assert 'Coordenadas' in js("document.querySelector('#reference').textContent")
    submit('erro')
    assert '422' in js("document.querySelector('#warning').textContent")
    assert js('document.activeElement.id') == 'warning' and visible('#warning')
    assert js("fetch('/test-posts').then(r=>r.json())") == [{'input':'Rua Teste, Cidade Exemplo'}, {'input':maps}, {'input':'erro'}]
    assert js('window.testErrors') == []
    print('PASS real browser 390x700: one POST, address/status, Maps, 422 visible; console errors=0')
finally:
    server.terminate()
    server.wait(timeout=5)
