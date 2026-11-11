const assert=require('node:assert/strict'),{setup,ok}=require('./frontend_helpers.cjs');
async function run(fail){
 let resolve;const calls=[];const {nodes}=setup((path,options)=>{calls.push({path,options});return path==='/api/destinations'?new Promise(r=>resolve=r):Promise.resolve(ok({state:'pending'}))});
 nodes.destination.value='Rua Original';const pending=nodes['destination-form'].handlers.submit({preventDefault(){}});
 nodes.destination.value='Novo valor';nodes.destination.handlers.input();await nodes['destination-form'].handlers.submit({preventDefault(){}});
 assert.equal(calls.filter(c=>c.path==='/api/destinations').length,1);
 resolve(fail?{ok:false,status:422}:ok({display:'Rua Original',uri:'waze://original'}));await pending;
 assert.equal(nodes.destination.value,'Novo valor');const message=nodes['action-status'].textContent+' '+nodes.warning.textContent;
 assert.match(message,fail?/422/:/registrado/i);assert.match(message,/Rua Original/);assert.equal(nodes.candidates.children.length,0);
}
(async()=>{await run(false);await run(true);console.log('edit and mutation outcomes passed')})().catch(e=>{console.error(e);process.exitCode=1});
