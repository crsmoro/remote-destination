package com.wazeremoto;

import android.content.Context;
import android.test.InstrumentationTestCase;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;

/** Uses synthetic credentials only; run on a disposable emulator. */
public class ProvisioningHarnessTest extends InstrumentationTestCase {
    public void testInvalidInputIsDeletedAndConsumesAttempt() throws Exception {
        Context c = getInstrumentation().getTargetContext();
        File dir = new File(c.getCacheDir(), "provision-invalid-" + System.nanoTime());
        assertTrue(dir.mkdir());
        File input = new File(dir, "input");
        Files.write(input.toPath(), "bad\n".getBytes(StandardCharsets.UTF_8));
        try {
            try { OneShotProvisioner.run(c, dir, new SecureStore(c)); fail("invalid accepted"); }
            catch (OneShotProvisioner.Failure expected) { }
            assertFalse(input.exists());
            assertTrue(new File(dir, "consumed").exists());
        } finally { input.delete(); new File(dir, "consumed").delete(); dir.delete(); }
    }
    public void testExistingConfigurationIsNotOverwritten() throws Exception {
        Context c = getInstrumentation().getTargetContext();
        File dir = new File(c.getCacheDir(), "provision-existing-" + System.nanoTime());
        assertTrue(dir.mkdir());
        File input = new File(dir, "input");
        String key = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
        SecureStore store = new SecureStore(c);
        store.clear();
        store.save(new SecureStore.Config("https://example.invalid", key));
        Files.write(input.toPath(), ("https://other.invalid\n" + key + "\n").getBytes(StandardCharsets.UTF_8));
        try {
            try { OneShotProvisioner.run(c, dir, store); fail("existing config overwritten"); }
            catch (OneShotProvisioner.Failure expected) { }
            assertFalse(input.exists());
            assertEquals("https://example.invalid", store.load().origin);
        } finally { store.clear(); input.delete(); new File(dir, "consumed").delete(); dir.delete(); }
    }
    public void testSavesAndRejectsReplay() throws Exception {
        Context c = getInstrumentation().getTargetContext();
        File dir = new File(c.getCacheDir(), "provision-test-" + System.nanoTime());
        assertTrue(dir.mkdir());
        String key = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
        File input = new File(dir, "input");
        Files.write(input.toPath(), ("https://example.invalid\n" + key + "\n").getBytes(StandardCharsets.UTF_8));
        SecureStore store = new SecureStore(c);
        store.clear();
        try {
            OneShotProvisioner.run(c, dir, store);
            assertFalse(input.exists());
            assertEquals(key, store.load().key);
            Files.write(input.toPath(), ("https://example.invalid\n" + key + "\n").getBytes(StandardCharsets.UTF_8));
            try { OneShotProvisioner.run(c, dir, store); fail("replay accepted"); }
            catch (OneShotProvisioner.Failure expected) { }
            assertFalse(input.exists());
        } finally { store.clear(); input.delete(); new File(dir, "consumed").delete(); dir.delete(); }
    }
}
