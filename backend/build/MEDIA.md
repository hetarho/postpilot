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

The T076 consumer must stage one source at a time under the callback's workspace,
probe it, remove it before staging another source, and retain only `MediaInfo`.
After validating the whole manifest and reserving the exact chunk count, it can
stage each source again for observation. `PrepareAnalysisChunks` synchronously
hands off one proxy at a time and removes that proxy even if the consumer fails or
panics. `WithWorkspace` owns final cleanup; cancellation never bypasses cleanup.
This bounds local footage to the current source (at most 2 GiB) and current proxy.
Cloud source/proxy cleanup remains the durable worker's responsibility.
