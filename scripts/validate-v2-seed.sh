#!/usr/bin/env bash
# Real CLI/Compose + read-only MongoDB evidence. Keep raw output, including failures.
set -euo pipefail
cd "$(dirname "$0")/.."
project=${SEED_PROJECT:-campus-v2-seed15}
output=${SEED_OUTPUT:-docs/validation/v2/seed}
base=${BASE_TIME:?explicit BASE_TIME required}
mkdir -p "$output"
output=$(realpath "$output")
dc=(docker compose -p "$project" -f compose.v1.yaml)
"${dc[@]}" build market
"${dc[@]}" up -d mongo --wait
"${dc[@]}" stop market
"${dc[@]}" run --rm --no-deps -e DB_NAME=campus_v2_seed15 -v "$output:/evidence" market seed --mode v2 --base-time "$base" --manifest /evidence/manifest.json --reset
"${dc[@]}" exec -T mongo mongosh --quiet campus_v2_seed15 --eval 'if(db.products.countDocuments({}) !== 230000 || db.users.countDocuments({}) !== 10000 || db.products.countDocuments({seedCohort:"dedicated",sold:false}) !== 30000 || db.transactions.countDocuments({}) !== 300000) throw new Error("full V2 background missing"); print("full V2 counts passed");'
python3 scripts/check-v2-manifest.py "$output/manifest.json"
"${dc[@]}" exec -T mongo mongosh --quiet campus_v2_seed15 --eval "$(cat scripts/check-seed.js)"
"${dc[@]}" exec -T mongo mongosh --quiet campus_v2_seed15 --eval "$(cat scripts/fingerprint-seed.js)" > "$output/fingerprints-first.txt"
"${dc[@]}" cp "$output/manifest.json" mongo:/tmp/v2-manifest.json
"${dc[@]}" exec -T mongo mongosh --quiet campus_v2_seed15 --eval "const manifestJSON=require('fs').readFileSync('/tmp/v2-manifest.json','utf8'); $(cat scripts/check-v2-manifest.js)"
cp "$output/manifest.json" "$output/manifest-first.json"
# Nonempty refusal and invalid pre-reset validation must preserve initial data.
if "${dc[@]}" run --rm --no-deps -e DB_NAME=campus_v2_seed15 market seed --mode v2 --base-time "$base"; then echo 'nonempty import unexpectedly succeeded'; exit 1; fi
if "${dc[@]}" run --rm --no-deps -e DB_NAME=campus_v2_seed15 market seed --mode v2 --base-time 0001-01-01T00:00:00Z --reset; then echo 'invalid reset unexpectedly succeeded'; exit 1; fi
"${dc[@]}" exec -T mongo mongosh --quiet campus_v2_seed15_other --eval 'db.sentinel.updateOne({_id:"keep"},{$set:{value:42}},{upsert:true})'
start=$(date +%s)
"${dc[@]}" run --rm --no-deps -e DB_NAME=campus_v2_seed15 -v "$output:/evidence" market seed --mode v2 --base-time "$base" --manifest /evidence/manifest.json --reset
"${dc[@]}" exec -T mongo mongosh --quiet campus_v2_seed15 --eval "$(cat scripts/check-seed.js)"
"${dc[@]}" exec -T mongo mongosh --quiet campus_v2_seed15_other --eval 'if(db.sentinel.findOne({_id:"keep"}).value!==42) throw new Error("reset affected other database"); print("other database retained")'
"${dc[@]}" exec -T mongo mongosh --quiet campus_v2_seed15 --eval "$(cat scripts/fingerprint-seed.js)" > "$output/fingerprints-restored.txt"
cmp "$output/fingerprints-first.txt" "$output/fingerprints-restored.txt"
cmp "$output/manifest-first.json" "$output/manifest.json"
echo "Repeated restore manifest identical; import+verification preparation seconds: $(($(date +%s)-start))"
