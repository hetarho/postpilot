# Deterministic clip rendering

`render-tools.sh` builds the current official resvg release **0.48.1** from
[its source tag](https://github.com/linebender/resvg/releases/tag/v0.48.1).
Archive SHA-256: `40dafea6b4b9d01e9d28b6d49f1e912daf3e9055676ad9179a5a2db6e7386945`.
The Rust Alpine base is pinned by multi-platform digest
`sha256:1716b3aa042d735f4566d14dc54e8037de9d69556e2d5dd58131d93a613d173d`.
The upstream Cargo.lock is used unchanged. The static executable, MIT/Apache-2.0
notices, source archive and exact dependency sources/notices ship in the image.
Readelf rejects a dynamically linked executable. Both development and production
copy the same build output; production remains distroless nonroot.

The renderer supplies `--skip-system-fonts --use-font-file` explicitly. It queries
shaped glyph bounds with `--query-all` once per caption, selecting only grapheme
breaks and at most two lines. The final SVG uses the same font weight and measured
bounds. The minimum-size failure is `CLIP_COPY_TOO_LONG`; unsupported glyphs and
control characters are invalid input, never substituted images or fonts.
Pretendard is pinned with its OFL notice in `backend/assets/fonts/pretendard`.
Style sizes, weights, padding, radius and accents match the T070 preview contract.

The three product-owned safe regions are deliberately conservative, not a claim
about an exact Naver overlay layout. Captions can only occupy top, center or bottom
within the region; an AI avoid region chooses among those same positions. Source
focal points drive cover-cropping, never stretch-to-fit. Times are integer ms until
the binary boundary. The plan duration is `sum(cut durations) - 200*(cuts-1)`.
Cumulative frame rounding avoids per-cut rounding drift at constant 30 fps.
An omitted original-audio gain defaults to 1; an explicit normalized zero mutes
the cut. Sources without audio get silence only when another selected cut has
audio. An entirely silent plan has no unnecessary audio track.

Source loading is callback-scoped and sequential; render intermediates are bounded
by the approved 15–90 second timeline. The output remains inside the media workspace
until the worker uploads it; `WithWorkspace` removes it on every terminal path.
No uploaded source, caption text or owner id enters FFmpeg filter syntax.

Official references checked on 2026-09-10:

- [resvg CLI](https://github.com/linebender/resvg/blob/v0.48.1/crates/resvg/src/main.rs): explicit fonts, bbox queries and PNG conversion.
- [Cargo target flags](https://doc.rust-lang.org/cargo/reference/config.html#buildrustflags): target-only static linking keeps host proc macros loadable.
- [FFmpeg filters](https://ffmpeg.org/ffmpeg-filters.html): cover scaling, overlay, trim, xfade and acrossfade.
- [Uniseg](https://pkg.go.dev/github.com/rivo/uniseg): Unicode grapheme boundaries; latest resolver pinned v0.4.7 in go.mod/go.sum.
- [SFNT](https://pkg.go.dev/golang.org/x/image/font/sfnt): fixed-font glyph coverage; latest resolver pinned x/image v0.46.0.

## Naver acceptance gate

On 2026-09-10 the [Clip creation help](https://help.naver.com/service/30048/contents/24422?lang=ko&osType=COMMONOS)
describes the PC uploader and a 7-hour/8-GB upper limit. The separate
[Naver TV Clip help](https://help.naver.com/service/17223/contents/23504?osType=COMMONOS)
describes a 3-minute/8-GB limit and recommends vertical video without stating that
horizontal video is forbidden. Our 15–90 second H.264/AAC MP4s are within both time
limits; these pages do not themselves prove every ratio is accepted by the picker.
On 2026-09-10 the owner confirmed that Naver web accepted all three synthetic
output fixtures and supplied a screenshot listing `vertical.mp4`,
`horizontal.mp4` and `square.mp4`, each with a 0:15 duration. No format/ratio
rejection was reported. This verifies T074's owner-assisted acceptance gate;
the agent did not publish, register or submit anything. Future manual checks
must likewise avoid publish/register/submit and cannot be replaced by the
documentation or local media checks alone.
