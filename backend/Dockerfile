# syntax=docker/dockerfile:1
# Production image for the postpilot API. Multi-stage static build — separate from
# Dockerfile.dev (air hot-reload). The builder compiles a CGO-free static binary; the
# runtime is distroless/static (no shell, non-root) with the API, pinned static
# media executables and the CA certs it needs for outbound TLS.

FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS media-build
COPY backend/build/media-tools.sh /build-media-tools.sh
RUN sh /build-media-tools.sh

FROM rust:alpine@sha256:1716b3aa042d735f4566d14dc54e8037de9d69556e2d5dd58131d93a613d173d AS render-build
COPY backend/build/render-tools.sh /build-render-tools.sh
RUN sh /build-render-tools.sh

FROM golang:1.26-alpine AS builder
WORKDIR /src
# Cache modules first — this layer is reused unless go.mod/go.sum change.
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
# Static, stripped, reproducible binary. CGO off so it runs on distroless/static.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go test -c -o /media.test ./internal/clip/media

FROM builder AS release-builder
RUN CGO_ENABLED=0 GOOS=linux go test -c -o /clip-release.test ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /
COPY --from=builder /api /api
COPY --from=media-build /opt/media/bin/ffmpeg /opt/media/bin/ffprobe /usr/local/bin/
COPY --from=media-build /opt/media/share/ /usr/share/postpilot-media/
COPY --from=render-build /opt/render/bin/resvg /usr/local/bin/resvg
COPY --from=render-build /opt/render/share/ /usr/share/postpilot-render/
COPY backend/build/render-tools.sh backend/build/RENDER.md /usr/share/postpilot-render/
COPY backend/assets/fonts/ /usr/share/postpilot-fonts/
COPY backend/build/media-tools.sh backend/build/MEDIA.md /usr/share/postpilot-media/
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

# Explicit local regression target: never contacts a real provider or stack.
FROM runtime AS release-smoke
COPY --from=release-builder /clip-release.test /clip-release.test
COPY backend/internal/clip/media/testdata/speech.m4a /usr/share/postpilot-media/speech.m4a
ENV CLIP_RELEASE_SMOKE=1
ENTRYPOINT ["/clip-release.test", "-test.run=^TestClipRelease$", "-test.v", "-test.timeout=45m"]

FROM runtime AS production
# This dependency makes the real-binary nonroot smoke a mandatory image-build gate.
COPY --from=media-smoke /usr/local/bin/ffmpeg /usr/local/bin/ffmpeg
