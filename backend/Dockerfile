# syntax=docker/dockerfile:1
# Production image for the postpilot API. Multi-stage static build — separate from
# Dockerfile.dev (air hot-reload). The builder compiles a CGO-free static binary; the
# runtime is distroless/static (no shell, non-root) with the API, pinned static
# media executables and the CA certs it needs for outbound TLS.

ARG MEDIA_ASSET_DIGEST=dc1d7d12a71c5ad49785e8abcbd39f755ce51431e75e8c9e261710cb9a83e485

FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS media-build
COPY backend/build/media-tools.sh /build-media-tools.sh
RUN sh /build-media-tools.sh

FROM rust:alpine@sha256:1716b3aa042d735f4566d14dc54e8037de9d69556e2d5dd58131d93a613d173d AS render-build
COPY backend/build/render-tools.sh /build-render-tools.sh
RUN sh /build-render-tools.sh

FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS builder
WORKDIR /src
# Cache modules first — this layer is reused unless go.mod/go.sum change.
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
# Static, stripped, reproducible binary. CGO off so it runs on distroless/static.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go test -c -o /media.test ./internal/clip/media

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /media-worker ./cmd/media-worker
RUN mkdir -p /worker-work
RUN CGO_ENABLED=0 GOOS=linux go test -c -o /diagnostic.test ./internal/clip/diagnostic

FROM builder AS release-builder
RUN CGO_ENABLED=0 GOOS=linux go test -c -o /clip-release.test ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab AS media-runtime
# Both independently versioned images declare the same contract and asset inputs.
ARG SOURCE_REVISION=local
ARG TARGETARCH
ARG MEDIA_ASSET_DIGEST
LABEL org.opencontainers.image.revision=$SOURCE_REVISION \
      org.opencontainers.image.architecture=$TARGETARCH \
      org.postpilot.media.protocol="1" \
      org.postpilot.media.renderer="cpu-v1" \
      org.postpilot.media.assets="assets-v2" \
      org.postpilot.media.asset-input-sha256=$MEDIA_ASSET_DIGEST
WORKDIR /
COPY --from=media-build /opt/media/bin/ffmpeg /opt/media/bin/ffprobe /usr/local/bin/
COPY --from=media-build /opt/media/share/ /usr/share/postpilot-media/
COPY --from=render-build /opt/render/bin/resvg /usr/local/bin/resvg
COPY --from=render-build /opt/render/share/ /usr/share/postpilot-render/
COPY backend/build/render-tools.sh backend/build/RENDER.md /usr/share/postpilot-render/
COPY backend/assets/fonts/ /usr/share/postpilot-fonts/
COPY backend/build/media-tools.sh backend/build/MEDIA.md /usr/share/postpilot-media/
# Shared pinned media/font assets above; API configuration never enters a worker.
USER nonroot:nonroot

FROM media-runtime AS runtime
LABEL org.opencontainers.image.title="Postpilot API" \
      org.postpilot.media.rollback-safe="1"
COPY --from=builder /api /api
# The model registry ships with the image it was written for. A stack may still mount
# its own file over /config/providers.yaml (or point PROVIDERS_CONFIG elsewhere).
COPY backend/config /config
ENV PROVIDERS_CONFIG=/config/providers.yaml
# distroless/static:nonroot runs as uid 65532 and bundles ca-certificates.
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]

FROM runtime AS media-smoke
COPY --from=builder /media.test /media.test
ENV CLIP_MEDIA_SMOKE=1
RUN ["/media.test", "-test.run=Test(MediaSmoke|MediaProfileSmoke|RenderSmoke)", "-test.v"]

