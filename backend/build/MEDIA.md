# Pinned media tools

Both Dockerfiles run `media-tools.sh`. Production builds from the repository root with
`docker build -f backend/Dockerfile .`; the deployment workflow uses the same context.
The final runtime remains `distroless/static-debian12:nonroot`. Go does not link FFmpeg.
FFmpeg and ffprobe are separate, statically linked musl executables, invoked without a shell.

| Component | Pin | SHA-256 | License |
|---|---|---|---|
| FFmpeg | 9.0.1 | cf38e0e28c7e5605942c4a77755349b0145804a397af37eb1fb4c77cb237f635 | GPL-2.0-or-later for this libx264-enabled build |
| x264 | stable b35605ace3ddf7c1a5d67a2eb553f034aef41d55 | 6eeb82934e69fd51e043bd8c5b0d152839638d1ce7aa4eea65a3fedcf83ff224 | GPL-2.0-or-later |
| musl / zlib | musl 1.2.6-r2 / zlib 1.3.2-r0, with resolved Alpine package versions recorded in the image | Signed Alpine packages; media-build base sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 | MIT / Zlib |

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
intermediate originals and all proxy/request workspaces must disappear at terminal
cleanup. Disk for the local fake bucket is fixture storage, not worker workspace.
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
