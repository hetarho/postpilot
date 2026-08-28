# syntax=docker/dockerfile:1
# Production image for the postpilot API. Multi-stage static build — separate from
# Dockerfile.dev (air hot-reload). The builder compiles a CGO-free static binary; the
# runtime is distroless/static (no shell, non-root) carrying just the binary + the CA
# certs it needs for outbound TLS.

FROM golang:1.26-alpine AS builder
WORKDIR /src
# Cache modules first — this layer is reused unless go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Static, stripped, reproducible binary. CGO off so it runs on distroless/static.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /api /api
# The model registry ships with the image it was written for. A stack may still mount
# its own file over /config/providers.yaml (or point PROVIDERS_CONFIG elsewhere).
COPY config /config
ENV PROVIDERS_CONFIG=/config/providers.yaml
# distroless/static:nonroot runs as uid 65532 and bundles ca-certificates.
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
