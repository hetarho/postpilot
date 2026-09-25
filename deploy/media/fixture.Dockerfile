# syntax=docker/dockerfile:1
# Disposable release-test storage only. MinIO no longer publishes community binaries.
# Build official source revisions, independently of the production API/worker images.
FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS build
ENV CGO_ENABLED=0 GOBIN=/out
# RELEASE.2025-09-07T16-13-09Z and RELEASE.2025-08-13T08-35-41Z, respectively.
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go install -trimpath -ldflags='-s -w' github.com/minio/minio@v0.0.0-20250907161309-07c3a429bfed && \
    mkdir -p /out/licenses && \
    cp /go/pkg/mod/github.com/minio/minio@v0.0.0-20250907161309-07c3a429bfed/LICENSE /out/licenses/minio.txt
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go install -trimpath -ldflags='-s -w' github.com/minio/mc@v0.0.0-20250813083541-7394ce0dd2a8 && \
    cp /go/pkg/mod/github.com/minio/mc@v0.0.0-20250813083541-7394ce0dd2a8/LICENSE /out/licenses/mc.txt

FROM alpine:3.23@sha256:85fe1e81d6758c208f3e1eed4338a1997e19d4be002d4dd32d3100c9a8c010a0 AS media-storage
LABEL org.opencontainers.image.title="Disposable Postpilot media release storage" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.postpilot.fixture.minio.source="https://github.com/minio/minio/tree/07c3a429bfed433e49018cb0f78a52145d4bedeb" \
      org.postpilot.fixture.mc.source="https://github.com/minio/mc/tree/7394ce0dd2a80935aded936b09fa12cbb3cb8096"
COPY --from=build /out/minio /out/mc /usr/local/bin/
COPY --from=build /out/licenses/ /usr/share/licenses/minio/
ENTRYPOINT ["minio"]
