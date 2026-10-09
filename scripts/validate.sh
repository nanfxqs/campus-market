#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ $# -gt 1 || ( $# -eq 1 && $1 != --reset ) ]]; then
  echo 'usage: scripts/validate.sh [--reset]' >&2
  exit 2
fi
# --reset applies only to the dedicated fixture database.
docker compose up -d --build --wait
docker compose build verify
docker compose run --rm api seed "$@"
docker compose run --rm verify vet ./...
docker compose run --rm verify test -v ./validation -run "TestChineseSynonyms|TestSearchChecksCurrentEligibility" -count=1
docker compose run --rm verify test -v ./validation -run TestTransactionCommitAndRollback -count=1
docker compose run --rm verify test -v ./validation -run TestTTLRetainsArchiveAndForbidsSale -count=1
# The small fixture has now been verified; rebuild the disposable scale fixture.
docker compose run --rm api seed --large --reset
docker compose run --rm -e SCALE=1 verify test -v ./... -count=1 -timeout=12m
