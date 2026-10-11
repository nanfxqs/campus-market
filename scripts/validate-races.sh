#!/usr/bin/env bash
# Own and destroy fresh independent competition fixtures; never use the formal pool.
set -euo pipefail
cd "$(dirname "$0")/.."
output=""
port=8087
configuration=C
groups=100
functional=false
while (($#)); do
  case "$1" in
    --output) output="${2:?--output needs a new directory}"; shift 2 ;;
    --configuration) configuration="${2:?configuration required}"; shift 2 ;;
    --groups) groups="${2:?groups required}"; shift 2 ;;
    --functional) functional=true; shift ;;
    --port) port="${2:?--port needs a host port}"; shift 2 ;;
    --help) echo "Usage: scripts/validate-races.sh --output NEW_DIRECTORY --configuration A|B|C [--port 8087] [--groups 2 --functional]"; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done
[[ -n "$output" ]] || { echo "--output is required" >&2; exit 2; }
[[ "$port" =~ ^[0-9]+$ ]] && ((port>1024 && port<65536)) || { echo "Invalid port" >&2; exit 2; }
[[ "$configuration" =~ ^[ABC]$ && "$groups" =~ ^[0-9]+$ ]] && ((groups>0 && groups<=100)) || exit 2
$functional || [[ "$groups" == 100 ]] || { echo "formal competition requires exactly 100 groups" >&2; exit 2; }
# Never overwrite evidence from a prior failure or successful run.
mkdir "$output"
output="$(realpath "$output")"
project="campus-v2-race-$(date -u +%Y%m%d%H%M%S)-$$"
export API_PORT="$port"
export DB_NAME=campus_v2_race
export GIN_MODE=release
if [[ -n "$(docker ps -aq --filter "label=com.docker.compose.project=$project")" ]] || [[ -n "$(docker volume ls -q --filter "label=com.docker.compose.project=$project")" ]]; then
  echo "Refusing existing project $project" >&2; exit 2
fi
compose=(docker compose -p "$project" -f compose.v2.yaml --profile "$configuration")
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
sha256sum scripts/race-experiment.py scripts/validate-races.sh scripts/wait-v2-mongo.py > "$output/script-hashes.txt"
git rev-parse HEAD > "$output/git-head.txt"
git status --short > "$output/git-status.txt"
docker version > "$output/docker-version.txt"
docker compose version > "$output/compose-version.txt"
"${compose[@]}" config > "$output/compose-resolved.yaml"
case "$configuration" in A) apps=(api-a);; B) apps=(api-b);; C) apps=(api-c1 api-c2 nginx);; esac
"${compose[@]}" build "${apps[@]}"
"${compose[@]}" up -d --wait mongo redis
python3 scripts/wait-v2-mongo.py --project "$project" --timeout 120 > "$output/search-readiness.log" 2>&1
carrier=${apps[0]}
"${compose[@]}" run --rm --no-deps "$carrier" seed --mode demo > "$output/seed.log" 2>&1
"${compose[@]}" up -d --wait "${apps[@]}"
args=(--project "$project" --url "http://127.0.0.1:$port" --configuration "$configuration" --output "$output" --groups "$groups")
$functional && args+=(--functional)
python3 scripts/race-experiment.py "${args[@]}" > "$output/group-results.jsonl" 2>&1
ids=( $("${compose[@]}" ps -q) )
docker inspect "${ids[@]}" > "$output/containers.json"
echo "Race experiment passed; raw evidence: $output"
