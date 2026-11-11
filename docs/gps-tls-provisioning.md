# GPS TLS isolado para containers — exemplo de staging

Não implantado. `Dockerfile.gps` e `compose.gps.example.yaml` são templates; o daemon Docker não estava disponível para validar build, volumes e rede. Não alterar produção sem teste de ponta a ponta e revisão.

**Bloqueio de segurança:** o emissor ainda não protege o caminho completo da PKI e da saída contra symlinks ou substituição concorrente de diretórios. Não execute o provisionador em caminhos ou pais controláveis por usuários não confiáveis. Uma correção completa exige operações ancoradas em descritores de diretório e testes de corrida; o isolamento e permissões reais dos volumes Docker também continuam sem validação. Não considere este template pronto para produção.

O emissor UID 10001 recebe apenas volume privado `gps-ca` (0700, proprietário 10001; CA privada 0600) e volume `gps-leaf` (diretório 0750, proprietário 10001, grupo 10002). O receptor UID 10003, grupo 10002, recebe o volume da folha **somente leitura**, CA pública separada, token e estado; nunca recebe `gps-ca` nem socket Docker. A folha `current.pem` contém certificado e chave (0640). Proteja-a como segredo: qualquer leitor pode se passar pelo servidor. Os nomes são volumes externos a provisionar separadamente por administrador; não use bind mount de arquivo individual, pois substituição atômica troca inode. O receptor não deve ter acesso ao volume privado por outros caminhos, inclusive backups, Docker API ou privilégios do host.

## Preparação

1. Crie os volumes externos indicados no compose e ajuste propriedade/permissões antes de executar como usuário não root. `gps-state` deve permitir escrita apenas ao receptor e leitura ao consumidor autorizado; `gps-secret/token` deve ser segredo forte dedicado, legível pelo receptor e inacessível ao emissor. Volume `gps-public` contém somente `ca.pem` pública. Monte `gps-leaf` inteiro, jamais somente `current.pem`.
2. Dentro de um ambiente de provisionamento isolado com os volumes privado e de folha, execute como 10001:10002:

```sh
python3 /app/tools/gps_tls.py init --dir /private --output-dir /leaf --hostname gps.internal.test --certifi /caminho/certifi-original-do-HA.pem
```

Copie `/private/ca.pem` para `gps-public/ca.pem` por procedimento administrativo seguro e somente de leitura para o receptor. Não copie `ca.key`. Preserve `/private/trust.pem`, que contém certifi original integral e a CA pública; distribua para HA por procedimento revisado, atualizando o bundle quando o certifi do HA mudar. Faça backup seguro da CA. Não recrie automaticamente uma CA após falha parcial.
3. Escolha subnet não conflitante, hostname fixo e modo de conectividade autorizado. A rede interna do exemplo não publica portas nem conecta HA automaticamente. O IP estático é apenas exemplo. Configure DNS/extra_hosts e conectividade após avaliação; se o HA não estiver na rede exemplo, ele não alcançará o receptor. Teste sem produção.

## Operação

O scheduler executa imediatamente e a cada 86400 s, configurável com `GPS_RENEW_INTERVAL_SECONDS` (60–604800), usa flock não bloqueante e timeouts. Ao se aproximar a expiração, gera nova chave e folha, valida cadeia/nome/finalidade e troca um único PEM atômico. Um processo independente para cada CA/volume; mantenha somente um emissor por folha. O receptor confere a cada 30 s (`GPS_TLS_RELOAD_SECONDS`, 1–3600), cria snapshot selado em memfd Linux do PEM e verifica CA pública, hostname, finalidade e chave correspondente antes de trocar contexto. Requer Linux e `/proc/self/fd` disponível. Conexões existentes mantêm o contexto anterior. Falha conserva contexto ativo e emite alerta em stderr; se reiniciado com PEM inválido, falha fechado até restaurar a folha. CA e hostname são obrigatórios. O antigo SIGHUP opcional não é necessário em Docker.

Colete e alerte sobre stderr com `ALERT`: falhas de renovação/reload e CA vencendo em menos de `GPS_CA_ALERT_DAYS` (padrão 180 dias). O scheduler tenta novamente no próximo intervalo; sair de `--once` com status não zero sinaliza erro/alerta. Monitore também externamente o certificado realmente apresentado, expiração da folha, disponibilidade e processo scheduler: logs sozinhos não detectam processo parado nem conectividade. Planeje rotação e distribuição da CA antes de vencer. O certificado carregado em memória não estende sua própria validade: quando vencer clientes o rejeitarão mesmo se o processo estiver de pé.

## Testes locais

`python3 -m unittest discover -s tests -p 'test_gps*.py' -v`. Eles usam PKI descartável e conexões TLS locais. Não demonstram isolamento real Docker nem garantem conectividade HA; verificar esses pontos em staging antes de implantar.
