#!/bin/sh
set -eu
RESVG_VERSION=0.48.1
RESVG_SHA256=40dafea6b4b9d01e9d28b6d49f1e912daf3e9055676ad9179a5a2db6e7386945
apk add --no-cache build-base curl binutils
mkdir -p /render-build /opt/render/bin /opt/render/share/sources /opt/render/share/licenses
cd /render-build
curl -fL --connect-timeout 10 --max-time 120 --retry 3 -o resvg.tar.gz "https://codeload.github.com/linebender/resvg/tar.gz/refs/tags/v${RESVG_VERSION}"
printf '%s  resvg.tar.gz\n' "$RESVG_SHA256" | sha256sum -c -
tar -xf resvg.tar.gz
cd "resvg-${RESVG_VERSION}"
render_target=$(rustc -vV | sed -n 's/^host: //p')
test -n "$render_target"
RUSTFLAGS='-C target-feature=+crt-static' cargo build --release --locked --target "$render_target" -p resvg --bin resvg
cp "target/${render_target}/release/resvg" /opt/render/bin/resvg
cp LICENSE-MIT LICENSE-APACHE /opt/render/share/licenses/
cp Cargo.lock /opt/render/share/
cp /render-build/resvg.tar.gz /opt/render/share/sources/
# Includes the exact dependency source and license notices from the upstream lock.
cargo vendor --locked /opt/render/share/sources/vendor > /opt/render/share/vendor-config.toml
/opt/render/bin/resvg --version
if readelf -l /opt/render/bin/resvg | grep -q INTERP; then exit 1; fi
