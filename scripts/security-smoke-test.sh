#!/usr/bin/env bash
set -uo pipefail
FAILURES=0; WARNINGS=0
pass(){ printf 'PASS  %s\n' "$1"; }
fail(){ printf 'FAIL  %s\n' "$1"; FAILURES=$((FAILURES+1)); }
warn(){ printf 'WARN  %s\n' "$1"; WARNINGS=$((WARNINGS+1)); }

printf '\nSIDINET Dashboard 1.0.0 Security Smoke Test\n============================================\n\n'
command -v docker >/dev/null 2>&1 || { echo 'FAIL  docker command not found'; exit 1; }
docker compose version >/dev/null 2>&1 || { echo 'FAIL  Docker Compose plugin not available'; exit 1; }

# Service names in the active Compose project. Current compose.yml uses sidinet-dashboard and docker-proxy.
DASHBOARD="$(docker compose ps -q sidinet-dashboard 2>/dev/null || true)"
[[ -z "$DASHBOARD" ]] && DASHBOARD="$(docker compose ps -q dashboard 2>/dev/null || true)"
PROXY="$(docker compose ps -q docker-proxy 2>/dev/null || true)"
[[ -n "$DASHBOARD" ]] || { fail "No running dashboard service in active Compose project"; exit 1; }
DASHBOARD_NAME="$(docker inspect "$DASHBOARD" --format '{{.Name}}' | sed 's#^/##')"
pass "Dashboard discovered: $DASHBOARD_NAME"

[[ "$(docker inspect "$DASHBOARD" --format '{{.HostConfig.Privileged}}')" == false ]] && pass 'Dashboard is not privileged' || fail 'Dashboard is privileged'
[[ "$(docker inspect "$DASHBOARD" --format '{{.HostConfig.ReadonlyRootfs}}')" == true ]] && pass 'Dashboard root filesystem is read-only' || fail 'Dashboard root filesystem is writable'
SEC="$(docker inspect "$DASHBOARD" --format '{{json .HostConfig.SecurityOpt}}')"; grep -q 'no-new-privileges' <<<"$SEC" && pass 'no-new-privileges enabled' || fail 'no-new-privileges missing'
CAPADD="$(docker inspect "$DASHBOARD" --format '{{json .HostConfig.CapAdd}}')"; [[ "$CAPADD" == null || "$CAPADD" == '[]' ]] && pass 'Dashboard adds no Linux capabilities' || fail "Dashboard adds capabilities: $CAPADD"
CAPDROP="$(docker inspect "$DASHBOARD" --format '{{json .HostConfig.CapDrop}}')"; grep -qi '"ALL"' <<<"$CAPDROP" && pass 'Dashboard drops all Linux capabilities' || fail 'Dashboard does not drop ALL capabilities'
UID_NOW="$(docker exec "$DASHBOARD" id -u 2>/dev/null || true)"; [[ -n "$UID_NOW" && "$UID_NOW" != 0 ]] && pass "Dashboard runs non-root (UID $UID_NOW)" || fail 'Dashboard runs root or UID unavailable'
[[ "$(docker inspect "$DASHBOARD" --format '{{.HostConfig.NetworkMode}}')" != host ]] && pass 'Host network namespace not used' || fail 'Dashboard uses host networking'
[[ "$(docker inspect "$DASHBOARD" --format '{{.HostConfig.PidMode}}')" != host ]] && pass 'Host PID namespace not used' || fail 'Dashboard uses host PID namespace'
[[ "$(docker inspect "$DASHBOARD" --format '{{.HostConfig.IpcMode}}')" != host ]] && pass 'Host IPC namespace not used' || fail 'Dashboard uses host IPC namespace'

