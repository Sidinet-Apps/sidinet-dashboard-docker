# SIDINET Dashboard Docker

SIDINET Dashboard es un dashboard web ligero y personalizable para organizar aplicaciones y visualizar información de servicios, Docker y del host. Está diseñado para ejecutarse en cualquier host que ya disponga de Docker/Compose, incluyendo Raspberry Pi, servidores Linux y NAS compatibles.

**SIDINET Dashboard no instala ni administra Docker, OpenMediaVault ni el sistema operativo.**

## Versión

**1.1.0**

## Funciones

- páginas, grupos y widgets personalizables;
- layouts independientes desktop/tablet/mobile de 12/8/4 columnas;
- editor visual nativo con drag, resize, undo/redo y guardado en lote;
- temas, fondos y presets visuales;
- aplicaciones y accesos directos;
- búsqueda y modo kiosk;
- métricas de CPU, RAM, load, temperatura y uptime;
- red, IP, Internet y DNS;
- almacenamiento configurado en modo read-only;
- Docker Discovery mediante proxy aislado de solo lectura;
- monitorización HTTP/HTTPS/TCP;
- widgets iframe y JSON;
- backup/restore de datos propios del dashboard;
- Safe Mode, Recovery y diagnóstico local.

## Límites de seguridad

El dashboard observa; no administra el host.

- el contenedor principal nunca monta `docker.sock`;
- el proxy Docker no publica puertos al host y solo permite consultas autorizadas GET/HEAD;
- no existen operaciones Docker Start/Stop/Restart/Create/Delete/Exec;
- `/proc`, thermal, `resolv.conf` y almacenamiento opcional se montan read-only;
- no se modifica DNS, red, firewall, usuarios, servicios, discos, SMB/NFS, VPN ni configuración del sistema operativo;
- el estado persistente propio se escribe únicamente bajo `/data`;
- root filesystem read-only, `cap_drop: ALL` y `no-new-privileges`.

Consulte `SECURITY.md` y ejecute `scripts/security-smoke-test.sh` sobre el despliegue antes de publicar un release.

## Distribución

Al publicar un tag `v1.1.0`, GitHub Actions ejecuta pruebas y construye la misma imagen para:

- `linux/amd64`
- `linux/arm64`

La imagen se publica en GHCR.

## Despliegue Docker Compose

Copie `compose.yml`, configure la imagen y levante el stack:

```bash
export SIDINET_IMAGE=ghcr.io/USUARIO-O-ORGANIZACION/sidinet-dashboard-docker:1.1.0
docker compose up -d
```

Por defecto el dashboard queda publicado en el puerto `8585`:

```text
http://IP-DEL-HOST:8585
```

Para Docker Discovery, configure `DOCKER_GID` con el GID que permite leer `/var/run/docker.sock` en el host.

`compose.dev.yml` es exclusivamente para desarrollo desde código fuente.

`compose.minimal.yml` ejecuta el dashboard sin integración Docker.

## Persistencia

Todo el estado persistente propio vive en `/data`:

- SQLite;
- configuración;
- páginas/widgets/layouts;
- temas;
- uploads;
- backups/recovery.

## Docker Discovery

```text
Browser
   |
SIDINET Dashboard
   |
red interna de Compose
   |
SIDINET Docker Proxy (read-only)
   |
Docker socket del host
```

El dashboard principal no tiene acceso directo al socket Docker.

## Desarrollo

Requiere Go 1.23+, compilador C y `libsqlite3` de desarrollo.

```bash
go test ./...
go build ./cmd/dashboard
go build ./cmd/docker-proxy
```

## Release 1.1.0

```bash
./scripts/security-smoke-test.sh

git tag v1.1.0
git push origin v1.1.0
```

El workflow `.github/workflows/release.yml` construye y publica el manifiesto multi-arquitectura.
