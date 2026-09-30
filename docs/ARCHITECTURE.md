# Architecture

Browser -> SIDINET Dashboard (Go + SQLite + embedded static UI) -> Providers.

Docker discovery uses a separate `sidinet-docker-proxy` process on an internal-only Docker network. The main dashboard container never receives `/var/run/docker.sock`.

Persistent application state is under `/data`. Host metrics are read from read-only `/host/proc` and `/host/sys/class/thermal` mounts. Docker and Compose are execution/integration targets; the project never installs Docker or OpenMediaVault.
