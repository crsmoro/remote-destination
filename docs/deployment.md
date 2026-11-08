# Deployment: blocked pending authentication and isolation

**Do not expose or deploy this configuration to production.** `compose.yaml` is a quarantined starting point: Traefik discovery is disabled, no host port is published, and its private internal network is not the existing `traefik` network. It is not a working remotely accessible deployment; the internal network also prevents outbound resolution of external destination links. No infrastructure was changed.

## Existing proxy considerations

The candidate assumes Traefik and forward-auth protecting both HTTP and HTTPS entrypoints. Verify the actual providers, middleware, TLS, rules and trust boundaries in your own environment. Do not remove or override shared entrypoint middleware globally.

The hostname `remote.example.com` below is an illustrative placeholder, not a production hostname. The backend checks a bearer device key for `/device/*`; the actual device operations are POST `/device/poll`, `/device/claim`, and `/device/ack`. All user-facing routes rely on the proxy for authentication; the backend does not authenticate users itself. In particular, the device bearer credential must not grant access to web or user API routes.

## Release gates

1. Determine and review the exact forward-auth version's matching semantics and configure **only** the intended host, exact three paths, and POST method to pass through forward-auth. All other requests must still require normal user authentication. Confirm path normalization, encoded paths, case, trailing slashes and routing precedence cannot extend the exception. Do not implement a broad `/device/` or host-wide bypass. The existing forward-auth has no such rule configured; changing its policy is a separate infrastructure change, not included here.
2. Establish isolation between Traefik and the backend. Joining the shared `traefik` network alone is unsafe: other containers on that network can connect directly to the backend and bypass user authentication. A dedicated private network shared only by Traefik and the backend, with appropriate host and Docker administrator trust boundaries, is one possible design; verify it in the actual environment. No published backend port. Merely setting `traefik.docker.network=traefik` does not provide this isolation.
3. Only after those gates, enable discovery and add an explicit router for ``Host(`remote.example.com`)`` on `https`, TLS, and a service targeting container port 8080 on the verified private network. These are design requirements, **not ready-to-apply labels**: the private network and exact policy have not been established. Ensure the existing HTTP-to-HTTPS route and global middleware remain effective. Check there is no competing router.
4. Provision one 32-byte cryptographically random device key encoded as unpadded base64url in a file outside the repository. `DEVICE_KEY_HOST_FILE` is an absolute host path, not the key itself. Deliver the same value securely to the client; never put it in Compose, labels, shell history, logs, or source control. The image runs as non-root `app`; check its numeric UID/GID and ensure the mounted file is readable by that account while restricting other readers. Compose file-backed secrets may be bind mounts and may not honor requested UID/mode remapping. Protect the host and backups. Docker administrators can access the key.
5. Preserve and back up the `destination_data` volume, including SQLite consistency and permissions. Test restores. Do not remove the volume during upgrades or rollback. Confirm short-link outbound connectivity in the final network design.
6. After authorized implementation, test through the real HTTPS edge: unauthenticated web and user API requests are rejected or redirected; authenticated users can use them; missing and wrong device keys fail each device endpoint; a valid device key reaches only the three exact POST paths; alternate methods, paths and encodings do not bypass user auth; and direct access from an unrelated container fails. Validate certificate/hostname and test actual Android operation. Avoid logging keys or sensitive destinations. Until all tests pass, keep the service unexposed.

## Local validation (no deployment)

From the repository root, once Docker Compose v2 and a local test-only key file path are available:

```sh
DEVICE_KEY_HOST_FILE=/absolute/path/to/test-only-device.key docker compose config -q
```

This checks Compose parsing/interpolation only; it does not prove that the key exists, that the non-root process can read it, that the image builds, or that any security boundary works. The file-backed secret path must exist on the Docker host when a container is eventually started. The actual server validates its content at startup: exactly 32 decoded bytes in unpadded base64url form, with surrounding whitespace trimmed.

For a later authorized release, build and runtime commands depend on the approved isolation and authentication design; deliberately no `up` or proxy-change command is provided while the release gates are open.
