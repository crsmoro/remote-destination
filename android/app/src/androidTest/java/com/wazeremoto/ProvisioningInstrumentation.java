package com.wazeremoto;

import android.os.Bundle;
import android.test.InstrumentationTestRunner;

/** Test APK runner; a separate explicit argument is required to provision. */
public final class ProvisioningInstrumentation extends InstrumentationTestRunner {
    static boolean authorized;
    @Override public void onCreate(Bundle args) {
        authorized = args != null && "yes".equals(args.getString("authorize_provisioning"));
        super.onCreate(args);
    }
}
