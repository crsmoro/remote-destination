'use strict';
const el = id => document.getElementById(id);
const names = {pending:'Pendente', failed:'Falha ao enviar intent', intent_sent:'Intent enviada (navegação não confirmada)', cancelled:'Cancelado'};
async function request(path, options) { const r = await fetch(path, options); if (!r.ok) throw new Error('Falha na solicitação (' + r.status + ')'); return r.json(); }
let statusGeneration = 0;
let appliedGeneration = 0;
let mutationGeneration = 0;
async function refresh() {
 const generation = ++statusGeneration;
 try {
  const s = await request('/api/status');
  if (generation <= appliedGeneration) return;
  appliedGeneration = generation;
  el('connection').textContent = s.online ? 'Carro online' : 'Carro offline';
  el('state').textContent = s.state ? (names[s.state] || s.state) : 'Nenhum destino';
  el('warning').textContent = s.warning || '';
  el('pending-display').textContent = s.state === 'pending' ? 'Destino pendente: ' + (s.display || s.uri || 'indisponível') + (s.display && s.uri ? '\nDestino convertido: ' + s.uri : '') : '';
  if (s.state !== 'pending') el('converted').textContent = '';
 } catch(e) { if (generation > appliedGeneration) { appliedGeneration = generation; el('connection').textContent = 'Conexão indisponível'; } }
}
el('destination-form').addEventListener('submit', async e => {
 e.preventDefault();
 const mutation = ++mutationGeneration; appliedGeneration = ++statusGeneration;
 try {
  const v = await request('/api/destinations', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({input:el('destination').value})});
  if (mutation !== mutationGeneration) return;
  appliedGeneration = ++statusGeneration;
  el('destination').value = ''; el('converted').textContent = 'Destino convertido: ' + v.uri; await refresh();
 } catch(e) { if (mutation === mutationGeneration) { appliedGeneration = ++statusGeneration; el('warning').textContent = e.message; } }
});
el('cancel').addEventListener('click', async () => {
 const mutation = ++mutationGeneration; appliedGeneration = ++statusGeneration;
 try {
  await request('/api/destinations/pending', {method:'DELETE'});
  if (mutation !== mutationGeneration) return;
  appliedGeneration = ++statusGeneration;
  el('converted').textContent = ''; await refresh();
 } catch(e) { if (mutation === mutationGeneration) { appliedGeneration = ++statusGeneration; el('warning').textContent = e.message; } }
});
refresh(); setInterval(refresh,5000);