# The delivered-clip identity baseline every speed change is measured against
# (CLIP-125). Run on purpose rather than on every build: it renders one fixed
# plan twice and compares the delivered files.
FROM runtime AS identity-smoke
COPY --from=builder /media.test /media.test
ENV CLIP_MEDIA_SMOKE=1
ENTRYPOINT ["/media.test", "-test.run=^Test(DeliveredClipIdentity|AnalysisCopyIdentityAcrossDecoderThreads|PreparationDecodesEachOriginalOnce|SampledGroundsSurviveTheBatchedRead)$", "-test.v", "-test.timeout=45m"]

# Same-host embedded/worker byte, frame and measurement parity, CPU profile.
FROM runtime AS worker-execution-smoke
COPY --from=builder /media.test /media.test
COPY --from=builder /media-worker /media-worker
ENV CLIP_MEDIA_SMOKE=1
RUN ["/media.test", "-test.run=^TestExecRunnerReapsDescendants$", "-test.v", "-test.timeout=1m"]
ENTRYPOINT ["/media.test", "-test.run=^Test(WorkerExecutionParity|WorkerCommandHealth)$", "-test.v", "-test.timeout=45m"]

# Explicit local regression target: never contacts a real provider or stack.
FROM runtime AS release-smoke
COPY --from=release-builder /clip-release.test /clip-release.test
COPY backend/internal/clip/media/testdata/speech.m4a /usr/share/postpilot-media/speech.m4a
ENV CLIP_RELEASE_SMOKE=1
ENTRYPOINT ["/clip-release.test", "-test.run=^TestClipRelease$", "-test.v", "-test.timeout=45m"]

# Every production build must prove the detailed-template / 20-source failure
# shape through queue, strict HTTP admission, planning, rendering and settlement.
# Provider responses and footage are synthetic; this target has no credentials.
FROM runtime AS clip-input-smoke
COPY --from=release-builder /clip-release.test /clip-release.test
COPY backend/internal/clip/media/testdata/speech.m4a /usr/share/postpilot-media/speech.m4a
ENV CLIP_RELEASE_SMOKE=1
RUN ["/clip-release.test", "-test.run=^TestClipWriterInputRelease$", "-test.v", "-test.timeout=15m"]

# ARCH-39. Seconds, not minutes: it runs the test binary the media smoke already built
# and only reads what the binaries report. The bundled ffmpeg is a --disable-everything
# build, so a filter, codec or muxer this code names correctly can still be absent from
# it — `atempo` shipped missing that way and broke every rated cut's audio in production.
FROM runtime AS media-tooling
COPY --from=builder /media.test /media.test
ENV CLIP_MEDIA_SMOKE=1
RUN ["/media.test", "-test.run=^TestBundledToolingCoverage$", "-test.v"]

# What the deploy pushes. Unlike the smokes, which run beside the deploy until closed beta
# (ARCH-38), the tooling check gates the image: it is cheap enough to hold the critical
# path, and what it catches cannot be caught anywhere else.
FROM runtime AS deployable
COPY --from=media-tooling /usr/local/bin/ffmpeg /usr/local/bin/ffmpeg

# Execution-only runtime: no API binary, registry config, database or provider env.
FROM media-runtime AS media-worker-runtime
LABEL org.opencontainers.image.title="Postpilot CPU media worker" \
      org.postpilot.media.manifest-command="/media-worker manifest"
COPY --from=builder /media-worker /media-worker
COPY --from=builder --chown=65532:65532 /worker-work /var/lib/postpilot-media/work
ENV MEDIA_ACCEL=cpu CLIP_WORK_ROOT=/var/lib/postpilot-media/work
HEALTHCHECK --interval=30s --timeout=30s --start-period=30s --retries=3 CMD ["/media-worker", "health"]
ENTRYPOINT ["/media-worker"]

FROM media-worker-runtime AS media-worker-tooling
# Offline, shell-free, actual CPU encode/decode plus the complete capability manifest.
RUN ["/media-worker", "manifest"]

FROM media-worker-runtime AS media-worker-deployable
COPY --from=media-worker-tooling /media-worker /media-worker

