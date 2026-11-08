package com.wazeremoto;

import android.content.Context;
import android.content.SharedPreferences;

final class LastAttemptStore implements CommandProcessor.Store {
    private final SharedPreferences prefs;
    LastAttemptStore(Context c) { prefs = c.getApplicationContext().getSharedPreferences("attempt", Context.MODE_PRIVATE); }
    public String attempted() { return prefs.getString("id", null); }
    public String result() { return prefs.getString("result", null); }
    public boolean pending() { return prefs.getBoolean("pending", false); }
    public void save(String id, String result, boolean pending) {
        if (!prefs.edit().putString("id", id).putString("result", result).putBoolean("pending", pending).commit()) throw new IllegalStateException("attempt save failed");
    }
    void clear() { if (!prefs.edit().clear().commit()) throw new IllegalStateException("attempt clear failed"); }
}
