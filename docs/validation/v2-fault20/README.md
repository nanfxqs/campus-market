# Independent Redis fault experiment (#20)

Run from the repository root, outside normal performance runs:

```sh
scripts/validate-cache-faults.sh --output /tmp/campus-fault-evidence-NEW --port 8094
```

The output directory must not already exist. The runner creates its own uniquely named `campus-v2-fault-*` Docker Compose B project and `campus_v2_fault` database, records resolved configuration and Git/source state, waits for real MongoDB and Search management readiness, seeds only demo accounts, and publishes its own sample. It does not reuse or reset A/B/C performance datasets. Existing resources with the generated project identity are rejected. Exit/signal traps retain logs and remove only this project's containers/network/volumes, including after setup or assertion failures. A cleanup failure or remaining owned resource fails the run. A subsequent performance run must use its own normal restore entry; fault samples are never normal performance samples.

The existing #19 Python fault entry is reused and extended. The runner covers:

- Real Redis eviction followed by reconstruction, with `evicted_keys` confirmation.
- A real Redis OOM SET refusal while content edits and detail reads still succeed.
- Redis `CLIENT PAUSE ALL 2000`: a detail request must fall back within a 1-second **test observation bound**, proving cache access is finite under a live but stalled server. The configured per-operation timeout remains 100ms.
- A paused old WRITE overlapping an HTTP edit and subsequent current-content read.
- Actual Redis stop; current detail, edit, price change, seller change, recent price history, sale, same-key replay and different-key conflict continue through MongoDB.
- Read-only MongoDB verification of one successful transaction, sold archive, removed eligibility and seller count exactly one. Redis does not arbitrate the sale or preserve idempotency.
- Actual Redis restart; the first read is a miss containing outage-edited content and the next is a hit containing current sold facts.
- Actual MongoDB stop with Redis running and warm: detail, health and authenticated profile return the existing exact HTTP `503 {"error":{"code":"unavailable"}}` response. Recovery returns current content again.

The fault-only Compose override uses 1-second MongoDB server-selection/connect timeouts so the original error response is observable before the existing HTTP write timeout. This changes the **experiment's connection budget**, not the HTTP error contract, and does not claim the unmodified default deployment has a 1-second outage SLA. The 3-second MongoDB failure observation bound is likewise a test bound. MongoDB/Search is the real fixed Atlas Local image; Redis is the real fixed image from `compose.v2.yaml`.

Raw artifacts retain orchestration/build output, readiness attempts, seed output, every HTTP response and observed cache/query/byte headers, Redis commands/results, external MongoDB facts, service logs including error/fallback counters, container identities/resource limits, final service state and cleanup/result status. Login tokens are redacted from HTTP artifacts. Redis snapshots and fault request latencies are not included in normal performance comparisons. The reusable `scripts/wait-v2-mongo.py --project campus-v2-... --timeout 120` helper checks container ownership and configured Search hosts, creates a nonce-specific probe database with an ownership token, requires three consecutive real Search-management successes, and drops only that owned probe database. It leaves the target seed database empty; both probe setup and cleanup outputs are preserved.

Earlier #19 concurrency/expiry tests remain in `../v2-cache19/`; this runner focuses on reproducible independent fault operation and recovery.

The implementation's four failed setup attempts are retained. `run-20261011` failed because Atlas Local's early internal health did not yet imply the advertised Mongo listener was ready. `run-20261011-02` connected before Search management was ready. `run-20261011-03` exposed a missing-namespace false positive: `$listSearchIndexes` can return an empty result without contacting Search. `run-20261011-04` exposed the initial Mongo bootstrap instance without configured Search hosts returning an empty result even for an existing namespace, then restarting before probe cleanup. All four failed runs cleaned their owned projects/volumes successfully. The final helper guards configured Search hosts and uses an owned existing namespace plus consecutive successes. Failures are retained preparation evidence, not passed business samples.

Final fresh-volume run `run-20261011-05` passed all HTTP, Redis and MongoDB assertions and cleanup (`exitCode=0`, `cleanupExitCode=0`). The deliberately stalled Redis detail fell back in 106.4ms. Warm-cache MongoDB outage detail/health/profile each returned exact 503 unavailable responses in about 1002ms under the fault-only 1s connection budget. Read-only persistence verification found one successful transaction, no listing, a sold product and seller count one despite same-key replay and a competing key. Recovery returned current content. These observed fault samples are not performance acceptance runs. Source hashes in that run match the delivered runner/helper source. `entry-guards.log` records refusal to overwrite evidence, invalid port refusal before resources are created, and rejection of a foreign project namespace.

After merging #15's seed/restore implementation, fresh-volume `run-20261011-06` repeated the full fault sequence successfully (`exitCode=0`, `cleanupExitCode=0`) and its source hashes also match the delivered scripts. Its configured Search-management probe initially failed, then passed three consecutive checks, dropped its owned probe database, and the previously empty target accepted the demo seed. This is the same helper invocation now added immediately after Mongo startup and before import in `scripts/restore-v2.sh`; a second full-size import was unnecessary to verify this preparation gate. The stalled Redis fallback in this repeat took 105.96ms. No fault project containers or volumes remain.
