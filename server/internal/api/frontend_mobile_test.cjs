const assert=require('node:assert/strict'),fs=require('node:fs'),{setup,ok}=require('./frontend_helpers.cjs');
const html=fs.readFileSync(__dirname+'/static/index.html','utf8');
(async()=>{
 assert.match(html,/Enviar destino/);assert.match(html,/id="action-status"[^>]*role="status"/);assert.doesNotMatch(html,/id="city"/);
 let resolve;const calls=[];const {nodes}=setup((path,opts)=>{calls.push({path,opts});return path==='/api/destinations'?new Promise(r=>resolve=r):Promise.resolve(ok({state:'pending'}))});
 nodes.destination.value='Rua A';const pending=nodes['destination-form'].handlers.submit({preventDefault(){}});assert.match(nodes['action-status'].textContent,/buscando/i);assert.equal(calls.filter(c=>c.path==='/api/destinations').length,1);assert(!calls.some(c=>c.path.startsWith('/api/geocode')));
 resolve(ok({display:'Rua A',uri:'waze://x'}));await pending;assert.match(nodes['action-status'].textContent,/Rua A/);assert.ok(nodes['action-status'].focused&&nodes['action-status'].scrolled);
 nodes.destination.value='Rua B';const p=nodes['destination-form'].handlers.submit({preventDefault(){}});resolve({ok:false,status:422});await p;assert.match(nodes.warning.textContent,/422/);assert.ok(nodes.warning.focused&&nodes.warning.scrolled);
 nodes.destination.value='Rua B';const retry=nodes['destination-form'].handlers.submit({preventDefault(){}});resolve(ok({display:'Rua B',uri:'waze://x'}));await retry;assert.match(nodes['action-status'].textContent,/Rua B/);assert.equal(nodes.warning.textContent,'');
 console.log('mobile feedback and retry passed');
})().catch(e=>{console.error(e);process.exitCode=1});
