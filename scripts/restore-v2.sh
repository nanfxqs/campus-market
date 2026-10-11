#!/usr/bin/env bash
# Stop -> import -> clear dedicated cache -> read-only verification -> start.
# Failed preparation deliberately leaves all application services stopped.
set -euo pipefail
cd "$(dirname "$0")/.."
: "${COMPOSE_PROJECT_NAME:?explicit isolated COMPOSE_PROJECT_NAME required}"
: "${DB_NAME:?explicit experiment DB_NAME required}"
: "${BASE_TIME:?explicit fixed RFC3339 BASE_TIME required}"
: "${RESULT_DIR:?explicit RESULT_DIR required}"
configuration=${1:?usage: restore-v2.sh A|B|C [--reset]}
reset=${2:-}
[[ $configuration =~ ^[ABC]$ && ($reset == '' || $reset == --reset) && $# -le 2 ]] || exit 2
[[ $DB_NAME =~ ^campus_v2_[A-Za-z0-9_]+$ ]] || { echo 'DB_NAME must be a campus_v2_ experiment database' >&2; exit 2; }
[[ $COMPOSE_PROJECT_NAME =~ ^campus-v2-[a-z0-9-]+$ ]] || { echo 'project must be an explicit campus-v2- isolated project' >&2; exit 2; }
export DB_NAME COMPOSE_PROJECT_NAME
mkdir -p "$RESULT_DIR"
RESULT_DIR=$(realpath "$RESULT_DIR")
dc=(docker compose -f compose.v2.yaml --profile "$configuration")
start=$(date +%s)
# Profiles hide nonselected applications; enumerate all profiles to stop every
# instance that could continue using the same experiment database/cache.
all=(docker compose -f compose.v2.yaml --profile A --profile B --profile C)
mapfile -t apps < <("${all[@]}" config --services | awk '/^(api-|nginx)/')
((${#apps[@]})) || { echo 'no configured application services' >&2; exit 2; }
"${all[@]}" stop "${apps[@]}"
"${dc[@]}" up -d mongo --wait
"${all[@]}" build "${apps[@]}"
seed_args=(seed --mode v2 --random-seed "${RANDOM_SEED:-42}" --base-time "$BASE_TIME" --manifest /evidence/manifest.json)
[[ $reset == --reset ]] && seed_args+=(--reset)
# api-a is a seed CLI carrier even when B/C are selected, never a running server.
"${all[@]}" run --rm --no-deps -v "$RESULT_DIR:/evidence" api-a "${seed_args[@]}" > "$RESULT_DIR/import.txt" 2>&1
"${dc[@]}" up -d redis --wait
"${dc[@]}" exec -T redis redis-cli FLUSHDB > "$RESULT_DIR/cache-reset.txt"
"${dc[@]}" exec -T mongo mongosh --quiet "$DB_NAME" --eval "$(cat scripts/check-seed.js)" > "$RESULT_DIR/check-seed.txt"
python3 scripts/check-v2-manifest.py "$RESULT_DIR/manifest.json" > "$RESULT_DIR/check-manifest.txt"
"${dc[@]}" cp "$RESULT_DIR/manifest.json" mongo:/tmp/v2-manifest.json
"${dc[@]}" exec -T mongo mongosh --quiet "$DB_NAME" --eval "const manifestJSON=require('fs').readFileSync('/tmp/v2-manifest.json','utf8'); $(cat scripts/check-v2-manifest.js)" > "$RESULT_DIR/check-manifest-db.txt"
case $configuration in A) services=(api-a);; B) services=(api-b);; C) services=(api-c1 api-c2 nginx);; esac
"${dc[@]}" up -d --wait "${services[@]}" > "$RESULT_DIR/start.txt" 2>&1
end=$(date +%s)
python3 - "$RESULT_DIR" "$start" "$end" "$configuration" <<'PY'
import json,sys,os
result,start,end,configuration=sys.argv[1:]
json.dump({'configuration':configuration,'database':os.environ['DB_NAME'],'project':os.environ['COMPOSE_PROJECT_NAME'],'baseTime':os.environ['BASE_TIME'],'prepareSeconds':int(end)-int(start),'requiresFreshLogin':True},open(result+'/preparation.json','w'),indent=2)
PY
echo "Restored $configuration; preparation=$((end-start))s. Load must log in again and wait for Search readiness before timing."
