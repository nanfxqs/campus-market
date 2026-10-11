# V2 resource/storage recorder (#17)

The public entry is `scripts/measure-v2.py`. It only observes containers carrying the exact Compose project label; it never resets data or manages service lifecycle. Python standard library, Linux `/proc`, readable cgroup v2, and Docker access are required. Measurements made without permissions retain errors and null values instead of zero. Output directories must be new so evidence cannot be overwritten.

After import/read-only verification, record an idle phase:

```sh
python3 scripts/measure-v2.py --project campus-v2-example --database campus_v2_experiment --phase idle --seconds 30 --output artifacts/A-round1/idle --git-base integration/v2 --locust experiments/.venv/bin/locust
```

For sale or detail load, start the client at its external start gate, supply its PID, and launch the same recorder without `--seconds`. Wait until `ready.json` exists before opening the gate. Record through the entire client window, then send SIGTERM to the PID in `ready.json`. SIGINT also finishes normally; both take the final storage snapshot and aggregate samples. Repeat `--client-pid` for multiple client roots; descendants are also observed. The recorder PID is distinct from client PIDs.

```sh
python3 scripts/measure-v2.py --project campus-v2-example --database campus_v2_experiment --phase sale-load --client-pid 12345 --output artifacts/A-round1/resources --git-base integration/v2 --locust experiments/.venv/bin/locust
python3 scripts/measure-v2.py --output artifacts/A-round1/resources --summarize --window artifacts/A-round1/window.json
```

Use exactly this method for A/B/C. `--window` accepts the sale runner's `startUnixNs`/`deadlineUnixNs`, writes `summary-window.json`, restricts averages/peaks to samples starting in that interval, and checks that outer sampling covers both boundaries. Raw samples are retained. A measured phase is not automatically a passed performance experiment.

## Observation and reproducibility

`commands.jsonl` preserves each argv, UTC start, duration, exit, stdout and stderr. `environment.json` records OS/CPU/memory/disks, Docker/Compose version and daemon budget, immutable image identities, running container budgets, Git HEAD/base and dirty status/diff summary, MongoDB/Search/Redis/Nginx versions, Locust version (or unavailable), client cmdline/limits/cgroups/affinity and Python. Application version is Git commit plus immutable image ID; the binary has no release version flag. Containers are discovered at start; restart the recorder for a changed deployment. Raw `containers.json` preserves full inspection.

`samples.jsonl` samples every second against an absolute monotonic schedule and includes start UTC/nanoseconds, actual monotonic timestamp, capture duration, host `/proc/stat`, memory and load average, container cgroup CPU/memory/IO, and actual host PID process user/system CPU plus RSS. CPU process/container percentages use 100%=one core; host percentage is all-core busy fraction. First process CPU samples have null deltas. Host memory is MemTotal minus MemAvailable. Shared resident pages may overlap across PIDs; process RSS must not be added to container memory. MongoDB `mongod` and Search Java `MongotCli` are separate PIDs; the shared Atlas container memory appears once, never assigned twice. Summaries retain per-instance service/PID and container metrics, component totals, sample counts and unknown/absent statuses. Missing means/peaks are null. A gap or capture time above one second must be reviewed rather than silently counted as exact 1 Hz.

`storage-before.json` and `storage-after.json` retain MongoDB logical `dataSize`, allocated collection/index storage statistics, and each mount's identity/destination/type, allocated bytes (`du -sk` ×1024), apparent file bytes (`du -sb`). Search's dedicated `/data/mongot` files are measured separately. Search logical-document byte size is explicitly unavailable rather than invented from file size. ConfigDB and proxy config bind mounts are distinct. Redis reports real INFO memory/persistence, save/appendonly/maxmemory/policy and persistent mount inventory. No persistent mount is stated explicitly; persistence flags determine whether its container writable layer can hold files. Redis image's anonymous `/data` volume, if present, is recorded as an actual volume rather than assumed absent. The recorded configuration disables persistence in these tests.

`collector-cost.json` reports collector and Docker CLI child CPU seconds plus RSS peaks. Sample capture wall time and collector RSS/CPU are included in summaries; command durations show storage/environment overhead outside the phase. Docker daemon and in-container exec costs remain visible in host/container totals and are not subtracted or falsely attributed as free collection. Several independent test stacks were active during this engineering validation, so host totals describe the machine rather than isolated application cost. Formal comparisons must run exclusively.

## Real validation and retained failures

- `idle`: five post-import idle samples on the full restored B dataset, separate MongoDB/Search, Redis and application, no Nginx; raw storage and environment retained. Initial Locust executable lookup was unavailable explicitly.
- `short-load`: actual #16 short2 Locust with 50 ready users, 10 seconds, 1,000 unique on-time new sales and no HTTP errors, external consistency check passed. Its complete load evidence is maintained by #16. Recorder covers both actual boundaries; `summary-window.json` contains exactly ten 1 Hz samples and actual client PID 660081. MongoDB 8.0.4 and Search 1.76.0 observed. Client command points at pinned Locust 2.46.6; global Locust lookup failed explicitly. This is not formal 30,000/600 acceptance.
- `C-validated` plus `C-validated-http.jsonl`: fresh isolated `campus-v2-measure17` project on port 8086, demo accounts, both Gin instances, MongoDB/Search, Redis and Nginx healthy. Six seconds of real detail reads produced 228 correct responses reaching both upstreams, while ten one-second samples measured each actual component, all mounted paths and Locust version. `tools/measurement/validate_live.py` reproduces this joint check against a caller-owned ready demo C stack; `verify.py` checks coverage, attribution, null absence, storage and summary evidence.
- `C-idle` was an exploratory incomplete deployment observation and is not a complete C validation. `C-load` retained the portable-command failure (`du -B1` unsupported in Alpine) and an exited client launch; `C-load-fixed` retained the failed client lifecycle observation. The final joint runner keeps its child client alive and uses portable `du -sk`. These failed attempts are not selected as passing evidence.

The dedicated measurement project was removed with all A/B/C profiles and its own volumes after validation. No other project was reset or cleaned. Initial artifact-seam red test failed because measurement output did not exist; C artifact verification then failed on unknown proxy mount allocation, and passed after the portable `du` correction. The seam checks use actual CLI output, HTTP responses and external storage evidence rather than mocked process metrics.
