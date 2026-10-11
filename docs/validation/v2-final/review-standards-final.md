Final Standards recheck: integration/v2 `3d4be5b98d266536b335758d3b4885acf89595ad`, fixes relative to `4cfc8d5`, within original review scope `955814f...HEAD`.

The prior possible Duplicated Code finding is resolved. Both sale/detail runners now call `scripts/search_readiness.py::wait_for_full_cohort`; the shared helper owns the expected full-cohort count, bounded polling and request budget. Caller-specific observation formats remain explicit for historical compatibility, and future invocation source hashes include the helper. These two currently needed formats and the tested HTTP CLI seam are not speculative abstractions.

Reviewed the helper/caller changes, expanded evidence binding and competition verification, focused public CLI/HTTP tests, report updates and fix evidence README. The verifier now associates raw observations with their experiment slots and independently checks coupled facts; these additions remain within its single evidence-validation responsibility. No new documented-standard breach or actionable baseline smell found. Existing domain vocabulary and ADR decisions remain satisfied; production application/deployment code is unchanged.

Read-only review; no workloads or Docker operations run. The delivered focused-test and stricter 28-stage matrix records were inspected as evidence, without treating passing tooling as a substitute for this review.

Totals: prior finding resolved; 0 remaining hard violations, 0 remaining actionable heuristics.
