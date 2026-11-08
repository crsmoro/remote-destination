package com.wazeremoto;

import android.app.ActivityManager;
import android.content.Context;
import android.content.Intent;
import android.test.InstrumentationTestCase;
import java.util.concurrent.atomic.AtomicInteger;

public class ReceiverInstrumentedTest extends InstrumentationTestCase {
    private Context app;
    @Override protected void setUp() throws Exception {
        super.setUp(); app = getInstrumentation().getTargetContext();
        app.stopService(new Intent(app, ReceiverService.class));
        synchronized (SecureStore.LOCK) { new SecureStore(app).clear(); new LastAttemptStore(app).clear(); }
    }
    @Override protected void tearDown() throws Exception {
        app.stopService(new Intent(app, ReceiverService.class));
        synchronized (SecureStore.LOCK) { new SecureStore(app).clear(); new LastAttemptStore(app).clear(); }
        super.tearDown();
    }
    public void testKeystoreRoundTripAndNoPlaintextPreferences() throws Exception {
        String key = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
        SecureStore store = new SecureStore(app); store.save(new SecureStore.Config("https://127.0.0.1:9", key));
        SecureStore.Config loaded = new SecureStore(app).load();
        assertEquals(key, loaded.key); assertEquals("https://127.0.0.1:9", loaded.origin);
        assertFalse(app.getSharedPreferences("config", Context.MODE_PRIVATE).getAll().toString().contains(key));
        assertFalse(app.getSharedPreferences("config", Context.MODE_PRIVATE).getAll().toString().contains(loaded.origin));
        assertTrue(store.enabled()); store.clear(); assertNull(store.load());
    }
    public void testPendingAckSurvivesStoreRecreationWithoutRelaunch() throws Exception {
        AtomicInteger launched = new AtomicInteger(), acked = new AtomicInteger();
        CommandProcessor.Api api = new CommandProcessor.Api() {
            public CommandProcessor.Command poll() { return new CommandProcessor.Command("one", "https://waze.com/ul?q=x"); }
            public boolean claim(String id) { return true; }
            public void ack(String id, String result) throws Exception { assertEquals("intent_sent", result); if (acked.getAndIncrement() == 0) throw new Exception(); }
        };
        try { new CommandProcessor(new LastAttemptStore(app)).process(api, uri -> launched.incrementAndGet()); fail(); } catch (Exception expected) { }
        assertTrue(new LastAttemptStore(app).pending());
        new CommandProcessor(new LastAttemptStore(app)).process(api, uri -> launched.incrementAndGet());
        new CommandProcessor(new LastAttemptStore(app)).process(api, uri -> launched.incrementAndGet());
        assertEquals(1, launched.get()); assertEquals(2, acked.get()); assertFalse(new LastAttemptStore(app).pending());
    }
    private boolean running() {
        ActivityManager manager = (ActivityManager) app.getSystemService(Context.ACTIVITY_SERVICE);
        for (ActivityManager.RunningServiceInfo info : manager.getRunningServices(100)) if (info.service.getClassName().equals(ReceiverService.class.getName()) && info.foreground) return true;
        return false;
    }
    private void waitFor(boolean expected) throws Exception {
        for (int i=0; i<100 && running()!=expected; i++) Thread.sleep(100);
        assertEquals(expected, running());
    }
    public void testBootStartsForegroundServiceOnlyWhenEnabledAndServiceSurvivesActivityAbsence() throws Exception {
        new SecureStore(app).save(new SecureStore.Config("https://127.0.0.1:9", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"));
        BootReceiver receiver = new BootReceiver();
        receiver.onReceive(app, new Intent(Intent.ACTION_BOOT_COMPLETED));
        waitFor(true); Thread.sleep(1500); assertTrue(running());
        app.stopService(new Intent(app, ReceiverService.class)); waitFor(false);
        new SecureStore(app).setEnabled(false);
        receiver.onReceive(app, new Intent(Intent.ACTION_BOOT_COMPLETED)); Thread.sleep(1000); assertFalse(running());
    }
    public void testWazeNotInstalledReportsFailureWithoutFallback() throws Exception {
        try { new WazeLauncher(app).launch("https://waze.com/ul?q=test&navigate=yes"); fail("Waze unexpectedly available"); }
        catch (android.content.ActivityNotFoundException expected) { }
    }
}
