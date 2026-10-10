#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ $# -gt 1 ]]; then
  echo 'usage: scripts/validate-expiry.sh [markdown-result-path]' >&2
  exit 2
fi
result=${1:-docs/validation/expiry-run.md}
mkdir -p "$(dirname "$result")"
# Tests create and remove their own databases. No existing dataset is reset.
docker compose up -d mongo --wait
docker compose build verify
{
  echo '# 自动下架验收实测'
  echo
  echo "UTC: $(date -u +%FT%TZ)"
  echo
  echo '真实 HTTP + MongoDB/Search；覆盖所有入口到期边界、并发成交与跨到期重试、TTL 物理清理及档案/图片/历史保留。'
  echo 'TTL 采用 3 分钟有界观察；这不是业务清理时限。测试库自动清理。'
  echo
  echo '```text'
} > "$result"
set +e
docker compose run --rm --no-deps -e GIN_MODE=release verify test -v ./internal/market \
  -run 'TestExpiry|TestSaleExactExpirationHTTP|TestSaleTransactionRetryAcrossExpirationHTTP|TestSaleConcurrentExactlyOnce' \
  -count=1 -timeout=8m 2>&1 | sed 's/[[:blank:]]*$//' | tee -a "$result"
status=${PIPESTATUS[0]}
set -e
{
  echo '```'
  echo
  if [[ $status -eq 0 ]]; then echo '结果：PASS'; else echo "结果：FAIL (exit $status)，见上述诊断。"; fi
} >> "$result"
exit "$status"
