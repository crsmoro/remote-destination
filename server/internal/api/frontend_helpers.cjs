'use strict';
const fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync(__dirname+'/static/app.js','utf8');
function setup(fetch){
 const nodes={};
 for(const id of ['connection','state','warning','converted','destination-form','destination','reference','cancel','pending-display','candidates','submit','action-status']) nodes[id]={textContent:'',value:'',children:[],handlers:{},replaceChildren(){this.children=[]},appendChild(x){this.children.push(x)},addEventListener(k,v){this.handlers[k]=v},focus(){this.focused=true},scrollIntoView(){this.scrolled=true}};
 const ctx={crypto:require('node:crypto').webcrypto,document:{getElementById:id=>nodes[id],createElement:()=>({})},setInterval(fn){ctx.tick=fn},fetch};
 vm.runInNewContext(source,ctx); return {nodes,ctx};
}
const ok=body=>({ok:true,json:async()=>body});
const flush=()=>new Promise(r=>setImmediate(r));
module.exports={setup,ok,flush};
