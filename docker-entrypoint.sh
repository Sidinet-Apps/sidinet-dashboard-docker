#!/bin/sh
set -eu

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

case "$PUID" in
    ''|*[!0-9]*)
        echo "ERROR: PUID must be a numeric UID" >&2
        exit 1
        ;;
esac

case "$PGID" in
    ''|*[!0-9]*)
        echo "ERROR: PGID must be a numeric GID" >&2
        exit 1
        ;;
esac

if [ "$PUID" -eq 0 ]; then
    echo "ERROR: PUID cannot be 0" >&2
    exit 1
fi

if [ "$PGID" -eq 0 ]; then
    echo "ERROR: PGID cannot be 0" >&2
    exit 1
fi

mkdir -p /data

CURRENT_UID="$(stat -c '%u' /data)"
CURRENT_GID="$(stat -c '%g' /data)"

if [ "$CURRENT_UID" != "$PUID" ] || [ "$CURRENT_GID" != "$PGID" ]; then
    echo "Preparing /data for PUID=${PUID} PGID=${PGID}"
    chown -R "$PUID:$PGID" /data
fi

echo "Starting SIDINET Dashboard as UID=${PUID} GID=${PGID}"

exec su-exec "$PUID:$PGID" "$@"