# Browser analysis artifact authorization

The browser preparation profile is `clip-browser-analysis-v1`. Its semantic
qualification flag remains false until T603 records and reviews actual
information-preservation evidence. Copy safety does not certify content
equivalence with an original or a model's understanding of it.

Preparation has an owner/project/source/quote/revision-bound session. Original
measurements retain explicit `browser_client` provenance in both preparation
and recovery source records. A native continuation independently re-probes those
originals while retaining compatible accepted paid observations; a missing
provenance field keeps the existing native cache semantics. Native v3 media
task/info envelopes remain unchanged. Slots cover each missing
60-second interval and its exact final remainder, with the existing 720-pixel long edge,
15fps H.264/yuv420p and mono 48kHz/64kbit AAC target. Each immutable copy is at most
8MiB and each selection has at most 49 intervals. Existing compatible validated
observations retain their coverage and avoid preparing those intervals.

Begin returns the exact missing slots. The owner approves the current quote and
starts its durable AI parent immediately with `analysis_preparation_id`, before
encoding/upload. That parent parks on a browser-analysis wait without a credit
hold or provider call. The quote digest has a distinct browser binding, so
omitting/changing that reference or rolling back to an older native API cannot
silently switch its preparation route.

Reserve issues an exact-byte, owner-bound private conditional PUT. Repeated
identical reservations keep the same object identity; altered bytes/hash/slot
fail. Complete verifies reservation/HEAD coverage and starts its finite
verification queue/stage clocks once, within the page session's 2-hour expiry.
Polling, retry and restart never extend those clocks.

The verification role reads only submitted copies. It verifies full SHA-256 and
bytes, probes the exact stream/geometry/profile, scans packet timestamps through
EOF including raw edit-list tails, and performs one bounded structured decode
of every frame/audio sample with strict decoder errors and allocation/pixel/time
bounds. A 65-second MP4 whose declared durations were rewritten to 60 seconds is a
checked attack fixture. The receipt publishes only under the current unexpired
lease and live owner/source/revision/parent fences. Every interval must pass
before existing quote/reservation/model admission can proceed. The live browser
fence also runs before model-access endpoint checks and again atomically before
a credit hold or each model dispatch.

Cancellation first uses the existing durable parent cancellation, then fences
the page session. Lease loss stops publication; terminal/expired/abandoned
sessions preserve originals and previous results and queue recoverable copy
deletion beyond issued PUT expiry plus orphan grace. Bounded rotating object
listing catches late orphan uploads. Startup drains bounded recovery pages before
the generic interruption sweep and restores only unconsumed authorized missing
waits. Periodic recovery never requeues a live initial handler; accepted work
wakes through existing waits and claimed/consumed work never replays paid calls.

## Existing-host verification process

The existing native worker retains its `cpu` v3 profile and native operations.
Add `docker-compose.media.analysis.yml` explicitly beside the colocated CPU
Compose files after checking matching execution-image evidence and host capacity.
Status and health advertise each authenticated role’s actual profile and its own
waiting/global-active/own-active counts; a verifier cannot claim native identity.
Configure a distinct random identity/token in `analysis-worker.env` and the API's
`MEDIA_WORKER_CREDENTIALS`; set `MEDIA_WORKER_ROLES` to map that identity to
`analysis-verification`. Unlisted credentials remain native for compatibility.
The API rejects cross-role claims/artifact access.

Initial independent API admission is one verification execution, two waiting
preparations and one preparation per account. `CLIP_ANALYSIS_VERIFY_ACTIVE`,
`CLIP_ANALYSIS_VERIFY_WAITING` and `CLIP_ANALYSIS_VERIFY_PER_ACCOUNT` have finite
domain bounds. The verifier is one process with one decode thread, 0.5CPU/256MiB
and a 32MiB workspace; each copy is released sequentially. This adds resources to
the existing API/native budgets: the default colocated CPU sum is 2.5CPU before
OS/Caddy headroom, and verifier memory is an additional 256MiB. Recheck available
RAM, CPU/cgroup budgets and disk using DEPLOY.md; no host installation or live
rollout is performed by this task.

The image gate is `TestAnalysisCopyVerificationSmoke` in the CPU worker image.
`TestAnalysisCopyVerificationHostDiagnostic` uses explicitly supplied host
binaries and is separate evidence. A passing host run never substitutes for an
unexecuted image smoke. Browser semantic qualification remains an independent
T603 gate in either case.
