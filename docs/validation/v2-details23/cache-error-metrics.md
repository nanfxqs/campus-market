# Per-request Redis error counts

Enabled B/C detail responses now include `X-Detail-Cache-Errors`. It counts Redis GET/decode and SET failures for that request, while keeping successful MongoDB fallback/business results intact. Disabled A retains its previous response headers and the runner records zero Redis errors for its disabled cache. Existing cumulative process counters remain unchanged.

At the confirmed real HTTP seam, `tools/details/verify_cache_errors.py` first failed against the old deployment: a forced SET OOM still returned HTTP200 and `miss`, but had no error count. After the narrow instrumentation change, actual Redis OOM (`maxmemory=1`, `noeviction`) returned the correct detail, HTTP200/miss/count1. Stopping the actual Redis container returned the same current detail with HTTP200/fallback/count1. Redis was restarted and resource policy restored; successful current reads returned count0. [Red](cache-errors-red.log) and [green](cache-errors-green.log) preserve HTTP and real Docker/configuration commands. The disposable demo B fixture uses project `campus-v2-detail-smoke23`, DB `campus_v2_detail_smoke`, HTTP port8087.

No response JSON, transaction semantics or persistent facts changed. These are auxiliary request metrics, not a cache fault endpoint or estimates derived from cumulative logs.
