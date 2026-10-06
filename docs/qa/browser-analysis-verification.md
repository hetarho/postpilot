# Browser analysis copy verification

T602 admits immutable owner/project/source/quote/revision-bound copy slots and
verifies their bytes, SHA-256, packet EOF, decoded frames/audio, geometry and
coverage before AI handoff. Browser original measurements remain explicit
`browser_client` claims in preparation and recovery. Native continuations
independently probe those originals and reuse compatible paid observations.

The browser profile remains `clip-browser-analysis-v1` with `qualified: false`.
Artifact safety does not prove original content equivalence or semantic quality;
T603 must supply that separate evidence.

## Exact CPU execution evidence

The tested source is `be200d828848cdb5e87a283f5fd4e7b8e8b1b9aa`, with backend tree
`fab5aca1eb2e8c92dd87d5602496dc734b687b96`. The actual Linux arm64 nonroot
`media-worker-smoke` image is
`sha256:48a8643238b80198672fb5d87f12c7d9980c02d1f7e778440976584c1f26f58a`.
The merged final planning parent retains exactly that backend tree; frontend
audio and task metadata changes therefore do not alter the tested CPU binaries.
Docker Engine 29.5.2 ran in an isolated Colima/Lima profile; the existing Docker
Desktop context and development containers were not changed.

The complete committed worker entrypoint passed with network disabled, 1GiB
memory/swap limit and two CPUs: analysis-copy verification, native byte/frame
parity across ratios and effects, narrated outputs, descendant cleanup, legacy
native health and image isolation. The analysis-copy gate also passed separately
with network disabled, 256MiB memory, swap disabled and 0.5CPU:

```sh
docker run --rm --network none --memory 256m --memory-swap 256m --cpus 0.5 \
  --entrypoint /media.test \
  sha256:48a8643238b80198672fb5d87f12c7d9980c02d1f7e778440976584c1f26f58a \
  -test.run='^TestAnalysisCopyVerificationSmoke$' -test.v -test.timeout=10m
```

That gate generates local silent/AAC copies, the exact 60-second bound, a
65-second packet timeline whose declared movie/track/media/edit durations were
patched to 60 seconds, and malformed MP4 data. The bounded run passed in 1.81s;
resource limits were enforced, and peak process usage was not measured.

The execution receipt is preserved beside this document. Full-image and bounded
analysis log SHA-256 values are respectively
`55996257a24de30dca4a8e9e1650371fd521d72032d9e1a8400803882dcc60fc`
and `6db2e270d2c181c3b7dd22a634ad44a769603775d0037fa3de2af6e9d9fa78b1`.

## Protocol, lifecycle and local checks

The transport retains native v3 readiness when an older peer omits its optional
profile list. Verifier readiness requires its explicit profile and exact
renderer/assets identity. Simultaneous native and verifier activity reports
independent waiting, active and own-active counters under authenticated roles.

Tests cover immutable conditional uploads, missing coverage, forged identities
and bounds, hidden timelines, stale/deleted/finalized source fences, no provider
access or credit hold before acceptance, delayed browser upload clocks, lease
reclaim, cancellation/late reports, restart settlement retries and orphan cleanup.
Accepted handoffs use the page/session TTL after verification ends; bounded
recovery scans rotate so live rows cannot starve later sessions.

Full backend tests/vet/build/gofmt, isolated frontend 408 files/3395 tests and
build/lint/FSD/style, dev tests, retirement, skill consistency, deploy 62 tests and
spec lint passed. Subsequent merged-parent and health-correction checks passed.
Actual Docker-based proto/SQL generation passed with 218 input hashes and 152
byte-identical outputs; source-only native generation independently passed with
pinned buf 1.73.0/sqlc 1.31.1 and exact committed EOF normalization. Generator
execution evidence remains distinct from CPU image execution evidence.

No paid provider request, live deployment, database-engine migration or semantic
qualification was performed by these checks.
