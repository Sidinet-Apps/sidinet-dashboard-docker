#!/usr/bin/env bash
set -uo pipefail

FAILURES=0
WARNINGS=0

pass(){ printf 'PASS  %s\n' "$1"; }
fail(){ printf 'FAIL  %s\n' "$1"; FAILURES=$((FAILURES+1)); }
warn(){ printf 'WARN  %s\n' "$1"; WARNINGS=$((WARNINGS+1)); }

printf '\nSIDINET Dashboard 1.1.0 Security Smoke Test\n'
printf '============================================\n\n'

command -v docker >/dev/null 2>&1 || {
    echo 'FAIL  docker command not found'
    exit 1
}

docker compose version >/dev/null 2>&1 || {
    echo 'FAIL  Docker Compose plugin not available'
    exit 1
}

# ------------------------------------------------------------
# Discover active services
# ------------------------------------------------------------

DASHBOARD="$(docker compose ps -q sidinet-dashboard 2>/dev/null || true)"

[[ -z "$DASHBOARD" ]] && \
    DASHBOARD="$(docker compose ps -q dashboard 2>/dev/null || true)"

PROXY="$(docker compose ps -q docker-proxy 2>/dev/null || true)"

[[ -n "$DASHBOARD" ]] || {
    fail "No running dashboard service in active Compose project"
    exit 1
}

DASHBOARD_NAME="$(
    docker inspect "$DASHBOARD" \
        --format '{{.Name}}' | sed 's#^/##'
)"

pass "Dashboard discovered: $DASHBOARD_NAME"

# ------------------------------------------------------------
# Dashboard runtime security
# ------------------------------------------------------------

[[ "$(docker inspect "$DASHBOARD" \
    --format '{{.HostConfig.Privileged}}')" == false ]] \
    && pass 'Dashboard is not privileged' \
    || fail 'Dashboard is privileged'

[[ "$(docker inspect "$DASHBOARD" \
    --format '{{.HostConfig.ReadonlyRootfs}}')" == true ]] \
    && pass 'Dashboard root filesystem is read-only' \
    || fail 'Dashboard root filesystem is writable'

SEC="$(
    docker inspect "$DASHBOARD" \
        --format '{{json .HostConfig.SecurityOpt}}'
)"

grep -q 'no-new-privileges' <<<"$SEC" \
    && pass 'no-new-privileges enabled' \
    || fail 'no-new-privileges missing'

# ------------------------------------------------------------
# Linux capabilities
#
# v1.1.0 requires exactly:
#
# CHOWN  - prepare ownership of /data
# SETUID - drop privileges to PUID
# SETGID - drop privileges to PGID
#
# No other added capabilities are allowed.
# ------------------------------------------------------------

CAPDROP="$(
    docker inspect "$DASHBOARD" \
        --format '{{json .HostConfig.CapDrop}}'
)"

grep -qi '"ALL"' <<<"$CAPDROP" \
    && pass 'Dashboard drops ALL Linux capabilities' \
    || fail 'Dashboard does not drop ALL capabilities'

CAPADD="$(
    docker inspect "$DASHBOARD" \
        --format '{{json .HostConfig.CapAdd}}'
)"

CAPADD_NORMALIZED="$(
    tr '[:lower:]' '[:upper:]' <<<"$CAPADD" |
    tr -d '[]" ' |
    tr ',' '\n' |
    sed '/^$/d' |
    sort
)"

EXPECTED_CAPS="$(
    printf '%s\n' CHOWN SETGID SETUID | sort
)"

if [[ "$CAPADD_NORMALIZED" == "$EXPECTED_CAPS" ]]; then
    pass 'Dashboard adds only CHOWN, SETGID and SETUID'
else
    fail "Dashboard capabilities are not restricted to CHOWN, SETGID and SETUID: $CAPADD"
fi

# ------------------------------------------------------------
# Final application process must be non-root
# ------------------------------------------------------------

UID_NOW="$(
    docker exec "$DASHBOARD" \
        sh -c 'awk "/^Uid:/ {print \$2}" /proc/1/status' \
        2>/dev/null || true
)"

if [[ -n "$UID_NOW" && "$UID_NOW" != 0 ]]; then
    pass "Dashboard PID 1 runs non-root (UID $UID_NOW)"
else
    fail "Dashboard PID 1 runs as root or UID unavailable"
fi

GID_NOW="$(
    docker exec "$DASHBOARD" \
        sh -c 'awk "/^Gid:/ {print \$2}" /proc/1/status' \
        2>/dev/null || true
)"

if [[ -n "$GID_NOW" && "$GID_NOW" != 0 ]]; then
    pass "Dashboard PID 1 runs with non-root GID $GID_NOW"
else
    fail "Dashboard PID 1 GID is root or unavailable"
fi

# ------------------------------------------------------------
# Namespace isolation
# ------------------------------------------------------------

[[ "$(docker inspect "$DASHBOARD" \
    --format '{{.HostConfig.NetworkMode}}')" != host ]] \
    && pass 'Host network namespace not used' \
    || fail 'Dashboard uses host networking'

[[ "$(docker inspect "$DASHBOARD" \
    --format '{{.HostConfig.PidMode}}')" != host ]] \
    && pass 'Host PID namespace not used' \
    || fail 'Dashboard uses host PID namespace'

[[ "$(docker inspect "$DASHBOARD" \
    --format '{{.HostConfig.IpcMode}}')" != host ]] \
    && pass 'Host IPC namespace not used' \
    || fail 'Dashboard uses host IPC namespace'

# ------------------------------------------------------------
# Dashboard mounts
# ------------------------------------------------------------

