#!/bin/sh
# Shared by production and dev: static musl binaries, no runtime package manager.
set -eu
FFMPEG_VERSION=9.0.1
FFMPEG_SHA256=cf38e0e28c7e5605942c4a77755349b0145804a397af37eb1fb4c77cb237f635
X264_REV=b35605ace3ddf7c1a5d67a2eb553f034aef41d55
X264_SHA256=6eeb82934e69fd51e043bd8c5b0d152839638d1ce7aa4eea65a3fedcf83ff224
X264_MIRROR_SHA256=cd71a7515b0e9a012e1ac9b1f8415bebcaf6fc97d4db32286642ac4c0fbe24f9
# Keep the generic indexed path first; retained signed artifacts recover index rotation.
if ! apk add --no-cache build-base bash nasm pkgconf curl xz bzip2 gnupg zlib=1.3.2-r1 zlib-dev=1.3.2-r1 zlib-static=1.3.2-r1; then
  (
    zlib_arch=$(apk --print-arch)
    case "$zlib_arch" in
      aarch64)
        zlib_sha=1f77a542ac6634761b9fe38d8bdca19b154071dcfa25432e73f2bc8a0740a33f
        zlib_dev_sha=ed8acbc6e524accefbf62e236216c6f3f3141e45abc015caf05a4304ee11f0c6
        zlib_static_sha=9114059965de3a496021b98b1fffb6d0b25b105684db0c34ee4f88e499cf5005
        ;;
      armhf)
        zlib_sha=ff936d0aa98b7ab6a28aeaf13ef662df27ee224fe60dff74eff965a2e53c3db9
        zlib_dev_sha=bb45bf59c4a4bbaff5dc3c123139a544ab8a5b79a3338b2ff879f3d62cefe2b5
        zlib_static_sha=ab520d3e2aa32d2b363528542d57c756d5144aa357fe27f6ec30ed543bc160e3
        ;;
      armv7)
        zlib_sha=4a4077c3243a6dfd5e16df20ecbe100acd72ab24eab32c91790ec1eb32d23053
        zlib_dev_sha=cd24f88dee17e03533990d1c478d30b921440e0a725dbb73e04b4707b5bbf38e
        zlib_static_sha=82ccd90ce5825395eb57f3bdc06cfc307b0747e3147f2e860bfedf8bcc6c13b4
        ;;
      ppc64le)
        zlib_sha=da41fad1af7debdb5dcaa02599975b34c7f8f12be99cc9c3775ff8a2a0b916d1
        zlib_dev_sha=f4fd5ad81fb55d7556bba75cd7a724f74e0a796691a93c0413c7b9c821debcd6
        zlib_static_sha=cf3e37ec8adcb8742470166d6e5f0c5a820e0c1828d50c3092ce294afff2fd85
        ;;
      riscv64)
        zlib_sha=df0fed8bca1579423b1a78f3dde7e8130ea81908bcaacf179d452ae81cc4fdda
        zlib_dev_sha=e7b400d415591e5e9537c28ac488a175f8ef8dcd668f48e8ceb15707596c52cb
        zlib_static_sha=6b0280a4a83cca1738c5738f2fbb0409bb1d8b3e3dd85641e989b7be30895f92
        ;;
      s390x)
        zlib_sha=8f1bf7536ef6b1ced56b0cb61670bcaff81789402fa0c696e8d7ab5d824cceac
        zlib_dev_sha=ba06965a2a2d3f82ba851f4ff7b031548a0212942e56cecb82c00e8217210b46
        zlib_static_sha=3e6d1e2daa55ef0e45fdae17a0318c5c1d7ebc3d45e24630a462a2276397bbe5
        ;;
      x86_64)
        zlib_sha=63aeea03c15a2f9018f81805cfc8aa926bdf5cd68921f22149c2fbb5d0ee9f47
        zlib_dev_sha=50e47b778d894841a9c8ace7640edc1e585269963f29d6e4fdc008c8d6364890
        zlib_static_sha=1b7ce8d0515576599450144a5652eeba0e04e4b7342bba32cbc5a2a4ae256939
        ;;
      *) printf "%s\n" "The original APK install failed; no complete verified zlib 1.3.2-r1 fallback is available for this architecture." >&2; exit 1 ;;
    esac
    zlib_apk_dir=$(mktemp -d /tmp/postpilot-zlib-apks.XXXXXX)
    zlib_retained_dir="$(dirname "$0")/zlib-apks/$zlib_arch"
    trap 'rm -rf -- "$zlib_apk_dir"' EXIT
    trap 'exit 1' HUP INT TERM
    for package in zlib zlib-dev zlib-static; do
      retained_apk="$zlib_retained_dir/$package-1.3.2-r1.apk"
      if [ -f "$retained_apk" ]; then
        cp "$retained_apk" "$zlib_apk_dir/$package-1.3.2-r1.apk"
      elif ! wget -q -T 20 -O "$zlib_apk_dir/$package-1.3.2-r1.apk" "https://dl-cdn.alpinelinux.org/alpine/v3.24/main/$zlib_arch/$package-1.3.2-r1.apk"; then
        printf '%s\n' "Unable to download the pinned $package 1.3.2-r1 artifact." >&2
        exit 1
      fi
    done
    if ! printf '%s  %s\n' \
      "$zlib_sha" "$zlib_apk_dir/zlib-1.3.2-r1.apk" \
      "$zlib_dev_sha" "$zlib_apk_dir/zlib-dev-1.3.2-r1.apk" \
      "$zlib_static_sha" "$zlib_apk_dir/zlib-static-1.3.2-r1.apk" | sha256sum -c -; then
      printf '%s\n' 'Pinned zlib artifact checksum verification failed.' >&2
      exit 1
    fi
    # Normal apk installation verifies signatures using this architecture's existing keys.
    if ! apk add --no-cache build-base bash nasm pkgconf curl xz bzip2 gnupg \
      "$zlib_apk_dir/zlib-1.3.2-r1.apk" "$zlib_apk_dir/zlib-dev-1.3.2-r1.apk" "$zlib_apk_dir/zlib-static-1.3.2-r1.apk"; then
      printf '%s\n' 'Pinned zlib artifact signature verification or installation failed.' >&2
      exit 1
    fi
  )
