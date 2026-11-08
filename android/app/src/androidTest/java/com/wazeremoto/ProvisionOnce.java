package com.wazeremoto;

import android.content.Context;
import android.test.InstrumentationTestCase;
import java.io.File;

/** Invoke this exact method only with explicit ADB authorization. */
public final class ProvisionOnce extends InstrumentationTestCase {
    public void testProvision() {
        if (!ProvisioningInstrumentation.authorized) { fail("explicit authorization required"); return; }
        Context c = getInstrumentation().getTargetContext();
        try {
            OneShotProvisioner.run(c, new File(c.getNoBackupFilesDir(), "provision-once"), new SecureStore(c));
        } catch (Throwable ignored) { fail("provisioning rejected"); }
    }
}
