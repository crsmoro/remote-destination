package com.wazeremoto;

import android.content.Context;
import android.test.InstrumentationTestCase;

/** Explicitly invoked only on a disposable emulator to prepare a reboot test. */
public class RebootFixture extends InstrumentationTestCase {
    public void testSeed() throws Exception {
        Context app = getInstrumentation().getTargetContext();
        synchronized (SecureStore.LOCK) {
            new LastAttemptStore(app).save("fixture-id", "failed", true);
            new SecureStore(app).save(new SecureStore.Config("https://127.0.0.1:9", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"));
        }
    }
    public void testVerify() throws Exception {
        Context app = getInstrumentation().getTargetContext();
        assertTrue(new SecureStore(app).enabled());
        assertEquals("https://127.0.0.1:9", new SecureStore(app).load().origin);
        assertEquals("fixture-id", new LastAttemptStore(app).attempted());
        assertEquals("failed", new LastAttemptStore(app).result());
        assertTrue(new LastAttemptStore(app).pending());
    }
}
