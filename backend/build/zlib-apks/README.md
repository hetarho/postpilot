# Retained signed Alpine zlib packages

`x86_64/` contains the unchanged Alpine **1.3.2-r1** runtime, development and static
packages. They were downloaded on 2026-11-11 from:

- https://dl-cdn.alpinelinux.org/alpine/v3.24/main/x86_64/zlib-1.3.2-r1.apk (56,210 bytes)
- https://dl-cdn.alpinelinux.org/alpine/v3.24/main/x86_64/zlib-dev-1.3.2-r1.apk (34,429 bytes)
- https://dl-cdn.alpinelinux.org/alpine/v3.24/main/x86_64/zlib-static-1.3.2-r1.apk (61,678 bytes)

Every file matches its pre-existing SHA-256 in [MEDIA.md](../MEDIA.md). That document
also identifies the pinned Alpine package source, patch, license and foreign
architecture limits. These are Alpine's original signed archives, not rebuilt or
re-signed packages. The build checks all three hashes and uses the pinned base's
normal `apk add` signature verification before admitting them.

Alpine's [official mirror list](https://mirrors.alpinelinux.org/) identifies the CDN.
Retention varies by endpoint: the GitHub runner received a 404 for zlib-static in
[Deploy backend 43011959371](https://github.com/hetarho/postpilot/actions/runs/43011959371),
while the host endpoint still served these identical bytes. Keeping them in the
build context preserves the qualified pin without depending on mirror retention.
The indexed installation still runs first; other architectures use the existing
bounded download fallback. The packages do not enter the final runtime image.
