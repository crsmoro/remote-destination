# mTLS leaf staging (not a renewal deployment)

`stage-mtls-leaf.sh` signs **one** leaf using the supplied existing leaf key and CA key. It never installs certificates, triggers reloads, touches Docker, changes the CA, schedules work, or performs rollback. It is a staging aid, not a production renewal mechanism. Do not activate its output until the runbook's isolation, reload, rollback, monitoring and production maintenance gates are satisfied.

Run on the PKI host in a restricted environment with OpenSSL and Bash, from a trusted private parent directory on a private filesystem. This creates a new private output directory containing `leaf.crt` and `leaf.key`; it will not overwrite an existing output path. The output private key is a copy of the supplied key; protect and securely dispose of all copies. Both the output directory and files are private to the invoking user; authorized container UID/ownership and readable permissions must be configured and validated separately.

```
scripts/stage-mtls-leaf.sh server /path/server-ca.crt /path/server-ca.key /path/server.key /private/new-server proxy 30
scripts/stage-mtls-leaf.sh client /path/client-ca.crt /path/client-ca.key /path/client.key /private/new-client traefik-client 30
python3 -m unittest discover -s tests -p 'test_stage_mtls.py' -v
```

The identity is an explicitly supplied ASCII DNS-style name; a server certificate has exactly one DNS SAN, while a client certificate has no SAN. It does **not** copy or discover existing SANs, subject fields, extensions, ownership or permissions. Before using it for a particular deployment, inspect the actual existing certificates and clients; do not use it if additional identities or extensions must be retained. Maximum validity is 365 days and the CA must remain valid at least one day beyond the requested period. It checks CA key matching, leaf key validity, strict signature/purpose, server hostname and key match. It does not check revocation, trust distribution, running process state or end-to-end connectivity. CA and leaf key file paths are arguments (not the secret bytes); do not enable shell tracing or log sensitive path details. Prefer an offline CA and approved signing controls.

No automatic restart, renewal or alert is installed. A successful staging command is not evidence that nginx or Traefik loaded the result. Back up and protect all source material before any separately authorized deployment.
