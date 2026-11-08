# Pré-verificação de produção — 2026-11-08

Nenhuma implantação ou alteração da configuração de produção foi realizada.

- A pré-verificação identificou Traefik e forward-auth existentes; confirme a configuração real antes de qualquer alteração.
- O Traefik usa o provider de arquivo um arquivo de configuração, aplica forward-auth globalmente nas entradas HTTP e HTTPS e possui `--serversTransport.insecureSkipVerify=true` global. Um transporte mTLS novo deve definir explicitamente verificação de certificado e nome do servidor; não herdar o transporte global inseguro. Não mudar o padrão global, pois outros serviços podem depender dele.
- O ensaio descartável `python3 security-design/integration.py` retornou `good=200`, `missing=400`, `bad=400`, `wrong=0`, `direct=false`, `clean=true`; confirmou remoção de todos os recursos temporários. Isso testa a fixture nginx, não a cadeia Traefik/Auth0 de produção.
- A leitura sanitizada do Compose e do arquivo dinâmico de produção foi solicitada, mas ficou pendente de aprovação da ferramenta. Não é seguro editar ou recriar serviços sem verificar os arquivos reais e preservar regras e configurações existentes.
- Antes de habilitar regras, testar o caminho real de cabeçalhos encaminhados: o forward-auth usa `X-Forwarded-Host`, método e URI para suas regras. Um cabeçalho de host forjado pode criar exceção em outro serviço se a origem o transmitir sem sanitização. A correspondência isolada Host + POST + Path, por si só, não prova segurança da cadeia.
- Ainda pendentes: implementação e validação reais de mTLS, egress, regras estritas, rota, backups e rollback; testes anônimo/dispositivo e funcional logado; provisionamento seguro e teste do Android/carro. Não apresentar os testes da fixture como implantação.
