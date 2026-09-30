# Changelog

## 1.1.1

- Restored default deployment port 8088.
- Fixed Docker Discovery frontend endpoint.
- Added visual container picker with image, state, URL and one-click adoption.
- Added automatic application logos using Docker image/label identity with safe fallback.
- Refined application cards and discovery UI for a cleaner launcher-style dashboard.
- Repaired dashboard CRUD: edit application/card properties, duplicate with layouts, delete with orphan cleanup, and protected edit controls from drag capture.

## 1.1.0

- Rediseño visual del dashboard con navegación lateral, barra superior y presentación renovada de tarjetas.
- Flujo restaurado y mejorado para agregar aplicaciones y widgets desde el modo de edición.
- Tarjetas de aplicaciones y métricas adaptadas al nuevo diseño, conservando layouts responsive y edición visual.
- Mejoras en renderizado de métricas del sistema y detección de zonas térmicas.
- Soporte de PUID/PGID para permisos de los datos persistentes.
- Workflow manual de betas con numeración automática, imagen multi-arquitectura y prerelease de GitHub.
- Se mantienen los límites de seguridad: dashboard de observación, Docker Discovery mediante proxy read-only y sin administración de OMV/host.

## 1.0.0

- Stable release based on 0.13.0; no new product features.
- Finalized generic Docker/Compose documentation and security release gate.
- Added Compose-aware security smoke test for host/Docker isolation.


## 0.13.0
- Technical stabilization only; no new dashboard features.
- Fixed CPU first-sample warm-up reporting.
- Completed host IP provider with IPv6 from `/proc/net/if_inet6`.
- DNS health check now tests the host-configured nameserver directly when available.
- Added CI and GHCR Buildx workflows for `linux/amd64` and `linux/arm64`.
- Kept the same dashboard + optional read-only Docker proxy deployment architecture.

## 0.12.0
- Final dashboard UX feature release.
- Native Pointer Events drag and resize; no JS framework added.
- 12/8/4 breakpoint-aware layouts with batched save.
- In-memory undo/redo and cancel snapshot while editing.
- Widget property editor and optional groups.
- Lightweight in-page search and kiosk presentation mode.
- Generic iframe and JSON display widgets.
- Additive migration for optional widget parent/group relationship.
- No new containers, databases, queues, or runtime services.

## 0.11.0
- Provider TTL cache + singleflight and bounded collection concurrency.
- HTTP hardening and internal performance diagnostics.
- SQLite WAL checkpoint tuning and Raspberry Pi validation guide.
- Failure isolation and pre-RC hardening.

# 0.9.0
- Theme Engine global/per-page with strict property allowlist.
- Presets: SIDINET Dark, Light, Minimal, Glass, Transparent and OLED.
- Live appearance editor with save/cancel behavior.
- Background URL, card opacity/blur, typography, spacing and navigation variables.
- Reduced-motion support and no new polling/timers for appearance.
- Theme API and persisted JSON settings in SQLite.

# 0.8.0
- Shared network providers: interfaces, RX/TX rates, host IP detection, Internet and DNS.
- Shared storage provider: total/used/free space for explicit read-only paths.
- Network/storage widgets and runtime integration.
- Optional host DNS/storage mounts; missing paths degrade safely.


## 0.7.0

- Motor de monitoreo compartido con pool acotado de workers.
- Monitores HTTP/HTTPS y TCP con timeout y latencia.
- Estados ONLINE, DEGRADED, OFFLINE y UNKNOWN.
- Backoff progresivo hasta 300 segundos después de fallos.
- Estado del monitor integrado en el runtime de shortcuts.
- API `/api/v1/monitors` para consulta y alta.
- Adopción Docker crea/reutiliza monitor HTTP automáticamente.
- Migración aditiva desde 0.6.x sin reconstruir `/data`.

## 0.6.0

- Docker Discovery normalizado y persistente.
- Identidad estable Compose proyecto/servicio.
- Resolución de labels, puertos y URL sugerida.
- Adopción idempotente como aplicación + shortcut.
- Ignorar sugerencias desde UI.
- Docker proxy reducido a ping/version/info/list/inspect/stats.

## 0.4.0 - Dynamic runtime and responsive layouts
- Runtime API by page and breakpoint.
- Persistent desktop/tablet/mobile widget layouts.
- Lightweight visual layout editor with batch saves.
- Dynamic page navigation and widget rendering.
- Default system widgets on fresh installations.


## 0.2.0 - GitHub/OMV foundation

- Separated production OMV Compose from source-development Compose.
- Added GitHub Actions tests and GHCR multi-architecture publishing.
- Added an internal read-only Docker socket proxy binary.
- Added Docker status/container read API to the dashboard.
- Added proxy allowlist security tests.
- Added OMV deployment and security documentation.
- Production Compose now assumes Docker/Compose already exists and never installs Docker.

## 0.1.0 - Bootstrap

- Go application core.
- SQLite/WAL foundation.
- System providers and runtime API.
- Embedded responsive frontend.

## 0.5.0
- Widget Store in the browser.
- Create pages from the browser.
- Create persistent applications and application shortcuts.
- Add, rename, duplicate, hide and delete widgets.
- Application shortcuts resolve persistent application metadata at runtime.
- Responsive layout editor retained for desktop, tablet and mobile.

## 0.10.0
- Consistent SQLite backups with manifest/checksums and uploads.
- Staged restore with ZIP safety checks and restart-time application.
- Recovery status, diagnostics, and safe-mode recovery endpoints.
- Restore rollback protection and archive limits.
