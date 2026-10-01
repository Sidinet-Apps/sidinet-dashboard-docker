#!/usr/bin/env bash
set -euo pipefail

IMAGE="sidinet-dashboard:persistence-test"
NAME="sidinet-dashboard-persistence-test"
PORT="18088"
DATA_DIR="$PWD/.tmp-persistence-data"
BASE="http://127.0.0.1:$PORT"

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  rm -rf "$DATA_DIR"
}
trap cleanup EXIT

rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"

docker build -t "$IMAGE" .
docker run -d --name "$NAME" \
  -p "$PORT:8080" \
  -e PUID="$(id -u)" -e PGID="$(id -g)" \
  -v "$DATA_DIR:/data" \
  "$IMAGE" >/dev/null

for _ in $(seq 1 60); do
  if curl -fsS "$BASE/api/v1/health" >/dev/null; then break; fi
  sleep 1
done
curl -fsS "$BASE/api/v1/health" >/dev/null

# Create persistent user content through the public dashboard API.
curl -fsS -X POST -H 'Content-Type: application/json' \
  -d '{"name":"Multimedia QA","slug":"multimedia-qa"}' "$BASE/api/v1/pages" >/dev/null
curl -fsS -X POST -H 'Content-Type: application/json' \
  -d '{"name":"Red QA","slug":"red-qa"}' "$BASE/api/v1/pages" >/dev/null

PAGES_JSON="$(curl -fsS "$BASE/api/v1/pages")"
MULTIMEDIA_ID="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(next(x["id"] for x in d["pages"] if x["slug"]=="multimedia-qa"))' <<<"$PAGES_JSON")"
RED_ID="$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(next(x["id"] for x in d["pages"] if x["slug"]=="red-qa"))' <<<"$PAGES_JSON")"

curl -fsS -X POST -H 'Content-Type: application/json' \
  -d "{\"page_id\":$MULTIMEDIA_ID,\"type\":\"system.cpu\",\"title\":\"CPU QA\"}" "$BASE/api/v1/widgets" >/dev/null
curl -fsS -X POST -H 'Content-Type: application/json' \
  -d "{\"page_id\":$RED_ID,\"type\":\"network.internet\",\"title\":\"Internet QA\"}" "$BASE/api/v1/widgets" >/dev/null
APP_JSON="$(curl -fsS -X POST -H 'Content-Type: application/json' \
  -d '{"name":"Plex QA","url":"http://plex.example.test:32400","description":"Aplicación persistente de prueba"}' "$BASE/api/v1/applications")"
APP_ID="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <<<"$APP_JSON")"
curl -fsS -X POST -H 'Content-Type: application/json' \
  -d "{\"page_id\":$MULTIMEDIA_ID,\"type\":\"application.shortcut\",\"title\":\"Plex QA\",\"application_id\":$APP_ID}" "$BASE/api/v1/widgets" >/dev/null
curl -fsS -X PUT -H 'Content-Type: application/json' \
  -d '{"name":"SIDINET Persistence QA"}' "$BASE/api/v1/settings/dashboard" >/dev/null

# Stop cleanly, record the logical persistent state, remove the container only.
docker stop "$NAME" >/dev/null
snapshot() {
  sqlite3 "$DATA_DIR/database.sqlite" <<'SQL'
.mode list
.separator |
SELECT id,name,slug,position,enabled,is_default,background_mode,revision FROM pages ORDER BY id;
SELECT id,page_id,widget_type,provider_type,title,subtitle,enabled,refresh_mode,refresh_interval,visibility,style_override,config,COALESCE(parent_widget_id,0),revision FROM widgets ORDER BY id;
SELECT widget_id,breakpoint,x,y,width,height,COALESCE(min_width,0),COALESCE(min_height,0),COALESCE(max_width,0),COALESCE(max_height,0) FROM widget_layouts ORDER BY widget_id,breakpoint;
SELECT id,name,description,url,icon_type,COALESCE(icon_value,''),source_type,COALESCE(source_id,''),open_mode,enabled,COALESCE(monitor_id,0) FROM applications ORDER BY id;
SELECT key,value,type FROM settings ORDER BY key;
SQL
}
snapshot > "$DATA_DIR/before.txt"
docker rm "$NAME" >/dev/null

# Recreate a fresh container with the exact same /data bind mount.
docker run -d --name "$NAME" \
  -p "$PORT:8080" \
  -e PUID="$(id -u)" -e PGID="$(id -g)" \
  -v "$DATA_DIR:/data" \
  "$IMAGE" >/dev/null
for _ in $(seq 1 60); do
  if curl -fsS "$BASE/api/v1/health" >/dev/null; then break; fi
  sleep 1
done
curl -fsS "$BASE/api/v1/health" >/dev/null
curl -fsS "$BASE/api/v1/pages" | grep -q '"slug":"multimedia-qa"'
curl -fsS "$BASE/api/v1/pages" | grep -q '"slug":"red-qa"'
curl -fsS "$BASE/api/v1/applications" | grep -q '"name":"Plex QA"'

docker stop "$NAME" >/dev/null
snapshot > "$DATA_DIR/after.txt"

diff -u "$DATA_DIR/before.txt" "$DATA_DIR/after.txt"
test -f "$DATA_DIR/database.sqlite"
test "$(find "$DATA_DIR/backups" -maxdepth 1 -name 'sidinet-backup-*.zip' -type f | wc -l)" -ge 1

echo "PASS: pages, widgets, layouts, applications and settings are identical after container recreation with persistent /data."
