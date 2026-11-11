'use strict';
const assert=require('node:assert/strict');
const {setup,ok,flush}=require('./frontend_helpers.cjs');
(async()=>{
 for(const status of [{id:'a',state:'cancelled'},{id:'b',state:'pending'},{id:'a',state:'intent_sent'},'fail','slow','legacy']) {
  let calls=0, release;
  const slow=new Promise(r=>release=r);
  const {nodes}=setup(async(path)=>{
   if(path==='/api/destinations') { if(++calls===1) throw Error('lost'); return ok({id:'a',state:'pending',...(status==='legacy'?{}:{replayed:true}),uri:'https://waze.com/ul?ll=1,2',display:'<img onerror=alert(1)>'}); }
   if(calls===0) return ok({});
   if(status==='fail') throw Error('offline');
   if(status==='slow') return slow;
   return ok(status);
  });
  nodes.destination.value='destination'; await nodes['destination-form'].handlers.submit({preventDefault(){}});
  await nodes['destination-form'].handlers.submit({preventDefault(){}}); await flush();
  assert.match(nodes['action-status'].textContent,/já registrada/i);
  assert.doesNotMatch(nodes['action-status'].textContent,/registrado para envio/i);
  if(status==='slow') release(ok({id:'a',state:'cancelled'}));
 }
 console.log('historical replay UI passed');
})().catch(e=>{console.error(e);process.exitCode=1});