DASH_MOUNTS="$(
    docker inspect "$DASHBOARD" \
        --format '{{range .Mounts}}{{println .Source "|" .Destination "|" .RW}}{{end}}'
)"

grep -q '| /var/run/docker.sock |' <<<"$DASH_MOUNTS" \
    && fail 'Dashboard has Docker socket' \
    || pass 'Dashboard has no Docker socket'

awk -F'|' '
    $1 ~ /^[[:space:]]*\/[[:space:]]*$/ {
        found=1
    }
    END {
        exit !found
    }
' <<<"$DASH_MOUNTS" \
    && fail 'Host root filesystem is mounted' \
    || pass 'Host root filesystem is not mounted'

while IFS='|' read -r SRC DEST RW; do

    DEST="$(xargs <<<"$DEST")"
    RW="$(xargs <<<"$RW")"

    [[ -z "$DEST" ]] && continue

    case "$DEST" in
        /host/*)
            [[ "$RW" == false ]] \
                && pass "Host mount read-only: $DEST" \
                || fail "Host mount writable: $DEST"
            ;;
    esac

done <<<"$DASH_MOUNTS"

# ------------------------------------------------------------
# Docker proxy
# ------------------------------------------------------------

if [[ -n "$PROXY" ]]; then

    [[ "$(docker inspect "$PROXY" \
        --format '{{.HostConfig.Privileged}}')" == false ]] \
        && pass 'Docker proxy is not privileged' \
        || fail 'Docker proxy is privileged'

    PROXY_CAPADD="$(
        docker inspect "$PROXY" \
            --format '{{json .HostConfig.CapAdd}}'
    )"

    [[ "$PROXY_CAPADD" == null || "$PROXY_CAPADD" == '[]' ]] \
        && pass 'Docker proxy adds no Linux capabilities' \
        || fail "Docker proxy adds capabilities: $PROXY_CAPADD"

    PROXY_CAPDROP="$(
        docker inspect "$PROXY" \
            --format '{{json .HostConfig.CapDrop}}'
    )"

    grep -qi '"ALL"' <<<"$PROXY_CAPDROP" \
        && pass 'Docker proxy drops ALL Linux capabilities' \
        || fail 'Docker proxy does not drop ALL capabilities'

    PP="$(
        docker inspect "$PROXY" \
            --format '{{json .HostConfig.PortBindings}}'
    )"

    [[ "$PP" == null || "$PP" == '{}' ]] \
        && pass 'Docker proxy exposes no host ports' \
        || fail "Docker proxy exposes host ports: $PP"

    PSOCK="$(
        docker inspect "$PROXY" \
            --format '{{range .Mounts}}{{if eq .Destination "/var/run/docker.sock"}}{{println .RW}}{{end}}{{end}}'
    )"

    [[ "$PSOCK" == false ]] \
        && pass 'Docker socket is read-only in proxy' \
        || fail 'Proxy socket missing or writable'

else

    warn 'docker-proxy not active; proxy runtime checks skipped'

fi

# ------------------------------------------------------------
# Docker API must not be exposed publicly
# ------------------------------------------------------------

docker ps --format '{{.Ports}}' |
    grep -Eq '(^|[^0-9])(2375|2376)([^0-9]|$)' \
    && fail 'Docker API port 2375/2376 exposed' \
    || pass 'Docker API ports 2375/2376 not exposed'

# ------------------------------------------------------------
# Static source checks
# ------------------------------------------------------------

if [[ -f go.mod ]]; then

    FORBIDDEN='systemctl|shutdown|reboot|apt-get|dpkg|yum[[:space:]]|dnf[[:space:]]|iptables|nft[[:space:]]|ufw[[:space:]]|nmcli|mkfs|umount|useradd|usermod|groupadd|wg-quick'

    R="$(
        grep -RniE "$FORBIDDEN" . \
            --exclude-dir=.git \
            --exclude-dir=docs \
            --exclude='*.md' \
            --exclude='security-smoke-test.sh' \
            2>/dev/null || true
    )"

    [[ -z "$R" ]] \
        && pass 'No forbidden host-administration commands found' \
        || {
            fail 'Forbidden host-administration commands found'
            printf '%s\n' "$R"
        }

    WRITE='containers/.*/start|containers/.*/stop|containers/.*/restart|containers/.*/kill|containers/create|containers/.*/exec'

    R="$(
        grep -RniE "$WRITE" . \
            --exclude-dir=.git \
            --exclude-dir=docs \
            --exclude='*.md' \
            --exclude='security-smoke-test.sh' \
            2>/dev/null || true
    )"

    [[ -z "$R" ]] \
        && pass 'No Docker management endpoints found' \
        || {
            fail 'Docker management endpoints found'
            printf '%s\n' "$R"
        }

else

    warn 'Source unavailable; static checks skipped'

fi

# ------------------------------------------------------------
# Compose configuration
# ------------------------------------------------------------

CFG="$(docker compose config 2>/dev/null || true)"

grep -Eq 'privileged:[[:space:]]*true' <<<"$CFG" \
    && fail 'Compose contains privileged mode' \
    || pass 'Compose contains no privileged services'

grep -Eq 'network_mode:[[:space:]]*host' <<<"$CFG" \
    && fail 'Compose uses host networking' \
    || pass 'Compose does not use host networking'

# ------------------------------------------------------------
# Result
# ------------------------------------------------------------

printf '\n============================================\n'
printf 'Failures: %d\n' "$FAILURES"
printf 'Warnings: %d\n' "$WARNINGS"

if (( FAILURES != 0 )); then
    printf '\nSECURITY RESULT: FAILED - DO NOT RELEASE 1.1.0\n'
    exit 1
fi

printf '\nSECURITY RESULT: PASSED\n'
exit 0