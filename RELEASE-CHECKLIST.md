# Public release checklist

Do not publish until every applicable item is reviewed and approved. A public source release does not mean the service is safe to deploy.

- [ ] Owner selects a license and adds its actual text; until then, do not imply a license or permission to reuse.
- [ ] Review every file to be published, including source, tests, configuration, documentation, binary assets and generated files; remove private hostnames, infrastructure details, personal destinations and real provisioning data. Replace test inputs with synthetic examples.
- [ ] Run a dedicated secret scanner on the exact staged snapshot and inspect findings privately. Check for private keys, access tokens, service accounts, signing material, credential-bearing URLs and sensitive JSON; rotate any exposed credentials. Pattern scanning and `.gitignore` cannot guarantee detection.
- [ ] Check ignored and untracked files with `git status --ignored`, inspect the exact staged names and diff, and confirm no keys, databases, APKs, device payloads, logs or local configuration are included. Ignore rules do not remove already tracked files.
- [ ] Review hard-coded issuer/audience, Compose hostnames and test infrastructure references; coordinate source changes with their maintainers and rerun the relevant tests. Do not publish private infrastructure details by accident.
- [ ] Validate builds, automated tests, dependencies and supported toolchains; review third-party licenses and attribution.
- [ ] Independently complete the deployment security gates in `docs/deployment.md`, including auth, isolation, certificate validation, egress, backups and device testing, before exposing any service.
- [ ] Confirm repository visibility, owner approval, release contents and intended remote immediately before publishing. No deployment or remote write is authorized by this checklist.
