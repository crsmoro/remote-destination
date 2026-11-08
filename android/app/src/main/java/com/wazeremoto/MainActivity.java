package com.wazeremoto;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;
import android.text.InputType;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.TextView;
import java.net.URI;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public final class MainActivity extends Activity {
    private final ExecutorService executor = Executors.newSingleThreadExecutor();
    private volatile boolean destroyed;
    private TextView status;
    private EditText base, key;
    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        LinearLayout layout = new LinearLayout(this); layout.setOrientation(LinearLayout.VERTICAL);
        int padding = (int)(16 * getResources().getDisplayMetrics().density); layout.setPadding(padding, padding, padding, padding);
        base = new EditText(this); base.setHint("Origem HTTPS (ex.: https://servidor)"); base.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_URI);
        key = new EditText(this); key.setHint("Chave do dispositivo (43 caracteres)"); key.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD); key.setSaveEnabled(false);
        status = new TextView(this); status.setText(new SecureStore(this).enabled() ? "Monitoramento habilitado. A chave salva não é exibida." : "Monitoramento parado. Configure a origem e a chave.");
        Button start = new Button(this); start.setText("Salvar e iniciar");
        Button stop = new Button(this); stop.setText("Parar monitoramento");
        Button clear = new Button(this); clear.setText("Excluir configuração e estado");
        layout.addView(base); layout.addView(key); layout.addView(start); layout.addView(stop); layout.addView(clear); layout.addView(status);
        setContentView(layout, new ViewGroup.LayoutParams(-1, -1));
        start.setOnClickListener(v -> saveAndStart());
        stop.setOnClickListener(v -> change(false));
        clear.setOnClickListener(v -> change(true));
    }
    private void message(String text) { runOnUiThread(() -> status.setText(text)); }
    private void saveAndStart() {
        final String origin = base.getText().toString().trim(), token = key.getText().toString();
        try {
            URI u = new URI(origin);
            if (!"https".equalsIgnoreCase(u.getScheme()) || u.getHost() == null || u.getUserInfo() != null || u.getQuery() != null || u.getFragment() != null || !(u.getPath().isEmpty() || u.getPath().equals("/")) || !token.matches("[A-Za-z0-9_-]{43}")) throw new IllegalArgumentException();
        } catch (Exception e) { status.setText("Informe origem HTTPS e chave base64url de 43 caracteres."); return; }
        key.setText(""); status.setText("Salvando configuração…");
        final long generation = SecureStore.GATE.invalidate();
        stopService(new Intent(this, ReceiverService.class));
        executor.execute(() -> {
            try {
                synchronized (SecureStore.LOCK) {
                    SecureStore.GATE.check(generation);
                    SecureStore store = new SecureStore(this); SecureStore.Config old = store.load();
                    LastAttemptStore attempts = new LastAttemptStore(this);
                    boolean changed = old != null && (!old.origin.equals(origin) || !old.key.equals(token));
                    if (changed && (attempts.pending() || (attempts.attempted() != null && attempts.result() == null))) throw new IllegalStateException("pending");
                    store.save(new SecureStore.Config(origin, token));
                    if (changed) attempts.clear();
                }
                runOnUiThread(() -> {
                    SecureStore.GATE.runIfCurrent(generation, () -> {
                        if (destroyed) return;
                        try { startForegroundService(new Intent(this, ReceiverService.class)); status.setText("Monitoramento iniciado. A chave salva não é exibida."); }
                        catch (RuntimeException e) { status.setText("Não foi possível iniciar o serviço; verifique restrições do Android."); }
                    });
                });
            } catch (Exception e) { runOnUiThread(() -> { if (!destroyed && SecureStore.GATE.isCurrent(generation)) status.setText("Não foi possível salvar. Se houver ACK pendente ou tentativa incerta, retome a configuração anterior ou exclua o estado explicitamente."); }); }
        });
    }
    private void change(boolean clear) {
        final long generation = SecureStore.GATE.invalidate();
        stopService(new Intent(this, ReceiverService.class)); status.setText("Parando…");
        executor.execute(() -> {
            try {
                synchronized (SecureStore.LOCK) {
                    SecureStore.GATE.check(generation);
                    SecureStore store = new SecureStore(this);
                    if (clear) { store.clear(); new LastAttemptStore(this).clear(); }
                    else store.setEnabled(false);
                }
                runOnUiThread(() -> { if (!destroyed && SecureStore.GATE.isCurrent(generation)) status.setText(clear ? "Configuração e estado excluídos." : "Monitoramento parado; configuração mantida."); });
            } catch (Exception e) { runOnUiThread(() -> { if (!destroyed && SecureStore.GATE.isCurrent(generation)) status.setText("Não foi possível atualizar a configuração."); }); }
        });
    }
    @Override protected void onDestroy() { destroyed = true; executor.shutdown(); super.onDestroy(); }
}
