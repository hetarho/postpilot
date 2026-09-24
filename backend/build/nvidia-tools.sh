#!/bin/sh
# Isolated diagnostic encoder. Production CPU binaries are copied unchanged.
set -eu
FFMPEG_VERSION=9.0.1
FFMPEG_SHA256=cf38e0e28c7e5605942c4a77755349b0145804a397af37eb1fb4c77cb237f635
NV_HEADERS_VERSION=n13.0.19.0
NV_HEADERS_SHA256=86d15d1a7c0ac73a0eafdfc57bebfeba7da8264595bf531cf4d8db1c22940116
# Immutable base + dated package repositories pin compiler/runtime provenance.
rm -f /etc/apt/sources.list.d/debian.sources
cat > /etc/apt/sources.list <<'SOURCES'
deb [check-valid-until=no] https://snapshot.debian.org/archive/debian/20260924T000000Z bookworm main
deb [check-valid-until=no] https://snapshot.debian.org/archive/debian-security/20260924T000000Z bookworm-security main
SOURCES
apt-get -o Acquire::Retries=3 update
apt-get install -y --no-install-recommends build-essential pkg-config nasm curl xz-utils
mkdir -p /nvidia-build /opt/nvidia/share/sources /opt/nvidia/share/licenses
cd /nvidia-build
curl -fL --connect-timeout 10 --max-time 120 --retry 3 -o ffmpeg.tar.xz "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz"
printf '%s  ffmpeg.tar.xz\n' "$FFMPEG_SHA256" | sha256sum -c -
curl -fL --connect-timeout 10 --max-time 120 --retry 3 -o nv-headers.tar.gz "https://codeload.github.com/FFmpeg/nv-codec-headers/tar.gz/refs/tags/${NV_HEADERS_VERSION}"
printf '%s  nv-headers.tar.gz\n' "$NV_HEADERS_SHA256" | sha256sum -c -
tar -xf nv-headers.tar.gz
make -C "nv-codec-headers-${NV_HEADERS_VERSION}" PREFIX=/opt/nvidia install
cp "nv-codec-headers-${NV_HEADERS_VERSION}/include/ffnvcodec/nvEncodeAPI.h" /opt/nvidia/share/licenses/
cp "nv-codec-headers-${NV_HEADERS_VERSION}/README" /opt/nvidia/share/
tar -xf ffmpeg.tar.xz
cd "ffmpeg-${FFMPEG_VERSION}"
PKG_CONFIG_PATH=/opt/nvidia/lib/pkgconfig ./configure \
  --prefix=/opt/nvidia --disable-shared --enable-static \
  --disable-doc --disable-debug --disable-autodetect --disable-network --disable-everything \
  --enable-ffmpeg --enable-ffprobe --enable-avcodec --enable-avformat --enable-avfilter --enable-swscale --enable-swresample \
  --enable-ffnvcodec --enable-nvenc --enable-cuvid \
  --enable-protocol=file,pipe --enable-demuxer=mov,image2,image2pipe,wav \
  --enable-decoder=h264,h264_cuvid,aac,pcm_s16le,wrapped_avframe \
  --enable-parser=h264,aac --enable-encoder=h264_nvenc,rawvideo,pcm_s16le,wrapped_avframe \
  --enable-muxer=mp4,null,image2,image2pipe,wav,rawvideo,framemd5 \
  --enable-bsf=h264_mp4toannexb,aac_adtstoasc \
  --enable-indev=lavfi --enable-filter=color,scale,setsar,fps,format,null,anull,hwdownload,hwupload_cuda
make -j2
make install
cp COPYING.LGPLv2.1 /opt/nvidia/share/licenses/
cp /nvidia-build/ffmpeg.tar.xz /nvidia-build/nv-headers.tar.gz /opt/nvidia/share/sources/
dpkg-query -W > /opt/nvidia/share/build-packages.txt
cp ffbuild/config.mak /opt/nvidia/share/config.mak
/opt/nvidia/bin/ffmpeg -version
/opt/nvidia/bin/ffmpeg -hide_banner -encoders | grep -w h264_nvenc
/opt/nvidia/bin/ffmpeg -hide_banner -decoders | grep -w h264_cuvid
# A glibc loader is intentional; host driver libraries are injected at runtime.
ldd /opt/nvidia/bin/ffmpeg
