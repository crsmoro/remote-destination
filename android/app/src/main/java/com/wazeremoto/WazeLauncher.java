package com.wazeremoto;

import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import java.net.URI;

final class WazeLauncher implements CommandProcessor.Launcher {
    private final Context context;
    WazeLauncher(Context context) { this.context=context; }
    public void launch(String value) throws Exception {
        URI u = new URI(value);
        if (!"https".equalsIgnoreCase(u.getScheme()) || !"waze.com".equalsIgnoreCase(u.getHost()) || !"/ul".equals(u.getPath()) || u.getUserInfo() != null || u.getPort() != -1 || u.getFragment() != null) throw new IllegalArgumentException("invalid Waze URL");
        Intent intent = new Intent(Intent.ACTION_VIEW, Uri.parse(value)).setPackage("com.waze").addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        context.startActivity(intent);
    }
}
