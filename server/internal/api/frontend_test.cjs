const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const src = fs.readFileSync(__dirname + '/static/app.js', 'utf8');
async function run(status) {
 const nodes = {};
 for (const id of ['connection','state','warning','converted','destination-form','destination','cancel','pending-display']) nodes[id] = {textContent:'', addEventListener(){}};
 const context = {document:{getElementById:id=>nodes[id]}, fetch:async()=>({ok:true,json:async()=>status}),setInterval(){}};
 vm.runInNewContext(src,context); await new Promise(resolve=>setImmediate(resolve)); return nodes;
}
function deferred() {
 let resolve, reject;
 const promise = new Promise((res, rej) => { resolve = res; reject = rej; });
 return {promise, resolve, reject};
}
function controlledApp() {
 const nodes = {}, calls = [];
 for (const id of ['connection','state','warning','converted','destination-form','destination','cancel','pending-display']) {
  nodes[id] = {textContent:'', value:'', handlers:{}, addEventListener(type, fn){this.handlers[type] = fn;}};
 }
 const context = {document:{getElementById:id=>nodes[id]}, fetch(path, options) {
  const d = deferred(); calls.push({path, options, ...d}); return d.promise;
 }, setInterval(fn){context.tick = fn;}};
 vm.runInNewContext(src, context);
 return {nodes, calls, context};
}
const respond = (call, body) => call.resolve({ok:true, json:async()=>body});
const flush = () => new Promise(resolve=>setImmediate(resolve));
async function slowPollTest() {
 const {nodes, calls, context} = controlledApp();
 const newer = context.tick();
 respond(calls[0], {state:'pending', display:'slow response', online:true}); await flush();
 assert.equal(nodes['pending-display'].textContent, 'Destino pendente: slow response');
 respond(calls[1], {state:'cancelled', online:true}); await newer;
 assert.equal(nodes.state.textContent, 'Cancelado');
}
async function raceTest() {
 const {nodes, calls, context} = controlledApp();
 const newer = context.tick();
 respond(calls[1], {state:'cancelled', online:true}); await newer;
 respond(calls[0], {state:'pending', display:'obsolete', online:false}); await flush();
 assert.equal(nodes.state.textContent, 'Cancelado');
 assert.equal(nodes['pending-display'].textContent, '');
 assert.equal(nodes.connection.textContent, 'Carro online');
}
async function additionalRaces() {
 {
  const {nodes, calls, context} = controlledApp();
  const newer = context.tick();
  calls[0].reject(new Error('slow failure')); await flush();
  assert.equal(nodes.connection.textContent, 'Conexão indisponível');
  respond(calls[1], {state:'cancelled', online:true}); await newer;
  assert.equal(nodes.connection.textContent, 'Carro online');
 }
 for (const action of ['submit', 'cancel']) {
  const {nodes, calls, context} = controlledApp();
  const operation = action === 'submit' ? nodes['destination-form'].handlers.submit({preventDefault(){}}) : nodes.cancel.handlers.click();
  const during = context.tick();
  calls[1].reject(new Error('mutation failure')); await operation;
  respond(calls[2], {state:'pending', display:'obsolete', online:true}); await during;
  calls[0].reject(new Error('old failure')); await flush();
  assert.equal(nodes.warning.textContent, 'mutation failure');
  assert.equal(nodes['pending-display'].textContent, '');
  assert.equal(nodes.connection.textContent, '');
 }
 {
  const {nodes, calls, context} = controlledApp();
  const newer = context.tick();
  respond(calls[1], {state:'intent_sent', online:true, warning:'new'}); await newer;
  calls[0].reject(new Error('old failure')); await flush();
  assert.equal(nodes.connection.textContent, 'Carro online');
  assert.equal(nodes.state.textContent, 'Intent enviada (navegação não confirmada)');
  assert.equal(nodes.warning.textContent, 'new');
 }
 {
  const {nodes, calls, context} = controlledApp();
  const newer = context.tick(); calls[1].reject(new Error('new failure')); await newer;
  respond(calls[0], {state:'pending', display:'obsolete', online:true}); await flush();
  assert.equal(nodes.connection.textContent, 'Conexão indisponível');
  assert.equal(nodes['pending-display'].textContent, '');
 }
 for (const action of ['submit', 'cancel']) {
  const {nodes, calls, context} = controlledApp();
  const operation = action === 'submit' ? nodes['destination-form'].handlers.submit({preventDefault(){}}) : nodes.cancel.handlers.click();
  const during = context.tick();
  respond(calls[1], {uri:'https://waze.com/new'}); await flush();
  respond(calls[2], {state:'pending', display:'obsolete', online:false}); await during;
  assert.equal(nodes['pending-display'].textContent, '');
  respond(calls[3], action === 'submit' ? {state:'pending', display:'new', uri:'https://waze.com/new', online:true} : {state:'cancelled', online:true});
  await operation;
  respond(calls[0], {state:'pending', display:'obsolete', online:false}); await flush();
  assert.equal(nodes.state.textContent, action === 'submit' ? 'Pendente' : 'Cancelado');
  assert.equal(nodes['pending-display'].textContent, action === 'submit' ? 'Destino pendente: new\nDestino convertido: https://waze.com/new' : '');
  assert.equal(nodes.connection.textContent, 'Carro online');
 }
}
(async()=>{
 await slowPollTest();
 await raceTest();
 await additionalRaces();
 const xss='<img src=x onerror=alert(1)>';
 let nodes=await run({state:'pending',display:xss,uri:'https://waze.com/ul?ll=1,2'});
 assert.ok(nodes['pending-display'].textContent.includes(xss));
 nodes=await run({state:'pending',uri:'https://waze.com/ul?ll=1,2'});
 assert.ok(nodes['pending-display'].textContent.includes('https://waze.com/ul?ll=1,2'));
 nodes=await run({state:'cancelled',display:xss});
 assert.equal(nodes['pending-display'].textContent,'');
 console.log('frontend display tests passed');
})().catch(e=>{console.error(e);process.exit(1)});
