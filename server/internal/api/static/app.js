'use strict';
const el = id => document.getElementById(id);
const names = {pending:'Pendente', failed:'Falha ao enviar intent', intent_sent:'Intent enviada (navegação não confirmada)', cancelled:'Cancelado'};
async function request(path, options) { const r = await fetch(path, options); if (!r.ok) throw new Error('Falha na solicitação (' + r.status + ')'); return r.json(); }
async function refresh() { try { const s = await request('/api/status'); el('connection').textContent = s.online ? 'Carro online' : 'Carro offline'; el('state').textContent = s.state ? (names[s.state] || s.state) : 'Nenhum destino'; el('warning').textContent = s.warning || ''; } catch(e) { el('connection').textContent = 'Conexão indisponível'; } }
el('destination-form').addEventListener('submit', async e => { e.preventDefault(); try { const v = await request('/api/destinations', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({input:el('destination').value})}); el('converted').textContent = 'Destino convertido: ' + v.uri; await refresh(); } catch(e) { el('warning').textContent = e.message; } });
el('cancel').addEventListener('click', async () => { try { await request('/api/destinations/pending', {method:'DELETE'}); el('converted').textContent = ''; await refresh(); } catch(e) { el('warning').textContent = e.message; } });
refresh(); setInterval(refresh,5000);
