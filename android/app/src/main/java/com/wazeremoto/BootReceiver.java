package com.wazeremoto;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

public final class BootReceiver extends BroadcastReceiver {
    @Override public void onReceive(Context context, Intent intent) {
        if (intent != null && Intent.ACTION_BOOT_COMPLETED.equals(intent.getAction()) && new SecureStore(context).enabled()) {
            try { context.startForegroundService(new Intent(context, ReceiverService.class)); }
            catch (RuntimeException ignored) { /* OS/firmware may prohibit starting at boot. */ }
        }
    }
}
