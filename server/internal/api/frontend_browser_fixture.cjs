'use strict';
// Synthetic local backend: no real provider, M2M service, or vehicle is contacted.
const http=require('node:http'),fs=require('node:fs'),path=require('node:path');
const dir=path.join(__dirname,'static'),posts=[];
http.createServer((req,res)=>{
 if(req.url==='/api/status'){res.setHeader('Content-Type','application/json');return res.end(JSON.stringify({online:true,state:'pending',display:'Rua Teste, Cidade Exemplo',uri:'https://waze.com/ul?ll=1,2'}))}
 if(req.url==='/api/destinations'&&req.method==='POST'){
  let data='';req.on('data',x=>data+=x);return req.on('end',()=>{
   const body=JSON.parse(data);posts.push(body);res.setHeader('Content-Type','application/json');
   if(body.input==='erro'){res.statusCode=422;return res.end('{}')}
   res.end(JSON.stringify({display:'Rua Teste, Cidade Exemplo',uri:'https://waze.com/ul?ll=1,2',reference:'coordinates'}));
  });
 }
 if(req.url==='/test-posts'){res.setHeader('Content-Type','application/json');return res.end(JSON.stringify(posts))}
 const name=req.url==='/'?'index.html':req.url.slice(1);
 if(!['index.html','app.js','style.css'].includes(name)){res.statusCode=404;return res.end()}
 res.setHeader('Content-Type',name.endsWith('.js')?'text/javascript':name.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(path.join(dir,name)));
}).listen(18766,'127.0.0.1',()=>console.log('fixture ready'));