fi
mkdir -p /media-build /opt/media/share/licenses /opt/media/share/sources
cd /media-build
curl -fL --connect-timeout 10 --max-time 120 --retry 3 -o musl.tar.gz https://distfiles.alpinelinux.org/distfiles/v3.24/musl-1.2.6.tar.gz
printf '%s  musl.tar.gz\n' '1adad96eddb3a2eb0cacb3e363b0046568925fcdd75cf8b0503f2139df1f693d64730779ca0ce8131b7624ab2d37f4247bb1d3393c523de6e30d2b1d7732555c' | sha512sum -c -
tar -xOf musl.tar.gz musl-1.2.6/COPYRIGHT > /opt/media/share/licenses/musl-COPYRIGHT.txt
cp musl.tar.gz /opt/media/share/sources/
curl -fL --connect-timeout 10 --max-time 120 --retry 3 -o /opt/media/share/licenses/zlib-LICENSE.txt https://raw.githubusercontent.com/madler/zlib/v1.3.2/LICENSE
# A transport fallback never bypasses the release's hash and signature checks below.
download_ffmpeg_input() {
  curl -fL --connect-timeout 10 --max-time 120 --retry 3 -o "$1" "$2"
}
if ! download_ffmpeg_input ffmpeg.tar.xz "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz"; then
  download_ffmpeg_input ffmpeg.tar.xz "https://distfiles.macports.org/ffmpeg/ffmpeg-${FFMPEG_VERSION}.tar.xz"
fi
if ! download_ffmpeg_input ffmpeg.tar.xz.asc "https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz.asc"; then
  if [ "$FFMPEG_VERSION" != '9.0.1' ]; then
    printf '%s\n' 'No retained detached signature for this FFmpeg version.' >&2
    exit 1
  fi
  cat > ffmpeg.tar.xz.asc <<'FFMPEG_9_0_1_SIGNATURE'
-----BEGIN PGP SIGNATURE-----

iQFMBAABCgA2FiEE/PmG6hXm4pOlZE8QtDIvBNZ2WNgFAmp767MYHGZmbXBlZy1k
ZXZlbEBmZm1wZWcub3JnAAoJELQyLwTWdljYeKYH+wdZITIvEOPDHcgPoGq8l8bo
u5VvRwdLV/kMcT5FKilLhrkc3w+ZVg6zxMmtA6/+6DcEVuYGD7qSLpybPkmTPQPk
APqdDTmURv+FJDhApIm0uRV1JX1+mmWbTlqztXjGi+q3JKf+m0jktXgAtLJUT85z
GuAA7PTAKjRSX7jGs9KlKttzMtzDVUomvgW/NIuOFDyC8CzzOkS3c/E8amrE1nXW
09LrnMEQniKDHGe7KFc+hzxoS5aVPfT+p/yDNrboLwX3w/+1XU1hj3lE7Y/7cISp
6DuuuzMMxMTTnF0m7klKv5JTcEJpAF659HBITK2kjzYdJ73KeH0UtFtTtAXoQpg=
=DfGV
-----END PGP SIGNATURE-----
FFMPEG_9_0_1_SIGNATURE
  printf '%s  ffmpeg.tar.xz.asc\n' 'b613a00005232a1245ace7080088781ac23a916119d3e5b0d6c042368eee0177' | sha256sum -c -
