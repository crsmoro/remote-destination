'use strict';
const el = id => document.getElementById(id);
const names = {pending:'Pendente', failed:'Falha ao enviar intent', intent_sent:'Intent enviada (navegação não confirmada)', cancelled:'Cancelado'};
async function request(path, options) {
 let r;
 try { r = await fetch(path, options); } catch (_) { throw new Error('Conexão indisponível'); }
 if (!r.ok) throw new Error(r.status === 422 ? 'Destino ou seleção inválida (422). Confira os dados.' : 'Falha na solicitação (' + (Number.isInteger(r.status) ? r.status : 'erro') + ')');
 try { return await r.json(); } catch (_) { throw new Error('Resposta inválida do servidor'); }
}
let statusGeneration = 0;
let appliedGeneration = 0;
let mutationGeneration = 0;
let writeInFlight = false;
let retryAttempt = null;
let statusWarning = '', requestWarning = '';
function renderWarnings() { el('warning').textContent = [statusWarning, requestWarning].filter(Boolean).join('\n'); }
function clearRequestWarning() { requestWarning = ''; renderWarnings(); }
function reveal(node) { node?.scrollIntoView?.({block:'center'}); node?.focus?.({preventScroll:true}); }
function actionStatus(message, visible = false) { const node = el('action-status'); if (node) { node.textContent = message; if (visible) reveal(node); } }
function showRequestWarning(message) { requestWarning = message; renderWarnings(); reveal(el('warning')); }
function directWaze(input) { return /^https:\/\/(?:www\.|ul\.)?waze\.com\/ul(?:[?#]|$)/i.test(input.trim()) && !/[?&](?:q|%71)=/i.test(input); }
function updateSubmit() { const button = el('submit'); if (button) button.textContent = 'Enviar destino'; }
function clearSelection() { el('candidates').replaceChildren(); el('reference').textContent = ''; }
for (const id of ['destination']) el(id).addEventListener('input', () => { ++mutationGeneration; retryAttempt = null; clearSelection(); if (!writeInFlight) { clearRequestWarning(); actionStatus(''); } updateSubmit(); });
async function refresh() {
 const generation = ++statusGeneration;
 try {
  const s = await request('/api/status');
  if (generation <= appliedGeneration) return;
  appliedGeneration = generation;
  el('connection').textContent = s.online ? 'Carro online' : 'Carro offline';
  el('state').textContent = s.state ? (names[s.state] || s.state) : 'Nenhum destino';
  statusWarning = s.warning || ''; renderWarnings();
  el('pending-display').textContent = s.state === 'pending' ? 'Destino pendente: ' + (s.display || s.uri || 'indisponível') + (s.display && s.uri ? '\nDestino convertido: ' + s.uri : '') : '';
  if (s.state !== 'pending') el('converted').textContent = '';
 } catch(e) { if (generation > appliedGeneration) { appliedGeneration = generation; el('connection').textContent = 'Conexão indisponível'; } }
}
el('destination-form').addEventListener('submit', async e => {
 e.preventDefault();
 if (writeInFlight) return;
 const mutation = ++mutationGeneration; appliedGeneration = ++statusGeneration; clearSelection(); clearRequestWarning();
 try {
  const input = el('destination').value;
  if (!retryAttempt || retryAttempt.input !== input) retryAttempt = {input, key: crypto.randomUUID()};
  const key = retryAttempt.key, retried = !!retryAttempt.attempted; retryAttempt.attempted = true;
  actionStatus('Buscando e enviando destino…');
  writeInFlight = true;
  try {
   const v = await request('/api/destinations', {method:'POST', headers:{'Content-Type':'application/json', 'Idempotency-Key':key}, body:JSON.stringify({input})});
   appliedGeneration = ++statusGeneration; retryAttempt = null;
   const historical = v.replayed === true || (retried && v.replayed !== false);
   if (!historical && typeof v.warning === 'string' && v.warning) showRequestWarning(v.warning);
   if (mutation === mutationGeneration) el('destination').value = ''; updateSubmit(); el('converted').textContent = historical ? '' : 'Destino convertido: ' + v.uri; el('reference').textContent = historical ? '' : (({gps_unverified_age:'Referência: GPS do carro; a posição pode estar desatualizada.',joinville:'Referência: Joinville/SC (GPS indisponível).',possible_text_city:'Cidade identificada no texto.',coordinates:'Coordenadas do link.'})[v.reference] || ''); actionStatus(historical ? 'Solicitação já registrada: ' + input + '. Verifique o estado atual abaixo; ele pode ser de outro destino.' : 'Destino registrado para envio ao carro: ' + (v.display || input) + '. Consulte o estado abaixo.', true); void refresh();
  } catch(e) { appliedGeneration = ++statusGeneration; actionStatus(''); showRequestWarning(input ? 'Envio de ' + input + ': ' + e.message : e.message); }
  finally { writeInFlight = false; }
 } catch(e) { if (mutation === mutationGeneration) { appliedGeneration = ++statusGeneration; actionStatus(''); showRequestWarning(e.message); } }
});
el('cancel').addEventListener('click', async () => {
 if (writeInFlight) return;
 writeInFlight = true; retryAttempt = null;
 const mutation = ++mutationGeneration; appliedGeneration = ++statusGeneration; clearSelection(); clearRequestWarning(); actionStatus('');
 try {
  await request('/api/destinations/pending', {method:'DELETE'});
  appliedGeneration = ++statusGeneration;
  el('converted').textContent = ''; actionStatus('Destino pendente cancelado.', true); void refresh();
 } catch(e) { appliedGeneration = ++statusGeneration; actionStatus(''); showRequestWarning(e.message); }
 finally { writeInFlight = false; }
});
refresh(); setInterval(refresh,5000);
