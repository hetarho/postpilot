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
The four style ids are clean, memo, bold and mark; every approved set keeps clean,
because every design-system fallback lands on it. Each is drawn only from design
tokens, never a literal:

- `clean` — an `ink.900` plate at `radius.box`, an 8 px accent bar clipped to the
  plate's own rounded rect at its left inner edge, 22/32 padding with the bar
  inside a 40 px left inset, `t.body` 56/700 white, at most two lines.
- `memo` — a `paper.50` plate, one 14 px accent dot at the top left, 18/28 padding
  with 54 px on the left, `t.caption` 44/600 `text.ink`, exactly one line.
- `bold` — no plate: white `t.title` 72/800 over a 6 px round-joined `stroke.dark`
  painted under the fill, plus the `shadow.text` drop shadow. One word may take
  the accent, as a `tspan` inside the line so the run stays shaped as one.
  Pretendard 800 stands in until Paperlogy is bundled.
- `mark` — no plate: white `t.mark` 60/800 with a 4 px stroke and the same shadow,
  and the keyword's `underline.mark` highlight (accent α0.9, 0.42em tall, raised
  0.28em above the baseline, 6 px past each side) drawn behind the text.

The keyword is a caption field, never a marker inside the text, and its highlight
starts at the measured advance of the prefix before it — `--query-all` on the real
font, not an estimate. Sub-pixel kerning between the prefix's last glyph and the
keyword's first is not modelled, which a rectangle can absorb and a glyph could
not; that is why `bold` colours its word in place instead of redrawing it.

Letter-spacing is the role's tracking in px at the rendered size, and the same
tracking scaled to 100 px is used when measuring, so the fit and the final SVG
agree. The fit loop searches only between a role's nominal size and its minimum
(body 48, caption 40, title 64, mark 52); below the minimum the copy is refused
with `CLIP_COPY_TOO_LONG`, never shrunk further.

Copy enters at its window's start with a 180 ms alpha fade that settles 12 px
upward — `overlay=x=0:y='12*pow(1-min(1,max(0,(t-T0)/0.180)),3)'`, an ease-out
cubic on the one looped plate image — and leaves with a 120 ms alpha fade and no
movement. Both are `fade=...:alpha=1` on the plate input; there is no second
rasterisation per frame and no other motion. A cut's default window is inset
120 ms at each end so no text straddles a transition; an explicit window is used
exactly as given.

The renderer emits a manifest of every element it places (kind, style, anchor,
region in canvas pixels, output-timeline window, font size, fill, background and
the motion values) and `design.Verify` gates the render on it BEFORE any source
byte is fetched and before FFmpeg runs, so a plan that breaks the design system
costs neither a download nor an encode. It implements V1 safe area, V2 size
floors, V5 lines and characters, V7 overlap between different cuts whose windows
meet, V9 the two permitted motions, V13 one anchor step between consecutive cuts
and V14 style frequency, and names the failing check as its own stable reason
(`CLIP_LAYOUT_*`). V3 contrast needs frame sampling and is a documented gap until
the brightness sampler lands; V4, V6, V8, V10, V11 and V12 cover components the
manifest does not carry yet.

The three safe areas and every placement number come from one embedded
configuration file, `internal/clip/design/design.json`, which the frontend mirrors
byte for byte; nothing here is a literal. 9:16 is the cross-platform intersection
SA-C (64, 250, 856, 1170), 16:9 is (96, 72, 1728, 936) and 1:1 is (64, 72, 952,
936). A caption resolves one of four vertical anchors (top, upper_mid, lower_mid,
bottom) and one alignment (center, left, right) to a plate region, and a region
that would leave the safe area by any pixel is refused rather than nudged — 9:16's
safe area is deliberately off-centre, so a full-width centred plate misses it. An
AI avoid region chooses among those same four anchors. Source
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
