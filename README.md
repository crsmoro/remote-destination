# Waze Remoto

A Go destination server and an Android receiver for sending a navigation destination to Waze. The server stores commands; the receiver polls, claims a command and attempts to launch Waze. It also provides a web interface and an optional authenticated machine-to-machine API.

- [Server design and configuration](server/README.md)
- [Android build and behavioral limitations](android/README.md)
- [Isolated deployment guidance](docs/public-deployment.md)
- [Public release checklist](RELEASE-CHECKLIST.md)

The backend does **not** authenticate web users itself. A correctly configured trusted authentication proxy and network isolation are required; the included Compose files are not a validated public deployment. Device delivery, background launches and real-car navigation are not guaranteed. Review security and test in an isolated environment before any exposure.

MIT licensed; see [LICENSE](LICENSE).

The optional `iris_m2m.py` client reads JSON configuration from `~/.config/waze-remoto/iris-m2m.json` (or the `WAZE_IRIS_CONFIG` environment variable). Provide `client_id`, `base`, `token_url`, and `secret_file`; keep the configuration and secret file outside the repository with restrictive permissions. The `send` command reads the destination from standard input.
