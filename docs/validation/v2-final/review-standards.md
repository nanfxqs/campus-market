Standards review of `955814f7caeb45ae385d59a5db6aad410d80de5e...4cfc8d551741c31af44db4f9acc090abfad74552`.

No hard documented-standard violations found. Reviewed AGENTS.md, RTK.md, the domain/issue/triage guidance, GLOSSARY.md, ADRs 0001–0005, README.md, assignment/development-plan documents, commit list, and changed application/deployment/experiment code. No CODING_STANDARDS.md or CONTRIBUTING.md exists. Vocabulary, retained archives, TTL eligibility, MongoDB transaction authority, and local Search follow the documented decisions.

One heuristic finding:

- **Possible Duplicated Code** — `scripts/run-sale-load.py:82–93` and `scripts/run-detail-load.py:46–57`. Baseline: “the same logic shape appears in more than one hunk or file in the change.” Both implement the full V2 Search-ingestion readiness gate: `deadline=time.monotonic()+180`, query `/search?q=%E5%8D%95%E8%BD%A6`, require `total==16667`, persist each observation, retry once per second. The copies already differ in deadline comparison (`>=` versus `>`), exception type, and observation field names (`UTC/expectedTotal/actualTotal` versus `unixNs/expected/actual`). Changing the known full-cohort fixture or fixing readiness therefore requires coordinated edits and can leave the experiments using different preparation rules. Extract a small shared full-cohort readiness helper used by both runners, keeping preparation outside timed windows and retaining raw success/error observations. Keep any differences explicitly required by the callers; this is a judgment call, not a documented-standard breach. Source hashes should record the shared helper for future provenance; historical raw results should remain unchanged.

Totals: 0 hard violations; 1 heuristic finding. No other actionable baseline smells identified.
