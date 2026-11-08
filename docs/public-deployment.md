# Isolated deployment guidance

This document describes requirements, not a ready-to-expose production configuration. Use a disposable environment with separate networks, data, credentials and DNS; do not reuse a production Compose project, volumes, reverse proxy or keys.

- Set `DEVICE_KEY_FILE` to a secret file containing 32 random bytes encoded as unpadded base64url. Mount it read-only and accessible to the non-root server process. Never commit it or place it in a URL.
- Keep the backend unreachable from untrusted networks. A trusted HTTPS reverse proxy must authenticate all web routes; the backend does not authenticate web users itself. Device routes require a separate, tightly scoped proxy rule and use the device key. Test route precedence and bypass attempts before any exposure.
- Optional M2M: set `AUTH0_AGENT_CLIENT_ID`, `AUTH0_AGENT_ISSUER` (for example `https://tenant.example.com/`) and `AUTH0_AGENT_AUDIENCE` (for example `https://api.example.com/agent`). These must match the identity provider and its dedicated RS256 client. If the client ID is set, missing or invalid issuer/audience prevents startup. Leave the client ID unset to disable the agent routes.
- If using the experimental egress relay, pass the same `AUTH0_AGENT_ISSUER` to it. The relay additionally permits the issuer hostname on port 443, subject to its public-IP checks. Isolate the relay; its IP-based allowlist is not identity-based authorization.
- Use persistent storage with restricted access and a backup/retention policy; SQLite secure deletion does not erase backups or snapshots. Use real TLS certificates and test JWT validation, proxy isolation, egress, Android/Waze behavior, recovery and rotation separately before exposure.

Changing these settings or deploying a new image can break existing integrations. Existing running images are unaffected by source edits; validate in isolation and plan migration separately before replacing them.
