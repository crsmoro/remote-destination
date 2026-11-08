package com.wazeremoto;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import org.json.JSONObject;

final class DeviceApi implements CommandProcessor.Api {
    private final URI origin;
    private final String key;
    DeviceApi(URI origin, String key) { this.origin = origin; this.key = key; }
    private static final class Response {
        final int status; final String body;
        Response(int status, String body) { this.status = status; this.body = body; }
    }
    private Response post(String path, JSONObject body) throws Exception {
        HttpURLConnection c = (HttpURLConnection) origin.resolve(path).toURL().openConnection();
        c.setInstanceFollowRedirects(false);
        c.setRequestMethod("POST");
        c.setConnectTimeout(8000); c.setReadTimeout(8000);
        c.setRequestProperty("Authorization", "Bearer " + key);
        c.setRequestProperty("Content-Type", "application/json");
        c.setDoOutput(true);
        try {
            try (OutputStream out = c.getOutputStream()) { out.write(body.toString().getBytes(StandardCharsets.UTF_8)); }
            int status = c.getResponseCode();
            if (status != 200) return new Response(status, "");
            try (InputStream in = c.getInputStream(); ByteArrayOutputStream out = new ByteArrayOutputStream()) {
                byte[] buffer = new byte[1024]; int n, count = 0;
                while ((n = in.read(buffer)) != -1) { count += n; if (count > 16384) throw new IllegalStateException("response too large"); out.write(buffer, 0, n); }
                return new Response(status, new String(out.toByteArray(), StandardCharsets.UTF_8));
            }
        } finally { c.disconnect(); }
    }
    public CommandProcessor.Command poll() throws Exception {
        Response r = post("/device/poll", new JSONObject());
        if (r.status != 200) throw new IllegalStateException();
        JSONObject command = new JSONObject(r.body).optJSONObject("command");
        return command == null ? null : new CommandProcessor.Command(command.getString("id"), command.getString("uri"));
    }
    public boolean claim(String id) throws Exception {
        Response r = post("/device/claim", new JSONObject().put("id", id));
        if (r.status == 409) return false;
        if (r.status != 200) throw new IllegalStateException();
        return true;
    }
    public void ack(String id, String result) throws Exception {
        Response r = post("/device/ack", new JSONObject().put("id", id).put("result", result));
        if (r.status != 200 && r.status != 409) throw new IllegalStateException();
    }
}
