# Pinned media tools

Both Dockerfiles run `media-tools.sh`. Production builds from the repository root with
`docker build -f backend/Dockerfile .`; the deployment workflow uses the same context.
The final runtime remains `distroless/static-debian12:nonroot`. Go does not link FFmpeg.
FFmpeg and ffprobe are separate, statically linked musl executables, invoked without a shell.

| Component | Pin | SHA-256 | License |
|---|---|---|---|
| FFmpeg | 9.0.1 | cf38e0e28c7e5605942c4a77755349b0145804a397af37eb1fb4c77cb237f635 | GPL-2.0-or-later for this libx264-enabled build |
| x264 | stable b35605ace3ddf7c1a5d67a2eb553f034aef41d55 | 6eeb82934e69fd51e043bd8c5b0d152839638d1ce7aa4eea65a3fedcf83ff224 | GPL-2.0-or-later |
| musl / zlib | musl 1.2.6-r2 / zlib 1.3.2-r1 (runtime, development and static packages), with resolved Alpine package versions recorded in the image | Signed Alpine packages; media-build base sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 | MIT / Zlib |

### Retained signed zlib packages

The indexed installation remains the first attempt on every architecture. If it fails,
`media-tools.sh` can recover the same runtime, development and static **1.3.2-r1**
packages from `https://dl-cdn.alpinelinux.org/alpine/v3.24/main/<arch>/<package>-1.3.2-r1.apk`.
BusyBox wget from the pinned base downloads them into an attempt-owned temporary
directory. All three fixed SHA-256 values are checked before normal `apk add`
verifies their signatures with the base image's existing architecture keys. No new
signing key, untrusted-package option or floating version is used. Only that temporary
APK directory is removed on success or failure; the remaining tool pins and FFmpeg
configuration stay the same.

| Fallback architecture | zlib SHA-256 | zlib-dev SHA-256 | zlib-static SHA-256 |
|---|---|---|---|
| aarch64 | 1f77a542ac6634761b9fe38d8bdca19b154071dcfa25432e73f2bc8a0740a33f | ed8acbc6e524accefbf62e236216c6f3f3141e45abc015caf05a4304ee11f0c6 | 9114059965de3a496021b98b1fffb6d0b25b105684db0c34ee4f88e499cf5005 |
| armhf | ff936d0aa98b7ab6a28aeaf13ef662df27ee224fe60dff74eff965a2e53c3db9 | bb45bf59c4a4bbaff5dc3c123139a544ab8a5b79a3338b2ff879f3d62cefe2b5 | ab520d3e2aa32d2b363528542d57c756d5144aa357fe27f6ec30ed543bc160e3 |
| armv7 | 4a4077c3243a6dfd5e16df20ecbe100acd72ab24eab32c91790ec1eb32d23053 | cd24f88dee17e03533990d1c478d30b921440e0a725dbb73e04b4707b5bbf38e | 82ccd90ce5825395eb57f3bdc06cfc307b0747e3147f2e860bfedf8bcc6c13b4 |
| ppc64le | da41fad1af7debdb5dcaa02599975b34c7f8f12be99cc9c3775ff8a2a0b916d1 | f4fd5ad81fb55d7556bba75cd7a724f74e0a796691a93c0413c7b9c821debcd6 | cf3e37ec8adcb8742470166d6e5f0c5a820e0c1828d50c3092ce294afff2fd85 |
| riscv64 | df0fed8bca1579423b1a78f3dde7e8130ea81908bcaacf179d452ae81cc4fdda | e7b400d415591e5e9537c28ac488a175f8ef8dcd668f48e8ceb15707596c52cb | 6b0280a4a83cca1738c5738f2fbb0409bb1d8b3e3dd85641e989b7be30895f92 |
| s390x | 8f1bf7536ef6b1ced56b0cb61670bcaff81789402fa0c696e8d7ab5d824cceac | ba06965a2a2d3f82ba851f4ff7b031548a0212942e56cecb82c00e8217210b46 | 3e6d1e2daa55ef0e45fdae17a0318c5c1d7ebc3d45e24630a462a2276397bbe5 |
| x86_64 | 63aeea03c15a2f9018f81805cfc8aa926bdf5cd68921f22149c2fbb5d0ee9f47 | 50e47b778d894841a9c8ace7640edc1e585269963f29d6e4fdc008c8d6364890 | 1b7ce8d0515576599450144a5652eeba0e04e4b7342bba32cbc5a2a4ae256939 |

