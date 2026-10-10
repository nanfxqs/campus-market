#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ $# -gt 1 ]]; then
  echo 'usage: scripts/validate-v1.sh [markdown-result-path]' >&2
  exit 2
fi
result=${1:-docs/validation/v1-run.md}
mkdir -p "$(dirname "$result")"
# Never attach to the developer stack or reuse its volumes.
project="campus-v1-$(date -u +%Y%m%d%H%M%S)-$$-$RANDOM"
base_time=$(date -u +%Y-%m-%dT00:00:00Z)
compose=(docker compose -p "$project" -f compose.v1.yaml)
finish() {
  status=$?
  trap - EXIT
  if [[ $status -ne 0 ]]; then
    "${compose[@]}" logs --no-color >> "$result" 2>&1 || true
  fi
  if [[ $owns_project == true ]]; then
    "${compose[@]}" down --volumes --remove-orphans >> "$result" 2>&1 || {
      echo 'Cleanup failed: remove only the recorded project manually.' >> "$result"
      status=1
    }
  fi
  printf '\n```\n\nResult: %s (exit %s)\n' "$([[ $status -eq 0 ]] && echo PASS || echo FAIL)" "$status" >> "$result"
  sed -i 's/[[:blank:]]*$//' "$result"
  exit "$status"
}
printf '# V1 clean-environment acceptance\n\nUTC: %s\n\nProject: `%s`; base time: `%s`; random seed: 42; mode: full.\n\n```text\n' "$(date -u +%FT%TZ)" "$project" "$base_time" > "$result"
owns_project=false
trap finish EXIT
run() {
  printf '\n$' | tee -a "$result"
  printf ' %q' "$@" | tee -a "$result"
  printf '\n' | tee -a "$result"
  "$@" 2>&1 | tee -a "$result"
}
run git rev-parse HEAD
run uname -a
run docker version
run docker compose version
run docker info --format 'CPUs={{.NCPU}} Memory={{.MemTotal}} OS={{.OperatingSystem}} Architecture={{.Architecture}}'
run df -h .
# Collision is an error, never a reason to erase another stack.
existing=$(docker volume ls -q --filter "label=com.docker.compose.project=$project")
[[ -z $existing ]] || { echo 'Project volumes already exist; refusing reuse.' >> "$result"; exit 1; }
owns_project=true
run "${compose[@]}" up -d --build --wait mongo
run "${compose[@]}" build market api verify
run "${compose[@]}" run --rm --no-deps market seed --mode full --random-seed 42 --base-time "$base_time"
# Verify the untouched initial dataset before HTTP mutations.
run "${compose[@]}" exec -T mongo mongosh --quiet campus_v1 --eval "$(cat scripts/check-seed.js)"
run "${compose[@]}" up -d --wait market api
run "${compose[@]}" run --rm --no-deps verify vet ./...
# Real deployed HTTP service; bounded polling waits for Search to converge.
run "${compose[@]}" run --rm --no-deps -e V1_API_URL=http://market:8080 -e V1_DB_NAME=campus_v1 verify test -v ./internal/market -run '^TestV1FullSeedHTTP$' -count=1 -timeout=8m
# The feasibility tests use their own separate large fixture database.
run "${compose[@]}" run --rm --no-deps api seed --large
# All invariant and lifecycle tests, including TTL persistence, once at the end.
run "${compose[@]}" run --rm --no-deps -e GIN_MODE=release -e SCALE=1 verify test -v ./... -count=1 -timeout=15m
run "${compose[@]}" ps
