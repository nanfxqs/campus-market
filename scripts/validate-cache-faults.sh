#!/usr/bin/env bash
# Own and destroy a fresh project; do not fault a normal experiment deployment.
set -euo pipefail
cd "$(dirname "$0")/.."
output=""
port=8094
while (($#)); do
  case "$1" in
    --output) output="${2:?--output needs a new directory}"; shift 2 ;;
    --port) port="${2:?--port needs a host port}"; shift 2 ;;
    --help) echo "Usage: scripts/validate-cache-faults.sh --output NEW_DIRECTORY [--port 8094]"; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done
[[ -n "$output" ]] || { echo "--output is required" >&2; exit 2; }
[[ "$port" =~ ^[0-9]+$ ]] && ((port>1024 && port<65536)) || { echo "Invalid port" >&2; exit 2; }
# Never overwrite evidence from a prior failure or successful run.
mkdir "$output"
output="$(realpath "$output")"
project="campus-v2-fault-$(date -u +%Y%m%d%H%M%S)-$$"
export API_PORT="$port"
export DB_NAME=campus_v2_fault
export GIN_MODE=release
if [[ -n "$(docker ps -aq --filter "label=com.docker.compose.project=$project")" ]] || [[ -n "$(docker volume ls -q --filter "label=com.docker.compose.project=$project")" ]]; then
  echo "Refusing existing project $project" >&2; exit 2
fi
# Explicitly short MongoDB selection/connect timeouts make the original 503
# contract observable before HTTP's write timeout when the database is stopped.
cat > "$output/override.yaml" <<'YAML'
services:
  api-b:
    environment:
      MONGO_URI: mongodb://mongo:27017/?directConnection=true&serverSelectionTimeoutMS=1000&connectTimeoutMS=1000
      DB_NAME: campus_v2_fault
      CACHE_TIMEOUT: 100ms
      GIN_MODE: release
YAML
compose=(docker compose -p "$project" -f compose.v2.yaml -f "$output/override.yaml" --profile B)
exec > >(tee "$output/orchestration.log") 2>&1
cleanup() {
  local result=$?
  trap - EXIT INT TERM
  set +e
  "${compose[@]}" logs --no-color --timestamps > "$output/services.log" 2>&1
  "${compose[@]}" ps -a > "$output/final-services.txt" 2>&1
  "${compose[@]}" down --volumes --remove-orphans > "$output/cleanup.log" 2>&1
  local clean_status=$?
  if ((clean_status!=0)); then result=1; fi
  if [[ -n "$(docker ps -aq --filter "label=com.docker.compose.project=$project")" ]] || [[ -n "$(docker volume ls -q --filter "label=com.docker.compose.project=$project")" ]]; then result=1; fi
  printf '{"exitCode":%d,"cleanupExitCode":%d,"project":"%s","endedAt":"%s"}\n' "$result" "$clean_status" "$project" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$output/result.json"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
printf '{"project":"%s","database":"%s","port":%d,"startedAt":"%s"}\n' "$project" "$DB_NAME" "$port" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$output/context.json"
sha256sum scripts/validate-cache-faults.py scripts/validate-cache-faults.sh scripts/wait-v2-mongo.py > "$output/script-hashes.txt"
git rev-parse HEAD > "$output/git-head.txt"
git status --short > "$output/git-status.txt"
docker version > "$output/docker-version.txt"
docker compose version > "$output/compose-version.txt"
"${compose[@]}" config > "$output/compose-resolved.yaml"
"${compose[@]}" build api-b
"${compose[@]}" up -d --wait mongo redis
python3 scripts/wait-v2-mongo.py --project "$project" --timeout 120 > "$output/search-readiness.log" 2>&1
"${compose[@]}" run --rm -e MONGO_URI=mongodb://mongo:27017/?directConnection=true api-b seed --mode demo > "$output/seed.log" 2>&1
"${compose[@]}" up -d --wait api-b
python3 scripts/validate-cache-faults.py --project "$project" --url "http://127.0.0.1:$port" --compose-override "$output/override.yaml" --mongo-failure > "$output/faults.jsonl" 2>&1
# Actual runtime resource limits, image IDs and container ownership are evidence.
ids=( $("${compose[@]}" ps -q) )
docker inspect "${ids[@]}" > "$output/containers.json"
echo "Fault experiment passed; raw evidence: $output"
