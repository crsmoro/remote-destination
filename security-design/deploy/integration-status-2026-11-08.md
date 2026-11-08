# Candidate status — no production deployment

Use the existing production Traefik only. No second Traefik is to be installed or retained. The disposable test below is not production.

## Forward-auth candidate

The following is an illustrative candidate, not a ready-to-deploy rule. Use the approved hostname and preserve every existing argument and rule:

```text
--rule.remote-destination-device-v1.action=allow
--rule.remote-destination-device-v1.rule=Host(`remote.example.com`) && Method(`POST`) && (Path(`/device/poll`) || Path(`/device/claim`) || Path(`/device/ack`))
```

The `Path` matcher is not raw-byte equality. In a real isolated Traefik → forward-auth → nginx → Go backend test, `/device/%70oll` with a valid bearer returned 200, and without a bearer returned 401. Queries and normalized forms may match as well. Thus the security condition is **valid backend bearer**, not a promise that only literal bytes bypass user login. Rejecting all encoded alternatives would require a separate validated raw-path control. The bearer must never authorize UI/user API routes. The existing `rule_overlay.py` safely transforms JSON argument arrays and refuses collisions or rollback if the candidate has changed; it does not modify infrastructure. Do not store a production command array if it contains credentials in a tracked or world-readable file.

## Actual test on 2026-11-08

A disposable isolated test copied the real forward-auth image and command with the candidate, and used a disposable Traefik v2.8, the candidate nginx mTLS config and the genuine backend image. A test-only client on the private network seeded a destination directly, simulating an authenticated UI; then through disposable Traefik with test credentials the real backend returned 200 for poll (matching command ID), claim and ack, followed by an empty poll. Missing and invalid bearer returned 401 for poll (including `%70oll`), claim and ack. UI without login returned 307; forged forwarded host did not bypass; direct proxy without certificate returned 400; direct backend access from shared network failed; auth stopped produced 500. Test resources were removed: zero containers and networks remained.

This was **not** the complete production Compose: the test assembled containers with Docker API, used disposable certificates and a test key, did not run the egress relay, did not test real user login, and did not use production PKI. It does not certify DNS/egress, Cloudflare, the production mount permissions, or the existing Traefik file provider and reload behavior. The copied live forward-auth is directly reachable by shared-network peers in the test, so its decision is not an independent authorization boundary.

## Deployment remains blocked

1. Obtain the pending approved read of the exact production Compose and dynamic file; inspect their structure, secret handling, reload and mounts. Do not bypass the pending approval. Back up both files securely and verify a restore; preserve all rules and unrelated configuration. Test the final patch on copies with actual Compose inputs. Do not invent a service recreation command until its real configuration and dependencies are known.
2. Independently certify the private egress/DNS integration and the **complete candidate Compose** in an isolated setup, including actual app, relay, mTLS permissions and complete flow. The test above does not satisfy this gate.
3. Mount the existing production mTLS assets only in the existing Traefik and candidate nginx, read-only; install the reviewed `traefik-dynamic.yaml` transport into its existing provider without overwriting existing entries. Explicitly retain server verification and `serverName: proxy`; verify reload and both negative/positive certificate cases.
4. Back up and test restoration of the persistent SQLite data. Establish image pinning, secret permissions, renewal and rollback; validate authenticated UI, device, adversarial paths/headers and Cloudflare-to-origin behavior. Confirm no competing router. Preserve the existing global auth middleware and all other services.
5. Only once all gates pass and the final, exact changes are reviewed, apply narrowly scoped commands against the existing services; verify configuration and effective routing read-back. Never run `down -v`, remove volumes, prune, or restart unrelated services. If any stage fails, restore the previously verified exact files and reload only the affected existing service(s), using commands based on the actual inspected configuration.

Until those prerequisites are met there are deliberately no executable production `up`, restart, or file replacement commands in this document: doing so would imply that the unseen production configuration and rollback have been validated.
