#!/bin/sh
# Shared by production and dev: static musl binaries, no runtime package manager.
set -eu
FFMPEG_VERSION=9.0.1
FFMPEG_SHA256=cf38e0e28c7e5605942c4a77755349b0145804a397af37eb1fb4c77cb237f635
X264_REV=b35605ace3ddf7c1a5d67a2eb553f034aef41d55
X264_SHA256=6eeb82934e69fd51e043bd8c5b0d152839638d1ce7aa4eea65a3fedcf83ff224
apk add --no-cache build-base bash nasm pkgconf curl xz bzip2 gnupg zlib-dev=1.3.2-r0 zlib-static=1.3.2-r0
mkdir -p /media-build /opt/media/share/licenses /opt/media/share/sources
cd /media-build
curl -fL --connect-timeout 10 --max-time 120 --retry 3 -o musl.tar.gz https://distfiles.alpinelinux.org/distfiles/v3.24/musl-1.2.6.tar.gz
printf '%s  musl.tar.gz\n' '1adad96eddb3a2eb0cacb3e363b0046568925fcdd75cf8b0503f2139df1f693d64730779ca0ce8131b7624ab2d37f4247bb1d3393c523de6e30d2b1d7732555c' | sha512sum -c -
tar -xOf musl.tar.gz musl-1.2.6/COPYRIGHT > /opt/media/share/licenses/musl-COPYRIGHT.txt
cp musl.tar.gz /opt/media/share/sources/
curl -fL --connect-timeout 10 --max-time 120 --retry 3 -o /opt/media/share/licenses/zlib-LICENSE.txt https://raw.githubusercontent.com/madler/zlib/v1.3.2/LICENSE
curl -fL --retry 3 -o ffmpeg.tar.xz "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz"
curl -fL --retry 3 -o ffmpeg.tar.xz.asc "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz.asc"
curl -fL --retry 3 -o ffmpeg-signing-key.asc https://ffmpeg.org/ffmpeg-devel.asc
printf '%s  ffmpeg.tar.xz\n' "$FFMPEG_SHA256" | sha256sum -c -
gpg --batch --import ffmpeg-signing-key.asc
gpg --batch --status-fd 1 --verify ffmpeg.tar.xz.asc ffmpeg.tar.xz > signature-status
grep -q 'VALIDSIG FCF986EA15E6E293A5644F10B4322F04D67658D8 ' signature-status
curl -fL --retry 3 -o x264.tar.bz2 "https://code.videolan.org/videolan/x264/-/archive/${X264_REV}/x264-${X264_REV}.tar.bz2"
printf '%s  x264.tar.bz2\n' "$X264_SHA256" | sha256sum -c -
tar -xf x264.tar.bz2
cd "x264-${X264_REV}"
./configure --prefix=/opt/media --enable-static --enable-pic --disable-cli --disable-opencl --disable-avs --disable-lavf --disable-swscale --disable-ffms --disable-gpac --disable-lsmash
make -j2
make install
cp COPYING /opt/media/share/licenses/x264-GPL-2.0.txt
cd /media-build
tar -xf ffmpeg.tar.xz
cd "ffmpeg-${FFMPEG_VERSION}"
PKG_CONFIG_PATH=/opt/media/lib/pkgconfig ./configure \
  --prefix=/opt/media --pkg-config-flags=--static --extra-ldflags=-static \
  --disable-shared --enable-static --disable-doc --disable-debug --disable-autodetect \
  --disable-network --disable-everything --enable-gpl --enable-libx264 --enable-zlib \
  --enable-ffmpeg --enable-ffprobe --enable-avcodec --enable-avformat --enable-avfilter --enable-swscale --enable-swresample \
  --enable-protocol=file,pipe --enable-demuxer=mov,matroska,image2,image2pipe \
  --enable-decoder=h264,hevc,vp8,vp9,mpeg4,mjpeg,prores,aac,mp3,opus,vorbis,alac,pcm_s16le,pcm_s24le,pcm_s32le,pcm_f32le,pcm_s16be,pcm_s24be,pcm_s32be,wrapped_avframe \
  --enable-parser=h264,hevc,vp8,vp9,mpeg4video,mjpeg,aac,mpegaudio,opus,vorbis \
  --enable-decoder=png --enable-encoder=libx264,aac,wrapped_avframe,pcm_s16le,png --enable-muxer=mp4,null,image2,image2pipe,pcm_s16le \
  --enable-bsf=aac_adtstoasc,h264_mp4toannexb,hevc_mp4toannexb \
  --enable-filter=scale,setsar,fps,format,transpose,hflip,vflip,aresample,aformat,anull,null,trim,atrim,setpts,asetpts \
  --enable-filter=crop,overlay,xfade,acrossfade,concat,volume,apad,settb,asettb,fade,afade,loudnorm \
  --enable-indev=lavfi --enable-filter=color,sine,anullsrc
make -j2
make install
cp COPYING.GPLv2 COPYING.LGPLv2.1 /opt/media/share/licenses/
cp /media-build/ffmpeg.tar.xz /media-build/ffmpeg.tar.xz.asc /media-build/x264.tar.bz2 /opt/media/share/sources/
apk info -v | grep -E '^(musl|zlib)' > /opt/media/share/licenses/alpine-components.txt
/opt/media/bin/ffmpeg -version
/opt/media/bin/ffprobe -version
# Readelf confirms no dynamic loader is required by distroless/static.
if readelf -l /opt/media/bin/ffmpeg | grep -q INTERP; then exit 1; fi
if readelf -l /opt/media/bin/ffprobe | grep -q INTERP; then exit 1; fi