DASH_MOUNTS="$(docker inspect "$DASHBOARD" --format '{{range .Mounts}}{{println .Source "|" .Destination "|" .RW}}{{end}}')"
grep -q '| /var/run/docker.sock |' <<<"$DASH_MOUNTS" && fail 'Dashboard has Docker socket' || pass 'Dashboard has no Docker socket'
awk -F'|' '$1 ~ /^[[:space:]]*\/[[:space:]]*$/ {found=1} END{exit !found}' <<<"$DASH_MOUNTS" && fail 'Host root filesystem is mounted' || pass 'Host root filesystem is not mounted'
while IFS='|' read -r SRC DEST RW; do
  DEST="$(xargs <<<"$DEST")"; RW="$(xargs <<<"$RW")"; [[ -z "$DEST" ]] && continue
  case "$DEST" in /host/*) [[ "$RW" == false ]] && pass "Host mount read-only: $DEST" || fail "Host mount writable: $DEST";; esac
done <<<"$DASH_MOUNTS"

if [[ -n "$PROXY" ]]; then
  [[ "$(docker inspect "$PROXY" --format '{{.HostConfig.Privileged}}')" == false ]] && pass 'Docker proxy is not privileged' || fail 'Docker proxy is privileged'
  PP="$(docker inspect "$PROXY" --format '{{json .HostConfig.PortBindings}}')"; [[ "$PP" == null || "$PP" == '{}' ]] && pass 'Docker proxy exposes no host ports' || fail "Docker proxy exposes host ports: $PP"
  PSOCK="$(docker inspect "$PROXY" --format '{{range .Mounts}}{{if eq .Destination "/var/run/docker.sock"}}{{println .RW}}{{end}}{{end}}')"; [[ "$PSOCK" == false ]] && pass 'Docker socket is read-only in proxy' || fail 'Proxy socket missing or writable'
else
  warn 'docker-proxy not active; proxy runtime checks skipped'
fi

docker ps --format '{{.Ports}}' | grep -Eq '(^|[^0-9])(2375|2376)([^0-9]|$)' && fail 'Docker API port 2375/2376 exposed' || pass 'Docker API ports 2375/2376 not exposed'

if [[ -f go.mod ]]; then
  FORBIDDEN='systemctl|shutdown|reboot|apt-get|dpkg|yum[[:space:]]|dnf[[:space:]]|iptables|nft[[:space:]]|ufw[[:space:]]|nmcli|mkfs|umount|useradd|usermod|groupadd|wg-quick'
  R="$(grep -RniE "$FORBIDDEN" . --exclude-dir=.git --exclude-dir=docs --exclude='*.md' --exclude='security-smoke-test.sh' 2>/dev/null || true)"
  [[ -z "$R" ]] && pass 'No forbidden host-administration commands found' || { fail 'Forbidden host-administration commands found'; printf '%s\n' "$R"; }
  WRITE='containers/.*/start|containers/.*/stop|containers/.*/restart|containers/.*/kill|containers/create|containers/.*/exec'
  R="$(grep -RniE "$WRITE" . --exclude-dir=.git --exclude-dir=docs --exclude='*.md' --exclude='security-smoke-test.sh' 2>/dev/null || true)"
  [[ -z "$R" ]] && pass 'No Docker management endpoints found' || { fail 'Docker management endpoints found'; printf '%s\n' "$R"; }
else warn 'Source unavailable; static checks skipped'; fi

CFG="$(docker compose config 2>/dev/null || true)"
grep -Eq 'privileged:[[:space:]]*true' <<<"$CFG" && fail 'Compose contains privileged mode' || pass 'Compose contains no privileged services'
grep -Eq 'network_mode:[[:space:]]*host' <<<"$CFG" && fail 'Compose uses host networking' || pass 'Compose does not use host networking'

printf '\n============================================\nFailures: %d\nWarnings: %d\n' "$FAILURES" "$WARNINGS"
(( FAILURES == 0 )) || { printf '\nSECURITY RESULT: FAILED — DO NOT RELEASE 1.0.0\n'; exit 1; }
printf '\nSECURITY RESULT: PASSED\n'; exit 0