fi
if ! download_ffmpeg_input ffmpeg-signing-key.asc https://ffmpeg.org/ffmpeg-devel.asc; then
  download_ffmpeg_input ffmpeg-signing-key.asc https://raw.githubusercontent.com/FFmpeg/web/0745e67732776c48b69ca9441debb54ac9a891e5/htdocs/ffmpeg-devel.asc
fi
printf '%s  ffmpeg.tar.xz\n' "$FFMPEG_SHA256" | sha256sum -c -
gpg --batch --import ffmpeg-signing-key.asc
gpg --batch --status-fd 1 --verify ffmpeg.tar.xz.asc ffmpeg.tar.xz > signature-status
grep -q 'VALIDSIG FCF986EA15E6E293A5644F10B4322F04D67658D8 ' signature-status
# code.videolan.org answers some datacenter clients (GitHub's runners among them) with a small
# challenge page instead of the archive; the GitHub mirror serves the identical tree at the same
# revision, pinned by its own checksum. Whichever archive verified is the one shipped as source.
if curl -fL --retry 3 -o x264.tar.bz2 "https://code.videolan.org/videolan/x264/-/archive/${X264_REV}/x264-${X264_REV}.tar.bz2" &&
  printf '%s  x264.tar.bz2\n' "$X264_SHA256" | sha256sum -c -; then
  X264_ARCHIVE=x264.tar.bz2
  tar -xf x264.tar.bz2
else
  rm -f x264.tar.bz2
  curl -fL --retry 3 -o x264.tar.gz "https://github.com/mirror/x264/archive/${X264_REV}.tar.gz"
  printf '%s  x264.tar.gz\n' "$X264_MIRROR_SHA256" | sha256sum -c -
  X264_ARCHIVE=x264.tar.gz
  tar -xzf x264.tar.gz
fi
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
  --enable-protocol=file,pipe --enable-demuxer=mov,matroska,image2,image2pipe,wav,mp3,pcm_s16le \
  --enable-decoder=h264,hevc,vp8,vp9,mpeg4,mjpeg,prores,aac,mp3,opus,vorbis,alac,pcm_s16le,pcm_s24le,pcm_s32le,pcm_f32le,pcm_s16be,pcm_s24be,pcm_s32be,wrapped_avframe \
  --enable-parser=h264,hevc,vp8,vp9,mpeg4video,mjpeg,aac,mpegaudio,opus,vorbis \
  --enable-decoder=png --enable-encoder=libx264,aac,wrapped_avframe,pcm_s16le,png --enable-muxer=mp4,null,image2,image2pipe,pcm_s16le,wav \
  --enable-bsf=aac_adtstoasc,h264_mp4toannexb,hevc_mp4toannexb \
  --enable-filter=scale,setsar,fps,format,transpose,hflip,vflip,aresample,atempo,aformat,anull,null,trim,atrim,setpts,asetpts,select \
  --enable-filter=crop,overlay,xfade,acrossfade,concat,volume,apad,settb,asettb,fade,afade,loudnorm,loop,vfrdet,amix,adelay \
  --enable-indev=lavfi --enable-filter=color,sine,anullsrc
make -j2
make install
cp COPYING.GPLv2 COPYING.LGPLv2.1 /opt/media/share/licenses/
cp /media-build/ffmpeg.tar.xz /media-build/ffmpeg.tar.xz.asc "/media-build/${X264_ARCHIVE}" /opt/media/share/sources/
apk info -v | grep -E '^(musl|zlib)' > /opt/media/share/licenses/alpine-components.txt
/opt/media/bin/ffmpeg -version
/opt/media/bin/ffprobe -version
# Readelf confirms no dynamic loader is required by distroless/static.
if readelf -l /opt/media/bin/ffmpeg | grep -q INTERP; then exit 1; fi
if readelf -l /opt/media/bin/ffprobe | grep -q INTERP; then exit 1; fi
