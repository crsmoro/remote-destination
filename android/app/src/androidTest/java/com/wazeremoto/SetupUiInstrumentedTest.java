package com.wazeremoto;

import android.content.Intent;
import android.test.ActivityInstrumentationTestCase2;
import android.text.method.PasswordTransformationMethod;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.EditText;
import android.widget.TextView;

/** Destructive: run only on an isolated disposable emulator. */
public final class SetupUiInstrumentedTest extends ActivityInstrumentationTestCase2<MainActivity> {
    private static final String ORIGIN = "https://127.0.0.1:1";
    private static final String TOKEN = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
    public SetupUiInstrumentedTest() { super(MainActivity.class); }
    public void testSetupPersistsAndStartsAfterStop() throws Exception {
        MainActivity a = getActivity();
        ViewGroup root = (ViewGroup) ((ViewGroup) a.findViewById(android.R.id.content)).getChildAt(0);
        EditText origin = (EditText) root.getChildAt(0), token = (EditText) root.getChildAt(1);
        Button start = (Button) root.getChildAt(2), stop = (Button) root.getChildAt(3);
        TextView status = (TextView) root.getChildAt(5);
        assertTrue(token.getTransformationMethod() instanceof PasswordTransformationMethod);
        assertFalse(token.isSaveEnabled());
        getInstrumentation().runOnMainSync(() -> { origin.setText(ORIGIN); token.setText(TOKEN); start.performClick(); });
        waitForStatus(status, "Monitoramento iniciado");
        assertEquals("", token.getText().toString());
        SecureStore.Config saved = new SecureStore(a).load();
        assertEquals(ORIGIN, saved.origin); assertEquals(TOKEN, saved.key);
        assertTrue(new SecureStore(a).enabled());
        getInstrumentation().runOnMainSync(stop::performClick);
        waitForStatus(status, "Monitoramento parado;");
        assertFalse(new SecureStore(a).enabled());
        getInstrumentation().runOnMainSync(() -> { origin.setText(ORIGIN); token.setText(TOKEN); start.performClick(); });
        waitForStatus(status, "Monitoramento iniciado");
        assertTrue(new SecureStore(a).enabled());
        assertEquals(TOKEN, new SecureStore(a).load().key);
        assertNotNull(a.getSystemService(android.app.ActivityManager.class).getRunningServices(100).stream().filter(s -> s.service.getClassName().equals(ReceiverService.class.getName())).findFirst().orElse(null));
    }
    private void waitForStatus(TextView status, String expected) throws Exception {
        for (int i = 0; i < 120; i++) {
            getInstrumentation().waitForIdleSync();
            if (status.getText().toString().startsWith(expected)) return;
            Thread.sleep(500);
        }
        fail("Expected status " + expected + ", got " + status.getText());
    }
}
