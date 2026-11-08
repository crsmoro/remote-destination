package com.wazeremoto;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Intent;
import android.os.IBinder;
import java.net.URI;
import java.util.concurrent.ThreadLocalRandom;

public final class ReceiverService extends Service {
    private volatile boolean running;
    private Thread worker;
    private long generation;
    @Override public void onCreate() {
        super.onCreate();
        NotificationManager manager = getSystemService(NotificationManager.class);
        manager.createNotificationChannel(new NotificationChannel("receiver", "Monitoramento de destino", NotificationManager.IMPORTANCE_LOW));
        PendingIntent content = PendingIntent.getActivity(this, 0, new Intent(this, MainActivity.class), PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        Notification notification = new Notification.Builder(this, "receiver").setSmallIcon(android.R.drawable.ic_menu_mylocation)
                .setContentTitle("Waze Remoto ativo").setContentText("Monitorando destinos; navegação não é confirmada.").setContentIntent(content).setOngoing(true).build();
        startForeground(1, notification);
    }
    @Override public int onStartCommand(Intent intent, int flags, int startId) {
        if (!new SecureStore(this).enabled()) { stopSelf(startId); return START_NOT_STICKY; }
        if (worker != null && !SecureStore.GATE.isCurrent(generation)) {
            worker.interrupt();
            worker = null;
        }
        if (worker == null) {
            running = true;
            generation = SecureStore.GATE.current();
            final long workerGeneration = generation;
            worker = new Thread(new PollLoop(() -> {
                SecureStore.Config config;
                synchronized (SecureStore.LOCK) {
                    SecureStore.GATE.check(workerGeneration);
                    if (!running || Thread.currentThread().isInterrupted()) throw new InterruptedException();
                    SecureStore secure = new SecureStore(this);
                    if (!secure.enabled()) throw new InterruptedException();
                    config = secure.load();
                    if (config == null) throw new IllegalStateException("configuration missing");
                }
                DeviceApi api = new DeviceApi(new URI(config.origin), config.key);
                SecureStore.GATE.process(workerGeneration, new LastAttemptStore(this), api, uri -> new WazeLauncher(this).launch(uri));
            }, Thread::sleep, bound -> ThreadLocalRandom.current().nextLong(bound)), "destination-poll");
            worker.start();
        }
        return START_STICKY;
    }
    @Override public void onDestroy() { running = false; if (worker != null) { SecureStore.GATE.invalidateIfCurrent(generation); worker.interrupt(); } super.onDestroy(); }
    @Override public IBinder onBind(Intent intent) { return null; }
}
