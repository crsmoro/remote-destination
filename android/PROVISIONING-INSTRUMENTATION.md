# Disposable instrumentation provisioning (experimental)

This is a **debuggable-target-only** test APK, not a production receiver. It does not start the service. Do not run `connectedAndroidTest` without selecting a class: `ProvisionOnce` deliberately fails without explicit authorization, and the synthetic tests clear app configuration. Run synthetic tests **only on a disposable emulator**.

**Not deployment-ready:** the local app and test APKs use the current user's default debug key. A previously checked local debug certificate did not match the installed app certificates. Do not update, uninstall, or configure the car app with these artifacts. Build the test APK with the actual installed app's signing key, then compare signatures to a freshly pulled target APK before any installation. Do not replace that app or clear its data to get around a mismatch. This harness has only been tested on an Android 9 emulator.

For an already-authorized, explicitly selected ADB device, the intended flow below is illustrative and must not be used until the signing mismatch and target-specific risks are resolved. `SERIAL` must be the exact independently verified target; ADB device authorization and the explicit instrumentation flag are both required. Installing a test APK gives instrumentation access to the target app and is a privileged operation. An authorized ADB host on a debuggable app can read app data and remove the replay marker; this is not protection against that host. Never use a shell trace, terminal recording, secret in an argument/environment variable, or public/shared file. Protect the source file and its backups separately.

1. Build `:app:assembleDebugAndroidTest` with the installed app's original signing key; inspect test APK's signature and the installed target's signature. Check the target is debuggable, healthy, and currently unconfigured; this harness refuses to overwrite an existing `SecureStore` config. No vehicle operations were performed here.
2. Construct a private local payload containing exactly `https://host[:port]` then newline, followed by the 43-character base64url device key then newline. No additional whitespace. The example refers to a private file path, not a credential as a command argument.
3. Set `ADB` to the Android SDK adb binary, and `SERIAL` to the specific authorized device; verify `"$ADB" -s "$SERIAL" get-state` returns `device`. Install only the matching signed test APK with `"$ADB" -s "$SERIAL" install -r -t app/build/outputs/apk/androidTest/debug/app-debug-androidTest.apk`.
4. With shell tracing off, pipe the private payload over stdin. Use `set -o pipefail` in bash and verify both sides succeeded:

   ```bash
   set +x
   set -o pipefail
   cat /private/path/payload | "$ADB" -s "$SERIAL" shell -T "run-as com.wazeremoto sh -c 'umask 077; mkdir -p no_backup/provision-once; cat > no_backup/provision-once/input'"
   ```

   If transfer fails or is interrupted, attempt `"$ADB" -s "$SERIAL" shell "run-as com.wazeremoto rm -f no_backup/provision-once/input"` and verify absence. Interruption, process death, filesystem errors, or device loss can leave plaintext; deletion is not secure physical erasure. ADB transport confidentiality depends on the transport and device; don't assume an untrusted network is secure.
5. Invoke the exact class and authorization argument (the ADB exit status alone is insufficient; inspect `OK (1 test)` versus `FAILURES!!!`):

   ```bash
   "$ADB" -s "$SERIAL" shell am instrument -w \
     -e class com.wazeremoto.ProvisionOnce \
     -e authorize_provisioning yes \
     com.wazeremoto.test/com.wazeremoto.ProvisioningInstrumentation
   "$ADB" -s "$SERIAL" shell "run-as com.wazeremoto sh -c 'test ! -e no_backup/provision-once/input && test -f no_backup/provision-once/consumed'"
   ```

   The marker is created before validation/save and blocks subsequent attempts, including after a failure. The harness removes the staged file on handled success/failure, refuses replay and existing configuration, and reports only a generic failure. Do not delete the marker to work around an error without a deliberate recovery plan. Save and file deletion are separate operations; if deletion fails after save, the instrumentation reports failure even though config may have been stored. A failure is therefore not proof that nothing changed.
6. Remove **only the test APK** (`"$ADB" -s "$SERIAL" uninstall com.wazeremoto.test`) and verify it is absent. Do not uninstall the target. The app's existing `allowBackup=false` and the staging location exclude normal Android backup, but this is not a guarantee against privileged or forensic acquisition. The key is held in process memory and Java strings cannot reliably be wiped. No credential security claim covers a compromised host or device.

Test-only automated class: `-e class com.wazeremoto.ProvisioningHarnessTest` (without the authorization argument); it changes and clears the target app's secure configuration. The tests exercise synthetic save/load, replay, invalid payload, and existing-config rejection; the full ADB stdin flow was separately exercised on a disposable emulator, including no-auth rejection and a synthetic-key search of captured logcat. That finite log search is not proof that every Android component/version cannot log data.
