# Waze Remoto — Android

O serviço foreground mantém polling fora da tela do app e pede `claim` antes de enviar a intent explicitamente ao pacote `com.waze`. Ele reinicia em `BOOT_COMPLETED` se habilitado. O intervalo normal é 5 s; erros usam backoff com jitter até 60 s. A notificação indica que o serviço está ativo, não que o servidor esteja conectado nem que Waze tenha iniciado navegação. A partir do Android 10, e conforme firmware, a abertura de Activity em segundo plano pode ser bloqueada mesmo com serviço foreground. Não há fallback interativo. Android 9 no carro e comportamento real do Waze ainda precisam de confirmação.

Informe uma origem HTTPS (sem caminho) e chave base64url de 43 caracteres. Origem e chave são cifradas com AES-GCM e chave do Android Keystore; backup está desabilitado e a chave não é exibida depois. Parar desabilita reinício no boot mas preserva configuração e estado; excluir apaga ambos. Para reiniciar após parar, forneça novamente origem e chave. Troca de configuração é recusada enquanto houver ACK pendente ou tentativa de resultado incerto; apagar explicitamente o estado perde essa proteção e pode permitir nova abertura. Force-stop do Android impede o reinício automático até o usuário abrir o app novamente.

O ID da tentativa é gravado de forma síncrona antes da intent; resultado e ACK pendente são persistidos em armazenamento privado do app (ID e resultado não são cifrados). ACKs pendentes são reenviados após reinício sem reabrir, e `409` é descartado como obsoleto. Uma queda entre gravar o ID e salvar o resultado deixa uma tentativa incerta: ela não será reaberta nem falsamente confirmada; pode permanecer pendente no servidor. Nesse caso o processamento de novos comandos fica bloqueado até a exclusão explícita do estado; a interface ainda não informa esse bloqueio. Não existe exactly-once. Cancelar/substituir após o claim também pode ser tarde demais. `intent_sent` significa apenas que `startActivity` não lançou exceção, não confirmação de navegação. O serviço aceita somente links HTTPS `waze.com/ul`, conforme normalização do servidor.

## Build e testes

JDK 17 e Android SDK 36/build-tools 36. O Gradle wrapper fixa a versão 9.3.1 e verifica o SHA-256 da distribuição. Com o SDK configurado:

```
./gradlew --no-daemon :app:testDebugUnitTest :app:assembleDebug :app:assembleDebugAndroidTest
```

APK: `app/build/outputs/apk/debug/app-debug.apk`, assinado com chave debug, não é release. Testes instrumentados `ReceiverInstrumentedTest` exigem emulador descartável sem Waze; eles apagam configuração/estado do app. `RebootFixture` é auxiliar destrutivo, executá-lo só em emulador descartável. Android 9 emulador: Keystore, persistência, ACK, serviço foreground, receiver e reinício do emulador foram exercitados; não substituem teste de rede HTTPS ponta a ponta, UI, Waze, restrições de versões recentes ou testes no carro. Nunca executar esses testes no carro.
