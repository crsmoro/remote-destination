'use strict';
const assert=require('node:assert/strict');
const {setup,ok}=require('./frontend_helpers.cjs');
(async()=>{
 const keys=[]; let fail=true;
 const {nodes}=setup(async(path,opts)=>{if(path==='/api/destinations'){keys.push(opts.headers['Idempotency-Key']); if(fail){fail=false; throw Error('network')} return ok({uri:'waze://x'})} return ok({})});
 nodes.destination.value='one'; await nodes['destination-form'].handlers.submit({preventDefault(){}}); await nodes['destination-form'].handlers.submit({preventDefault(){}});
 nodes.destination.value='two'; await nodes['destination-form'].handlers.submit({preventDefault(){}});
 assert.ok(keys[0]); assert.equal(keys[0],keys[1]); assert.notEqual(keys[1],keys[2]); console.log('idempotency UI passed');
})().catch(e=>{console.error(e);process.exitCode=1});
