# Container health client

`/media-health` asks the running worker for its validated profile and authenticated
private API status through a private Unix socket. The worker retains capability,
owner generation, configuration, artifact and readiness checks. The client uses
the same SHA-256 namespace and configuration digests, bearer token and status
protocol as before.

Only this small executable is built with the official Go Cryptographic Module
snapshot `GOFIPS140=v1.0.0-c2097c7c`. The bundled source archive must have SHA-256
`daf3614e0406f67ae6323c902db3f953a1effb199142362a039e7526dfb9368b`;
both Dockerfiles verify it before compilation. The digest is supplied by the
pinned Go distribution's `lib/fips140/fips140.sum`. The snapshot's upstream
BSD 3-Clause license ships beside this document as `LICENSE`.

Go 1.26's current cryptography links a 32 MiB entropy scratch buffer. On the
measured amd64 image under Rosetta, importing SHA-256 backed that buffer even
without using its entropy source. The frozen snapshot removes that buffer:
the actual health client returned identical readiness JSON while its measured
RSS fell from 42,876 KiB to 10,072 KiB under the existing 512 MiB/1 CPU worker
budget. This observation does not establish native runner performance.

The supported `GOFIPS140` build setting enables FIPS mode by default **only in
the health executable**. API, worker, server cryptography and runtime environment
keep their existing settings. This packaging choice makes no application or
image certification claim. Version availability and mode behavior are described
in [Go's FIPS documentation](https://go.dev/doc/security/fips140). Updating the
pinned Go toolchain requires checking snapshot availability, archive digest,
client build metadata and the actual default-budget release gates again.
