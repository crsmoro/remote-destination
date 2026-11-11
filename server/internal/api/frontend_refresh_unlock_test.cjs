const assert = require('node:assert/strict');
const fs = require('node:fs'), vm = require('node:vm');
const src = fs.readFileSync(__dirname + '/static/app.js', 'utf8');
function node() { return {textContent:'', value:'', children:[], handlers:{}, appendChild(x){this.children.push(x)}, replaceChildren(){this.children=[]}, addEventListener(k,v){this.handlers[k]=v}, focus(){}, scrollIntoView(){}}; }
const flush = () => new Promise(r => setImmediate(r));
function setup() {
 const nodes = {}, calls = [];
 for (const id of ['connection','state','warning','converted','destination-form','destination','city','reference','cancel','pending-display','candidates','action-status','submit']) nodes[id]=node();
 const context = {crypto:require('node:crypto').webcrypto,document:{getElementById:id=>nodes[id],createElement:node}, setInterval(fn){context.tick=fn}, fetch(path,options){let resolve,reject; const promise=new Promise((a,b)=>{resolve=a;reject=b}); calls.push({path,options,resolve,reject}); return promise;}};
 vm.runInNewContext(src,context);
 return {nodes,calls,context};
}
const respond = (c, body) => c.resolve({ok:true,json:async()=>body});
async function test(first, second) {
 const {nodes,calls,context}=setup();
 respond(calls[0],{online:true}); await flush();
 let operation;
 if(first==='send') {
  nodes.destination.value='https://waze.com/ul?ll=1,2'; operation=nodes['destination-form'].handlers.submit({preventDefault(){}});
 } else operation=nodes.cancel.handlers.click();
 const write=calls.at(-1); assert.notEqual(write.path,'/api/status');
 // Concurrent mutation is still prohibited until the actual write completes.
 await nodes.cancel.handlers.click(); assert.equal(calls.at(-1),write);
 respond(write,{uri:'https://waze.com/ul?ll=1,2'}); await flush();
 assert.match(nodes['action-status'].textContent, first==='cancel'?/cancelado/:/registrado/);
 const stale=calls.at(-1); assert.equal(stale.path,'/api/status');
 let next;
 if(second==='cancel') next=nodes.cancel.handlers.click();
 else {nodes.destination.value='https://waze.com/ul?ll=3,4'; next=nodes['destination-form'].handlers.submit({preventDefault(){}});}
 const newer=calls.at(-1);
 assert.notEqual(newer,stale,`${first} success must unlock ${second} while refresh is pending`);
 assert.equal(newer.path,second==='cancel'?'/api/destinations/pending':'/api/destinations');
 respond(newer,{uri:'https://waze.com/ul?ll=3,4'}); await flush();
 const latest=calls.at(-1); assert.equal(latest.path,'/api/status');
 respond(latest,{online:true,state:second==='cancel'?'cancelled':'pending',display:'Novo'}); await flush();
 respond(stale,{online:false,state:'pending',display:'Obsoleto'}); await flush();
 assert.equal(nodes.state.textContent,second==='cancel'?'Cancelado':'Pendente');
 assert.doesNotMatch(nodes['pending-display'].textContent,/Obsoleto/);
 assert.match(nodes['action-status'].textContent,second==='cancel'?/cancelado/:/ll=3,4/);
 await operation; await next;
}
(async()=>{for(const first of ['send','cancel']) for(const second of ['send','cancel']) await test(first,second); console.log('pending refresh does not block writes');})().catch(e=>{console.error(e);process.exitCode=1});
