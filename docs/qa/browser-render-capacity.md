# Browser export and finite server capacity

The acceptance test source is commit `669c804b`. These are deterministic store
and service checks against SQLite, not sustained production-host load results.
They assert identities, counts and terminal state without ordering wall-clock
timestamps. The default native capacity is read from `DefaultRenderCapacity`:
one active execution, two waiting admissions and one admission per account.

| Required property | Pinning test | Scope and remaining limits |
| --- | --- | --- |
| Finite active/waiting defaults and valid overrides | `TestNativeCapacityDefaultsAndFiniteOverrides` (`internal/clip`) | Rejects invalid/unbounded settings. |
| Atomic total and per-account admission | `TestNativeCapacityConcurrentAdmissionDuplicateAndAccountBounds` | Two API database handles race admissions; duplicate starts preserve identity and lower limits preserve accepted work. |
| Native active execution bound | `TestNativeClaimLimitIncludesCancelledLeasesButExcludesAnalysis` | A cancelled, unexpired worker lease still owns execution capacity; analysis is separate. |
| Overload reserves neither an export count nor source consumption | `TestBrowserRenderStartsWhileServerCapacityIsSaturated` | The public server start refuses at the default total bound; the account's window remains unused and unreserved, with sources ready. |
| Browser start is independent of occupied native slots | `TestBrowserRenderStartsWhileServerCapacityIsSaturated` | Every default native admission is occupied by other accounts. `StartBrowserRender` durably admits a browser render and its distinct sampling job, without another native job or export reservation. This proves admission, not completion latency for legacy remote sampling. |
| Cancellation releases admission and the browser remains unaffected | `TestBrowserRenderStartsWhileServerCapacityIsSaturated` | Queue cancellation makes one queued server job terminal, releases its reservation, and admits the refused account; the browser record is unchanged. |
| Cancellation/publication has one winner | `TestRemoteRenderCancellationAndPublicationHaveOneWinner` | Races conditional publication and cancellation; only the winning terminal result can own the canonical artifact. |
| Failed admission rolls back; export reservation remains bound to its origin window | `TestNativeAdmissionRollbackAndOriginWindowRecovery` | Conflicting plan revision consumes nothing; restart/idempotency preserves the accepted identity and finite deadline. |
| Queue expiry releases capacity and preserves the old result | `TestNativeQueueExpiryBeforeDispatchPreservesPreviousResult` | Advances an injected clock beyond the fixed deadline; repeated reconciliation does not create work or invoke AI. |
| API restart preserves already admitted native work | `TestNativeRenderRestartBeforeFirstParkPreservesAdmittedWork` | Queued, leased and accepted stages retain identity, digest, deadline and attempt count; no replay of completed media/AI work. |
| Recovery skips completed analysis/plan work | `TestRecoveryRenderRestartSkipsCompletedWork` | Recreates the service, resumes the durable candidate without repeating completed work or paid calls. |
| Same-host API CPU/memory and worker memory configuration | `MediaComposeTest.test_colocated_remote_and_standalone_are_private_cpu_stacks` (`deploy/media_compose_test.py`) | Pins API 1 CPU/256 MiB and worker 512 MiB memory/swap over rendered Compose layouts. Static configuration proof, not measured contention. |
| Same-host worker CPU limit assertion | **Open** | Compose config sets worker CPU, but the named test does not assert its value. The deployment/release gates exercise explicit resource budgets separately; they do not supply this missing Compose assertion. |
| Sustained load, API headroom and hosting capacity | **Open** | Live/rented-host measurements remain outside this task under INFRA-15/INFRA-17; no hardware or GPU qualification is inferred. |

Run from the repository root with the toolchain and isolated Python environment
in [the verification runbook](../verification.md). The Go regression selection
and Compose test below were run against source commit `669c804b`; task results
record the gate log and final publication revision.

```sh
go -C backend test -timeout 30m -run 'TestNativeCapacity|TestNativeAdmission|TestNativeClaimLimit|TestNativeQueueExpiry|TestBrowserRenderStartsWhileServerCapacityIsSaturated|TestNativeRenderRestartBeforeFirstPark|TestRemoteRenderCancellation|TestRecoveryRenderRestart' ./internal/clip/store/
go -C backend test ./internal/clip/
go -C backend vet ./internal/clip/...
go -C backend build ./internal/clip/...
test -z "$(cd backend && gofmt -l ./internal/clip)"
/tmp/postpilot-deploy-checks/bin/python -m unittest discover -s deploy -p 'media_compose_test.py' -v
```

The same-host separate-process release checks in the runbook measure fixture
execution within their stated API/worker budgets. They complement the tests
above; neither a local pass nor ARM-host x64 emulation establishes real deployment
success or native-runner performance. Inspect the workflows for each pushed SHA.
