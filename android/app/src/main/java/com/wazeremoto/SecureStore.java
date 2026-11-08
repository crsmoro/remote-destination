package com.wazeremoto;

import android.content.Context;
import android.content.SharedPreferences;
import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;
import android.util.Base64;
import java.nio.charset.StandardCharsets;
import java.security.KeyStore;
import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;
import org.json.JSONObject;

final class SecureStore {
    static final Object LOCK = new Object();
    static final GenerationGate GATE = new GenerationGate(LOCK);
    private static final String ALIAS = "waze_remoto_config_v1";
    private final SharedPreferences prefs;
    SecureStore(Context context) { prefs = context.getApplicationContext().getSharedPreferences("config", Context.MODE_PRIVATE); }
    static final class Config {
        final String origin, key;
        Config(String origin, String key) { this.origin=origin; this.key=key; }
    }
    private SecretKey key(boolean create) throws Exception {
        KeyStore ks = KeyStore.getInstance("AndroidKeyStore"); ks.load(null);
        if (!ks.containsAlias(ALIAS)) {
            if (!create) throw new IllegalStateException("encryption key unavailable");
            KeyGenerator generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore");
            generator.init(new KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT | KeyProperties.PURPOSE_DECRYPT)
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM).setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE).build());
            generator.generateKey();
        }
        return (SecretKey) ks.getKey(ALIAS, null);
    }
    Config load() throws Exception {
        String blob = prefs.getString("ciphertext", null), iv = prefs.getString("iv", null);
        if (blob == null || iv == null) return null;
        Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
        cipher.init(Cipher.DECRYPT_MODE, key(false), new GCMParameterSpec(128, Base64.decode(iv, Base64.NO_WRAP)));
        JSONObject obj = new JSONObject(new String(cipher.doFinal(Base64.decode(blob, Base64.NO_WRAP)), StandardCharsets.UTF_8));
        return new Config(obj.getString("origin"), obj.getString("key"));
    }
    boolean enabled() { return prefs.getBoolean("enabled", false); }
    void save(Config config) throws Exception {
        Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding"); cipher.init(Cipher.ENCRYPT_MODE, key(true));
        byte[] data = new JSONObject().put("origin", config.origin).put("key", config.key).toString().getBytes(StandardCharsets.UTF_8);
        String blob = Base64.encodeToString(cipher.doFinal(data), Base64.NO_WRAP);
        if (!prefs.edit().putString("ciphertext", blob).putString("iv", Base64.encodeToString(cipher.getIV(), Base64.NO_WRAP)).putBoolean("enabled", true).commit()) throw new IllegalStateException("save failed");
    }
    void setEnabled(boolean enabled) { if (!prefs.edit().putBoolean("enabled", enabled).commit()) throw new IllegalStateException("save failed"); }
    void clear() throws Exception {
        if (!prefs.edit().clear().commit()) throw new IllegalStateException("clear failed");
        KeyStore ks = KeyStore.getInstance("AndroidKeyStore"); ks.load(null); ks.deleteEntry(ALIAS);
    }
}
