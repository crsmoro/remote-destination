# Isolated mTLS bridge: test and deployment design (not a release)

**Do not expose this design yet.** These assets make no production route, change no existing Traefik service, and do not alter `compose.yaml`. `proxy.conf` is an nginx configuration demonstrated with disposable fixtures, not a complete production deployment. This is a design and an isolated security test, not a claim that the real Traefik, application, authentication policy, or Google egress works.

## Proposed boundary

* An nginx proxy joins the existing Traefik network and a new, project-only internal network; the application joins only the latter. Neither publishes a host port. The proxy forwards to `backend:8080`. Restrict private-network membership to the proxy and application. The Docker host and Docker administrators remain trusted; network isolation does not defend against them, or a compromised proxy.
* Traefik must use an HTTPS ServersTransport with server-name `proxy` (or a dedicated stable DNS alias matching the certificate SAN), verify a dedicated server CA, and present a client certificate and key. Never enable insecureSkipVerify. Existing Traefik must have securely mounted access to those credentials and trust material; that is a separate infrastructure change requiring approval. A Docker label referring to a transport alone does not load the necessary keys.
* nginx requires a valid client certificate from `client-ca.crt`. **This is CA-based authorization, not certificate-subject pinning**: every suitable client certificate issued by this CA can connect. Use a dedicated narrowly controlled client CA solely for Traefik, issue only the intended certificate, prevent private-key access by shared-network workloads, plan rotation/revocation, and test expiry. Use a separate server CA. Neither CA private key belongs on the runtime host or in this repository. nginx's server key and Traefik's client key are secrets; mount read-only, restrict host/container permissions, do not bake into images, and ensure the actual nginx worker can read its key at startup. Temporary test certificates are not production credentials.
* The example removes forwarded identity/address headers. Confirm the application does not require them. Authentication is still the existing global Traefik forward-auth; mTLS does not authenticate users and is not a replacement for it. A valid Traefik client certificate is effectively authority to bypass that middleware by connecting directly to the proxy, so protect it accordingly. Device-key access is independent and must remain limited to the exact three POST routes in `docs/deployment.md`.
* Declare a single explicit router/service/network and an exact hostname only after authorization and release gates are satisfied. Keep Traefik global forward-auth in place. An exact device exception, scoped to host, method and path, requires an independently reviewed change to the existing forward-auth policy; do not create a broad exception or turn it off. Test normalization and encoded paths. No production labels are provided here because they would be unusable without separately installing and validating the ServersTransport and forward-auth policy.

## Egress remains a release gate

An experimental opt-in private-bridge CONNECT relay and its limitations are documented in [egress.md](egress.md). It has not been validated in the real Compose topology or against live Google, and does not replace the inbound mTLS/authentication gates. A second ordinary bridge on the backend would provide broad outbound access and can expose the listener to peers on that network; do not use one as a substitute.

## Running the isolated test

`PORTAINER_API_URL` and `PORTAINER_API_TOKEN` must already be set securely; the script does not print them. With Python 3 and OpenSSL installed, and existing `nginx:latest` and `node:alpine` images on Jarvis:

```sh
python3 security-design/integration.py
```

The script verifies the Docker host name is `jarvis`, creates uniquely labelled temporary internal networks and containers, generates ephemeral test-only keys in the scratch directory, and removes its own resources in `finally`. It verifies cleanup via Docker API. It uses no live network and publishes no ports. If killed abruptly or Portainer becomes unreachable, cleanup may fail; inspect and remove only resources bearing the reported test label. Test keys are intentionally readable within disposable test containers; that is not the production secret-handling design. Docker administrators can inspect them.

The fixture checks: trusted client → proxy → backend succeeds; absent/untrusted client certificates fail; server name mismatch fails; a shared-network client cannot connect directly to the actual private backend IP; selected spoofed forwarding headers are stripped. It validates nginx by starting it with `proxy.conf` and performing requests. It does not test actual Traefik's ServersTransport, real application, forward-auth, public edge, other headers, client certificate expiry/revocation, controlled egress, DNS rebinding, host-level isolation, or production certificates. The image tags are test fixtures and must be pinned to reviewed digests for production.

## Remaining release gates

Provision and validate the dedicated CAs/leaf certificates and least-privilege secret mounts; agree how Traefik receives trust and client keys; test the actual Traefik-to-proxy mTLS path including wrong server CA/name and unauthorized client; solve controlled application egress; implement and independently test exact forward-auth exceptions; test real web/device authorization and edge behavior; verify backups/restore, restart and rotation; then obtain explicit production authorization. Existing `docs/deployment.md` release gates remain in force.
