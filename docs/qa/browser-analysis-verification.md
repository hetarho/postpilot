# Browser analysis copy verification

T602 binds immutable owner/project/source/quote/revision-bound copy slots and
verifies actual bytes, SHA-256, packet EOF, decoded frame/audio bounds, geometry
and declared coverage before AI handoff. Original measurements retain explicit
`browser_client` provenance. The `clip-browser-analysis-v1` semantic qualification
flag remains false; safe copies do not prove original content equivalence.

## Restart and paid-work boundaries

API boot calls both browser-analysis and native-media ReconcileStartup before
SweepRunning. Browser recovery drains successive bounded 100-row pages and
reconstructs only authorized unconsumed preparing/verifying/accepted waits.
Accepted work wakes through the ordinary claimed continuation. Consumed or
already claimed work retains FailOnInterrupt and cannot be replayed. Periodic
recovery retains its bounded rotating scan and never creates a missing wait for
an initial handler still validating accepted copies.

Real SQLite tests cover all three pre-Park states, a valid parent beyond 100
expired sessions, source/revision/quote/batch/session fences, consumed/claimed
negative controls and the live accepted-handler race. The earlier periodic
correction was disproved by a temporary queue reproduction and replaced by the
startup-only guard. Those failing diagnostic logs remain in the external handoff;
they are not passing qualification evidence.

## Exact CPU execution evidence

Final combined source is `7db220c746e2a608e19846bc888d6b295eba7ed4`, backend tree
`fa396e4a79fc78d28bf807fca14a02b256d5066c`. Its actual Linux arm64 nonroot CPU-v3
image is `sha256:2c7d70427c12d7f03b0ba4075fd3b8e13382e09418a7cc0bdb9b49758c93819a`.
The final image passed focused AnalysisCopyVerificationSmoke, WorkerCommandHealth,
WorkerImageIsolation and ExecRunnerReapsDescendants with network disabled,
1GiB memory/swap and two CPUs. This is a focused run, not a full combined
narration/native-render matrix. Its exact receipt is stored beside this document.

The same final image separately passed analysis verification with network disabled,
256MiB memory, swap disabled and 0.5CPU:

```sh
docker run --rm --network none --memory 256m --memory-swap 256m --cpus 0.5 \
  --entrypoint /media.test \
  sha256:2c7d70427c12d7f03b0ba4075fd3b8e13382e09418a7cc0bdb9b49758c93819a \
  -test.run='^TestAnalysisCopyVerificationSmoke$' -test.v -test.timeout=10m
```

That gate covers silent/AAC copies, exact 60-second bounds, malformed MP4, and a
65-second packet timeline whose declared movie/track/media/edit durations were
patched to 60 seconds. The bounded run passed in 3.01s. Limits were enforced;
peak process usage was not measured.

The preceding CPU-v2 source `75c77fe145e98ad078e78177f77f0d4a382d52c2` passed the
complete worker entrypoint and the bounded analysis gate; its separate full
receipt is preserved. T595 independently passed its full native CPU-v3 image.
The590 API-only merge preserved all 2001 selected worker/media-test dependency,
build and asset inputs; the subsequent595 native transform/version changes are
explicitly recorded rather than claimed unchanged. An obsolete776 image run
was interrupted and is never treated as passing. No Desktop/dev restart occurred.

## Local consistency and coverage

All 101 backend packages have successful outcomes: 82 from the preserved initial
run and durable exit 0 for the remaining 19. The original 64327 aggregate exit was
lost after context refresh and its log stopped early; no whole-command exit 0 is
inferred. Merged boot/recovery/native/browser regressions separately passed with
durable exit 0, followed by combined CPU-v3 interface tests and vet/build/gofmt.
Focused recovery race tests also passed. Coverage and log hashes are preserved.

Frontend 418 files / 3424 tests, lint/format/FSD/style, build, dev 11, retirement,
skills, deploy 62 and spec lint 0 errors / 46 inherited warnings passed. The595
frontend merge is backed by its independent checks; the combined branch passed
TypeScript and 52 relevant interface tests without repeating unchanged suites.

Actual Docker proto/SQL generation on 5ac verified 218 inputs and 152 exact outputs.
All input and output hashes still match the final combined source. The generator
receipt and current-hash comparison distinguish execution from reuse evidence;
no extra generator run or image success was fabricated.

Transport retains the optional-profile-list compatibility for native peers.
Verifier readiness requires its exact explicit profile/renderer/assets, with
independent bounded activity counters. Upload replay, overwrite, stale ownership,
missing coverage, cancellation/late reports, lease reclaim, restart settlement
and orphan cleanup preserve originals/previous results and authorize no premature
provider request or credit hold.

No paid provider call, live deployment or database-engine migration was performed.
Semantic information preservation and live/device qualification remain separate
T603/T604 gates.
