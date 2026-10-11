#!/usr/bin/env bash
# Deployed A/B/C regression with original full V1 background, isolated from measured pools.
set -euo pipefail
cd "$(dirname "$0")/.."
[[ $# == 1 ]] || { echo 'usage: scripts/validate-v2-regression.sh NEW_OUTPUT_DIRECTORY' >&2; exit 2; }
mkdir "$1"
output=$(realpath "$1")
project="campus-v2-regression-$(date -u +%Y%m%d%H%M%S)-$$"
export DB_NAME=campus_v2_regression API_PORT=8098 GIN_MODE=release
all=(docker compose -p "$project" -f compose.v2.yaml --profile A --profile B --profile C --profile tools)
[[ -z $(docker ps -aq --filter "label=com.docker.compose.project=$project") && -z $(docker volume ls -q --filter "label=com.docker.compose.project=$project") ]] || exit 2
cleanup() {
 local status=$?
 trap - EXIT
 set +e
 "${all[@]}" logs --no-color --timestamps | gzip > "$output/services.log.gz"
 "${all[@]}" down --volumes --remove-orphans > "$output/cleanup.log" 2>&1
 local clean=$?
 [[ -z $(docker ps -aq --filter "label=com.docker.compose.project=$project") && -z $(docker volume ls -q --filter "label=com.docker.compose.project=$project") ]] || clean=1
 ((clean==0)) || status=1
 printf '{"exitCode":%d,"cleanupExitCode":%d,"project":"%s"}\n' "$status" "$clean" "$project" > "$output/result.json"
 exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
exec > >(tee "$output/orchestration.log") 2>&1
 git rev-parse HEAD > "$output/git-head.txt"
 git status --short > "$output/git-status.txt"
 sha256sum internal/market/*.go compose.v2.yaml deploy/nginx-v2.conf > "$output/source-hashes.txt"
 uname -a > "$output/machine.txt"
 docker version > "$output/docker-version.txt"
 "${all[@]}" config > "$output/compose.yaml"
 "${all[@]}" build api-a api-b api-c1 api-c2 tools
 "${all[@]}" up -d --wait mongo redis
 python3 scripts/wait-v2-mongo.py --project "$project" --timeout 120 > "$output/readiness.log" 2>&1
for configuration in A B C; do
 mkdir "$output/$configuration"
 "${all[@]}" stop api-a api-b api-c1 api-c2 nginx
 "${all[@]}" run --rm --no-deps api-a seed --mode full --reset --random-seed 42 --base-time 2026-10-11T00:00:00Z > "$output/$configuration/seed.log" 2>&1
 "${all[@]}" exec -T redis redis-cli FLUSHDB > "$output/$configuration/cache-reset.txt"
 "${all[@]}" exec -T mongo mongosh --quiet "$DB_NAME" --eval "$(cat scripts/check-seed.js)" > "$output/$configuration/initial-counts.txt"
 case "$configuration" in A) apps=(api-a); target=api-a;; B) apps=(api-b); target=api-b;; C) apps=(api-c1 api-c2 nginx); target=nginx;; esac
 "${all[@]}" up -d --wait "${apps[@]}"
 "${all[@]}" run --rm --no-deps -e V1_API_URL="http://$target:8080" -e V1_DB_NAME="$DB_NAME" tools test -v ./internal/market -run '^TestV1FullSeedHTTP$' -count=1 -timeout=8m > "$output/$configuration/http.log" 2>&1
 echo "deployed $configuration regression passed"
 done
 # Test-created databases exercise cache edit races and exact expiry/rollback boundaries.
 "${all[@]}" run --rm --no-deps tools test -v ./internal/market -run 'TestCached|TestCache|TestDetail|TestExpiry|TestSale' -count=1 -timeout=8m > "$output/boundary-tests.log" 2>&1
 docker inspect $("${all[@]}" ps -q) > "$output/containers.json"