These artifacts were read from the official mirror and checked against their signed
package identity. Actual signature verification, resolver and installation qualification
is linux/amd64 with the pinned Go base's default keys. Foreign native execution is
unqualified; each foreign build must pass its own normal architecture signature check.
The fallback covers the seven complete sets above. The official mirrors returned 404
for loongarch64's zlib-dev and x86's zlib-static artifact. Those architectures and any
unknown architecture retain the original indexed attempt, but fail clearly if fallback
would be needed. Historical artifact retention is not guaranteed: missing downloads or
changed checksums fail closed and require a reviewed repair.

The official [Alpine package source at 0afa2da0](https://github.com/alpinelinux/aports/blob/0afa2da0e8c8051c6f8f64a7a388e5a259904245/main/zlib/APKBUILD)
identifies 1.3.2-r1 and includes the adjacent `CVE-2026-85091.patch` backport. This is
Alpine's patched r1 build, not unpatched upstream 1.3.2. Its source and patch SHA-512 are:

```text
70963771ea5d763614278a69b474f09b7d237ef8f53b675a10fe31d9923aeef601504b35d7ebd1b1e7f347e9ebb048e6b3b47fffdf137e7bdc7e8d5eb4ec4692  zlib-1.3.2.tar.gz
878888308a51296470f05eca59a79212314d4c9a26c19bbf70c1cb1b8d25eb42e0cce5a001acbe287268aa215c854cb5a9990ff62b7aefa9de221803d172957e  CVE-2026-85091.patch
```

Both were downloaded and matched against that APKBUILD. Repository/index behavior
follows the [official apk handbook](https://docs.alpinelinux.org/user-handbook/0.1a/Working/apk.html).
This pins the recovered zlib artifacts; it does not freeze the build's other existing
unversioned package requests or certify GPU/foreign execution. The asset-input label
covers fonts, overlay presets, design and metrics; this packaging repair does not change
those inputs. Real-image CPU gates remain required for the current source candidate.

FFmpeg source and signature: https://ffmpeg.org/releases/ffmpeg-9.0.1.tar.xz
and `.asc`; the build checks SHA-256 and the official signing fingerprint
`FCF986EA15E6E293A5644F10B4322F04D67658D8`.
x264 source: https://code.videolan.org/videolan/x264/-/archive/b35605ace3ddf7c1a5d67a2eb553f034aef41d55/x264-b35605ace3ddf7c1a5d67a2eb553f034aef41d55.tar.bz2

Corresponding FFmpeg/x264 source archives, their license notices, the build script and
resolved Alpine component versions ship under `/usr/share/postpilot-media/`.
The musl copyright notice is extracted from the Alpine-mirrored 1.2.6 source archive,
verified with SHA-512 `1adad96eddb3a2eb0cacb3e363b0046568925fcdd75cf8b0503f2139df1f693d64730779ca0ce8131b7624ab2d37f4247bb1d3393c523de6e30d2b1d7732555c`
from Alpine's `3.24-stable/main/musl/APKBUILD`; zlib's 1.3.2 license is included too.
The enabled filter set is an allowlist, not a default build: `fade` is in it
because the clip design system's only permitted entrance and exit are a 180 ms
and a 120 ms alpha fade, and the renderer animates the caption plate's alpha
with it. `loop` reuses one decoded caption/card frame for a finite cut-length
sequence, avoiding infinite PNG demuxer inputs and repeated image decoding.
`select` takes only the few frames a cut's grounds are measured on, in one output,
so the sampler scales and writes those frames and no others.
No nonfree codec option is enabled. Input protocols are file and pipe only; network,
external-device capture and unneeded containers are disabled. Common H.264/HEVC,
VP8/VP9, MPEG-4, MJPEG and ProRes inputs are supported in the accepted containers;
unsupported codecs are rejected before observation.

Official references checked for this pin:

- https://ffmpeg.org/download.html — releases and signature verification
- https://ffmpeg.org/ffmpeg.html — argument ordering, autorotation and progress
- https://ffmpeg.org/ffprobe.html — machine-readable streams and containers
- https://ffmpeg.org/ffmpeg-filters.html — scale, frame rate and sample aspect ratio
- https://ffmpeg.org/legal.html — license conditions, including libx264
- https://code.videolan.org/videolan/x264 — source and configure options

`media-smoke` runs the actual adapter against rotated, silent, audio and 61-second
fixtures as uid 65532 inside the distroless runtime. Production depends on this stage.
Ordinary `go test` uses an injected runner and needs no host FFmpeg installation.

## Workspace contract

`CLIP_WORK_ROOT` defaults to `/tmp/postpilot-clip-work`. It must be a dedicated,
writable directory: first use accepts only an empty directory and creates a private
ownership marker; later boots verify the marker and reject symlinks, home/system
roots and unresolved environment expressions. Never set it to the repository, a
home directory or a shared temporary root. Only stale `postpilot-clip-<32 hex>`
children are removed. `CLIP_WORK_STALE_AGE` defaults to 6 hours and
`CLIP_MEDIA_TIMEOUT` to 15 minutes per binary call.

The T084 consumer downloads each original once for preparation, probes and converts
that same file, then removes it before the next original. Every analysis copy is
fully decoded and its timing, dimensions, codecs, audio and bytes are verified
before the one exact-count credit reservation. Paths remain workspace-owned until
their individual observations finish; no proxy is uploaded or presigned. Old cloud
proxy records remain sweepable. Cancellation and panic retain both local workspace
cleanup and durable original-source cleanup.

Analysis is 15 FPS H.264/yuv420p, CRF 28, long edge at most 720 without upscaling,
900 kbit/s maximum video rate with a 1,800 kbit buffer, and mono AAC 48 kHz/64 kbit/s.
An oversized copy gets one full same-interval retry at 650/1,300 kbit; it cannot
create extra chunks, omit coverage or retry AI. Each copy is at most 60 seconds
and 8 MiB. Exact audio sample trimming prevents AAC packetization from producing
a 60.011-second container. Timestamp gaps are filled/trimmed (`async=1`, no soft
time stretching), preserving speech synchronization rather than closing gaps.

Bounds: one global job worker, one media subprocess at a time, one original up to
2 GiB, prepared copies up to 512 MiB, complete workspace up to 8 GiB. Before credit
admission the filesystem must have room for the remaining entire workspace bound.
Downloads check capacity on every write; subprocesses check before starting and
every 100 ms while writing, with output file limits and muxer headroom. Root and
workspace ownership validation also applies to these checks. Nested caption-only
workspaces share the subprocess semaphore without locking the outer workspace.

Full-resolution composition uses a balanced tree with at most two video decoders
per subprocess. Intermediate video nodes are lossless H.264 4:4:4 with x264's
`ultrafast` preset (less CPU compression work, more bounded temporary disk); final rendering
keeps 30 FPS, CRF 20 and yuv420p. Original cut audio bypasses the intermediate nodes
and receives the existing one final AAC composition pass. The same bounded path
serves paid generation and credit-free rerender.

`TestMediaProfileSmoke` additionally checks deterministic noisy/moving 60-second
video, VFR and locally synthesized speech. The 60-second fixture produced a
7,372,694-byte proxy with an exact 60,000 ms playable timeline; same-offset speech
correlation checks cover its beginning, middle and end. The tests use real pinned
binaries in the nonroot runtime and no paid provider call. A separate runtime run
enforces `--memory 1g --cpus 2 --network none`; T086 owns the full 20-source stress.

## Isolated release regression

Run from the repository root. These targets use production's pinned binaries,
font and nonroot runtime, but only temporary SQLite databases, synthetic media
and a counted loopback HTTP provider. Do not supply an env file, credentials,
host database mounts or user footage. `--network none` still permits loopback.

```sh
docker build -f backend/Dockerfile --target release-smoke -t postpilot-clip-release:local .
docker run --rm --network none --memory 1g --memory-swap 1g --cpus 2 postpilot-clip-release:local
docker run --rm --network none --memory 1g --memory-swap 1g --cpus 2 -e CLIP_RELEASE_STRESS=1 postpilot-clip-release:local
docker build -f backend/Dockerfile --target media-smoke -t postpilot-clip-media:local .
docker run --rm --network none --memory 1g --memory-swap 1g --cpus 2 --entrypoint /media.test postpilot-clip-media:local -test.run='Test(MediaSmoke|MediaProfileSmoke|RenderSmoke)' -test.v -test.timeout=15m
```

`TestClipRelease` drives authenticated quote/start/poll through the real handlers,
SQLite source linkage, queue, frozen OpenRouter-compatible transport, usage ledger,
media adapter and original-footage renderer. Every verified proxy must exist before
the hold and first completion. The HTTP stub checks exact Content-Length, static
inline data, hashes of verified proxies, stage budgets, price bounds and disabled
optional features. It never acts as an external AI quality evaluation.

The ordinary-account matrix includes no-usage denial, partial paid failure,
reported supplier overage above unequal approval/hold limits, unknown usage,
malformed/truncated/oversized output, save failure, malformed last source,
oversized last proxy, disk refusal, unknown/drifting media rates, legacy client,
expired/changed approval, concurrent acceptance and an aborted HTTP response.
Master uses a separate fixture. New worker instances recover interruptions at
prepare/hold/partial-usage/save without installing a replay handler. The suites
below additionally cover failed terminal/settlement writes and simultaneous lots.

The stress case uses 19 distinct 61-second containers and one 641-second container
(20 sources, exactly 30 minutes, 49 chunks); identity uses the browser's bounded
v1 fingerprint, not filenames. Synthetic speech and simple frames deliberately
separate count/duration stress from the high-motion quality case. Every original
is independently downloaded/probed/prepared, then only selected originals are
downloaded again for final rendering. The final 1920×1080/15-second result is
decoded and speech is compared with the original at identical offsets.

Metrics report cgroup-wide peak memory (including subprocesses and fixture setup),
20 ms sampled workspace disk/proxy high-water marks, actual request/proxy bytes,
simultaneous originals/workspaces/subprocesses, preparation/render/total elapsed
time and approval/hold/charge/refund. Prepared paths are restatted at admission;
local intermediate originals and all proxy/request workspaces must disappear at terminal
cleanup. Confirmed private original objects remain reusable until their retention deadline;
first durable terminal release renews it once, while project deletion/finalization fences
access and queues physical deletion. Incomplete uploads still follow CLIP_SOURCE_BATCH_TTL. Disk for the local fake bucket is fixture storage, not worker workspace.
The separate noisy 60-second case measures both preparation and original rendering
and checks proxy/final speech timing, VFR, rotation and silence.

The release speech fixture caught a one-AAC-packet (~21.3 ms) early shift in final
rendering: resetting audio STARTPTS before aligning its samples discarded the
seek-relative clock. Cut rendering now applies `aresample` with `async=1`,
`min_hard_comp=0` and `first_pts=0` before trimming/resetting the output clock;
this preserves initial silence and gaps without stretching speech. The regression
also covers a nonzero cut start and delayed source audio. See the official
[resampler timestamp controls](https://www.ffmpeg.org/ffmpeg-resampler.html).

```sh
cd backend
go test -race ./internal/usage/... ./internal/job/... ./internal/clip/... ./cmd/api -timeout 20m
go test ./internal/llm/... ./internal/generation/... ./internal/experiment/... ./internal/provider/... ./internal/post/...
```

The regular tests pin conditional/cache/reasoning price units, known zero versus
absent rates, frozen metadata drift, atomic refunds, legacy payload refusal and
photo/signed-video-URL/free-rerender compatibility. Frontend regressions live in
`ClipGeneration.test.tsx`, `ClipCorrection.test.tsx` and source-session tests;
they cover quote invalidation, no automatic paid retry, owned preview lifetime,
page exit and authoritative settlement lag. The release harness itself neither
pushes nor deploys and requires no paid model completion.

Additional option references checked for T084:

- https://ffmpeg.org/ffmpeg-codecs.html#libx264_002c-libx264rgb — CRF, VBV and lossless encoding
- https://ffmpeg.org/ffmpeg-filters.html#atrim — exact sample bounds
- https://ffmpeg.org/ffmpeg-resampler.html — first timestamps and hard gap compensation

## Portable composition renderer

Version-5 plans render only retained declarations. Source cuts and transition-tree
nodes use lossless H.264 4:4:4; output-relative overlays are applied afterward in
bounded time windows with at most eight PNG inputs per pass. The final video has
one CRF encode. Audio stays PCM until final AAC/loudness processing. Layout and
render return the same element identities, authority, measured geometry, motion,
phrase windows, effective choices and omission reasons for draft preview.

`TestRenderSmokeComposition` checks all three ratios, exact output boundaries,
persistent opacity across source joins, last-frame luminance sampling and explicit
conversion of an old plan. The separate native resource gate renders 100 cuts over
90 seconds with 200 readable rapid phrases, verifies later-window frame clocks,
checks intermediate cleanup and reports cgroup peak memory. The 2,400-cue graph
bound is a regular unit test; that many sequential 300 ms phrases cannot fit into
a 90-second output.

```sh
docker build -f backend/Dockerfile --target media-smoke -t postpilot-clip-media:local .
docker run --rm --network none --memory 1g --memory-swap 1g --cpus 2 -e CLIP_COMPOSITION_STRESS=1 --entrypoint /media.test postpilot-clip-media:local -test.run='^TestRenderCompositionStress$' -test.v -test.timeout=30m
```

### Original cadence evidence

Slow playback requires the original full-decode admission pass to report constant
video timestamp intervals through `vfrdet`. The bounded stderr report shares that
pass with decoded frame count and duration; no second decode or per-frame log is
needed. Average FPS and frame count alone can agree for VFR, so they are only a
cross-check after this positive evidence. Missing or variable cadence (including
older stored metadata without the evidence flag) offers 1x and faster rates only.
Existing plans and result bytes are not rewritten. The production smoke generates
30/40/60 fps CFR and a VFR original averaging 48 fps to verify the distinction.
