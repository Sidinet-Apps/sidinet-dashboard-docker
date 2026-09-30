# Security

- Main dashboard runs as an unprivileged user.
- Root filesystem is read-only in the deployment Compose.
- All Linux capabilities are dropped.
- `no-new-privileges` is enabled.
- Main dashboard never mounts the Docker socket.
- Docker proxy is internal-only, method-restricted and path-allowlisted.
- No Docker write API is implemented in V1.
- Compose files, when later mounted for discovery, must be read-only.

## Reporting
Do not publish credentials, tokens, Compose environment secrets, or private infrastructure details in public issues.