FROM media-worker-deployable AS media-worker-smoke
COPY --from=builder /media.test /media.test
ENV CLIP_MEDIA_SMOKE=1
ENTRYPOINT ["/media.test", "-test.run=^Test(WorkerExecutionParity|WorkerCommandHealth|WorkerImageIsolation|ExecRunnerReapsDescendants)$", "-test.v", "-test.timeout=45m"]

# Disposable multi-process gate; fixture endpoints exist only in this test binary.
FROM release-smoke AS media-release-api
ENV MEDIA_RELEASE_SMOKE=1
ENTRYPOINT ["/clip-release.test", "-test.run=^TestMediaRelease$", "-test.v", "-test.timeout=30m"]

FROM media-worker-deployable AS media-release-worker
COPY --from=release-builder /clip-release.test /clip-release.test

# Candidate NVIDIA packaging is independent of the CPU deploy critical path.
FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251 AS nvidia-build
COPY --from=media-runtime /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY backend/build/nvidia-tools.sh /build-nvidia-tools.sh
RUN sh /build-nvidia-tools.sh

FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251 AS media-worker-nvidia-runtime
ARG SOURCE_REVISION=local
ARG TARGETARCH
ARG MEDIA_ASSET_DIGEST
LABEL org.opencontainers.image.revision=$SOURCE_REVISION \
      org.opencontainers.image.architecture=$TARGETARCH \
      org.opencontainers.image.title="Postpilot NVIDIA candidate worker" \
      org.postpilot.media.protocol="1" org.postpilot.media.renderer="cpu-v1" \
      org.postpilot.media.assets="assets-v2" org.postpilot.media.asset-input-sha256=$MEDIA_ASSET_DIGEST \
      org.postpilot.media.gpu-candidate="nvenc-candidate-v1" \
      org.postpilot.media.gpu-production-approved="false"
COPY --from=media-worker-deployable /media-worker /media-worker
COPY --from=media-runtime /usr/local/bin/ /usr/local/bin/
COPY --from=media-runtime /usr/share/postpilot-fonts/ /usr/share/postpilot-fonts/
COPY --from=media-runtime /usr/share/postpilot-media/ /usr/share/postpilot-media/
COPY --from=media-runtime /usr/share/postpilot-render/ /usr/share/postpilot-render/
COPY --from=media-runtime /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=nvidia-build /opt/nvidia/ /opt/nvidia/
COPY backend/build/nvidia-tools.sh /opt/nvidia/share/build-nvidia-tools.sh
COPY --from=builder --chown=65532:65532 /worker-work /var/lib/postpilot-media/work
RUN mkdir /reports && chown 65532:65532 /reports
ENV MEDIA_ACCEL=cpu CLIP_WORK_ROOT=/var/lib/postpilot-media/work \
    NVIDIA_DRIVER_CAPABILITIES=compute,video,utility \
    LD_LIBRARY_PATH=/usr/local/nvidia/lib:/usr/local/nvidia/lib64
USER 65532:65532
ENTRYPOINT ["/media-worker"]

FROM media-worker-nvidia-runtime AS media-worker-nvidia-candidate
# Real CPU execution works without driver/device exposure. GPU checks run later.
RUN ["/media-worker", "manifest"]

# Test binaries and fixture export stay outside the candidate execution image.
FROM media-worker-nvidia-candidate AS media-worker-nvidia-smoke
COPY --from=builder /media.test /media.test
COPY --from=builder /diagnostic.test /diagnostic.test
ENV CLIP_MEDIA_SMOKE=1
ENTRYPOINT ["/diagnostic.test", "-test.v", "-test.timeout=20m"]

FROM deployable AS production
# This dependency makes the real-binary nonroot smoke a mandatory image-build gate.
COPY --from=media-smoke /usr/local/bin/ffmpeg /usr/local/bin/ffmpeg
COPY --from=clip-input-smoke /usr/local/bin/ffprobe /usr/local/bin/ffprobe
