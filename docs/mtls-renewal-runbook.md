# Traefik → nginx mTLS: renewal and alert runbook

**Status: no renewal or alert mechanism has been installed or validated by this runbook.** Both current leaves expire on 2027-02-06 around 13:00 UTC. Check live values; do not rely on this date after a rotation. The server and client CAs are distinct; retain both CAs and the existing private keys. Treat the CA private keys and all leaf private keys as secrets.

## Daily monitoring and response

An operator must arrange an independently monitored daily check of **both** leaf certificates, both CA certificates, the running nginx endpoint and the actual Traefik-to-nginx path. Alert a named on-call recipient on any failed check, on leaves within 30 days of expiry, and on CAs within 180 days of expiry. A scheduler exit code or local log alone is not an alert: verify delivery to the recipient and alert on missing scheduler heartbeat. Escalate undelivered alerts or repeated failures immediately. No recipient or monitoring integration has been configured here.

On the PKI host, the following read-only checks return nonzero when a leaf is within 30 days or a CA within 180 days of expiry. They do not verify the served certificate, process health, or alert delivery:

```sh
PKI=/opt/data/secrets/waze-pki-strict-20261108
openssl x509 -in "$PKI/proxy/server.crt" -checkend 2592000 -noout
openssl x509 -in "$PKI/traefik/client.crt" -checkend 2592000 -noout
openssl x509 -in "$PKI/ca/server-ca.crt" -checkend 15552000 -noout
openssl x509 -in "$PKI/ca/client-ca.crt" -checkend 15552000 -noout
openssl verify -x509_strict -purpose sslserver -verify_hostname proxy -CAfile "$PKI/ca/server-ca.crt" "$PKI/proxy/server.crt"
openssl verify -x509_strict -purpose sslclient -CAfile "$PKI/ca/client-ca.crt" "$PKI/traefik/client.crt"
```

The monitor must treat command errors and missing files as failures, not just expiration. Do not print private keys or authentication material in alert output.

## Safe renewal release gate

Do not change production certificates, global Traefik defaults, network isolation or the shared proxy merely to test this procedure. Before deploying automation:

1. Verify actual production mounts, effective Traefik file-provider configuration, nginx configuration, UID/access and both live certificate fingerprints. Make restricted, recoverable backups of current leaf certificates, keys and configuration; preserve and separately protect both original CA keys. Confirm backup restoration without publishing them.
2. Build a disposable isolated replica using the exact production Traefik and nginx images, mount topology and non-root nginx UID. Issue replacement leaves with their existing keys and respective original CAs, with distinct serials, matching SAN and purpose, bounded validity and correct ownership. Strictly verify signatures, hostname, purpose, certificate/key matching, permissions and remaining CA validity before activation.
3. Prove an atomic activation and rollback design under interruptions at each step. Individual file binds can retain the old inode after rename. Directory mounts permit atomic leaf replacement but do not by themselves make Traefik reload its in-memory client certificate. The shared Traefik dynamic configuration is also individually file-bound: replacing its inode may not trigger the file provider. Validate a scoped update/reload method against the exact mount and provider behavior; do not restart shared Traefik without evaluating impact to unrelated services. nginx requires its configuration tested and a validated reload/restart.
4. Exercise actual renewed mTLS traffic and negative tests (absent/untrusted client, untrusted server, wrong server name, wrong certificate purpose). Verify rejected requests never reach the backend. Test both normal operation and process/container restart after rotation, test failed reload and rollback, and prove no permissive fallback or isolation regression. Ensure any write failure leaves the old working pair recoverable.
5. Schedule the verified renewal with locking, bounded retries, durable failure reporting and independent expiry/heartbeat alerts. Test an actual forced rotation and subsequent restart in the isolated replica first; then, during an approved production maintenance window, execute a controlled production rotation with a verified rollback ready. Verify the live served server fingerprint and the certificate actually used by Traefik, end-to-end success and negative checks. Verify monitoring delivery and retain a timestamped record without secrets.

If any gate fails, stop, restore the previous verified state if activation began, verify service and notify the operator. A passing OpenSSL file check does not demonstrate that either process loaded the new certificate. Automation or future renewal must not be represented as working until these tests have actually succeeded.
