package com.wazeremoto;

import android.content.Context;
import android.system.Os;
import android.system.OsConstants;
import android.system.StructStat;
import java.io.File;
import java.io.FileInputStream;
import java.net.URI;
import java.nio.charset.StandardCharsets;

/** Test APK only. Provisioning consumes a permanent per-installation marker even on failure. */
final class OneShotProvisioner {
    static final class Failure extends Exception { }
    static void run(Context context, File dir, SecureStore store) throws Failure {
        File input = new File(dir, "input");
        try {
            synchronized (SecureStore.LOCK) {
                if (!new File(dir, "consumed").createNewFile()) throw new Failure();
                StructStat stat = Os.lstat(input.getAbsolutePath());
                if (!OsConstants.S_ISREG(stat.st_mode) || stat.st_uid != android.os.Process.myUid()
                        || (stat.st_mode & 0077) != 0 || stat.st_size < 1 || stat.st_size > 1024) throw new Failure();
                byte[] bytes = new byte[(int) stat.st_size];
                try (FileInputStream stream = new FileInputStream(input)) {
                    int offset = 0, n;
                    while (offset < bytes.length && (n = stream.read(bytes, offset, bytes.length - offset)) > 0) offset += n;
                    if (offset != bytes.length || stream.read() != -1) throw new Failure();
                }
                String[] lines = new String(bytes, StandardCharsets.UTF_8).split("\\n", -1);
                java.util.Arrays.fill(bytes, (byte) 0);
                if (lines.length != 3 || !lines[2].isEmpty()) throw new Failure();
                URI u = new URI(lines[0]);
                if (!"https".equalsIgnoreCase(u.getScheme()) || u.getHost() == null || u.getUserInfo() != null
                        || u.getQuery() != null || u.getFragment() != null
                        || !(u.getPath().isEmpty() || u.getPath().equals("/"))
                        || !lines[1].matches("[A-Za-z0-9_-]{43}")) throw new Failure();
                if (store.load() != null) throw new Failure();
                store.save(new SecureStore.Config(lines[0], lines[1]));
            }
        } catch (Exception e) { throw new Failure(); }
        finally {
            try { java.nio.file.Files.deleteIfExists(input.toPath()); }
            catch (Exception ignored) { throw new Failure(); }
        }
    }
}
