# Docker Compose deployment

SIDINET Dashboard requires an existing Docker/Compose installation. It does not install, update or configure Docker or the host operating system.

1. Copy `compose.yml`.
2. Set `SIDINET_IMAGE` to the published GHCR image.
3. Optionally set `SIDINET_PORT`, `TZ` and `DOCKER_GID`.
4. Run `docker compose up -d`.
5. Open `http://HOST:8585` (or the configured port).

The main dashboard container has no Docker socket. Docker discovery is optional and uses the isolated `docker-proxy` service. Host metric mounts are read-only. Additional storage paths must be explicitly mounted read-only.

This deployment is compatible with hosts such as Debian, Ubuntu, Raspberry Pi systems and NAS platforms that provide Docker/Compose. OpenMediaVault is one possible host environment, not a dependency and not managed by SIDINET Dashboard.
