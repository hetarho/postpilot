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

Two faces ship, and the renderer passes both as separate `--use-font-file`
arguments with system fonts off. Pretendard Variable sets every role but two;
Paperlogy 8 ExtraBold sets `t.hook` and `t.title`, which is 크게 강조's face
(CDS-17, CDS-25). Each is pinned by exact size and SHA-256 at construction, so a
swapped file fails the renderer's constructor rather than a render, and the
glyph-coverage check runs against the face the text will actually be set in —
`checkCopy` reads the role's own face, and a Korean glyph Paperlogy lacks is
`CLIP_INVALID_INPUT`, never a silent substitution. CDS-17's Noto Sans KR fallback
never fires here: resvg runs with system fonts disabled and coverage is refused
before rasterisation, so that fallback is a frontend preview concern only.
Paperlogy comes from the designer's own distribution page
([freesentation.blog/paperlogyfont](https://freesentation.blog/paperlogyfont)) and
the release archive it links; the archive and file checksums, the OFL notice and
the family name inside the file are recorded in
`backend/assets/fonts/paperlogy/README.md`. Only 8 ExtraBold is bundled, because
CDS uses no other Paperlogy weight.

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
  the accent, as a `tspan` inside the line so the run stays shaped as one, and
  that word turns white on a bright ground (see the sampler below). Set in
  Paperlogy 8 ExtraBold.
- `mark` — no plate: white `t.mark` 60/800 with a 4 px stroke and the same shadow,
  and the keyword's `underline.mark` highlight (accent α0.9, 0.42em tall, raised
  0.28em above the baseline, 6 px past each side) drawn behind the text.

## Cards, scrims and brightness

Two cards frame the clip, each its own PNG plate on the same overlay chain as a
copy, so their timing reuses the fade filters and nothing else moves. Both are
`ink.900s` α0.88 at `radius.card` 24 with 40 px padding and `shadow.card`, and
both draw only the owner's answers and the preset's own phrases — never model
text, except the hook sentence itself.

- hook card (CDS-28) — over the FIRST cut, 0 to 1500 ms, present from frame 0
  with a 200 ms fade-out. It stacks the preset's category chip (`t.label` on an
  accent α1.0 pill with `#111` ink), the hook (`t.hook` 84 on 9:16, 72 on 16:9,
  76 on 1:1, broken into at most two lines of nine at a word boundary) and the
  상호 answer (`t.body` `text.muted`). The original audio dips 6 dB for the
  card's own window. No hook sentence or no 상호 means no card at all: the first
  frame stays real footage for the thumbnail either way.
- ending card (CDS-29) — over the LAST cut from end − 2500 ms, 200 ms fade-in and
  no fade-out. It stacks the 상호 answer, the 위치 answer (`t.label`
  `text.muted`), one 가격 or 메뉴 answer when given (`t.caption`, with the
  preset's price note) and the resolved CTA phrase in the accent. A project
  without 상호 renders no card and the manifest says so by carrying none.

Per ratio the card is the ratio's own width centred on the frame and on its own
card line: 9:16 x 144–936 centred y 840 (hook) and y 1040 (ending), 16:9 width
1120 centred y 540, 1:1 width 880 centred y 540 and y 560. On 9:16 that plate is
sixteen pixels wider than CDS-9's safe area on each side, which CDS-28 states
outright; the plate may sit there but its TEXT may not, so the verifier exempts
only the `card` kind from V1 and holds every line inside the safe area — the
40 px padding is what makes both true at once. Copy and chips yield to a card as
they do to the badge: the composer receives the card regions through
`CardElements` and drops any anchor that collides, and V7 refuses a manifest
where a caption or chip would show underneath one.

Brightness sampling exists for the two unplated styles only (CDS-16, CDS-44). A
plated element needs none: `ink.900` at α0.72 under white text stays above the
floor even over a white frame. For `bold` and `mark` the renderer takes the
first, middle and last frame of the copy window from the cut's own source —
`ffmpeg -ss <cut start + offset> -frames:v 1 -vf <the render's own cover chain>`
to PNG — inside the SAME source callback that renders the cut, so no second
download happens. The frames are decoded with `image/png` and averaged in Go
rather than parsed out of `signalstats`, so the luminance formula is one tested
function and no subprocess text is scraped. The crop is the copy's own region,
taken through the render's scale-and-crop so the pixels measured are the pixels
the viewer sees. `L` is the WCAG 2.1 relative luminance of the region's mean
colour and `σ` its deviation across the three frames. `L ≥ 0.6` or `σ ≥ 0.25`
adds `scrim.top` (0, 250, 1080, 310) or `scrim.bottom` (0, 1040, 1080, 380) —
260/200 on the other two ratios — at the copy's own anchor, riding the copy's own
plate so it shares its window and both fades. `L ≥ 0.6` also turns 크게 강조's
accent word white; 형광펜's accent is the marker stroke behind white text, so it
is left alone — whitening it would paint white on white. A scrim is never placed
under a plated element, and the verifier refuses one that is.

V3 is that sample's own check: every text is measured against its EFFECTIVE
background at WCAG 2.1's 4.5:1. For a plated element that is the plate over the
sampled or assumed ground; for a card line the card; for the category chip the
accent pill. For an unplated style it is the `stroke.dark` α0.85 outline CDS-25
and CDS-26 give it, composited over the scrim-washed footage — the stroke is the
mechanism those styles use, and it holds white text at 13.2:1 over a white frame
where the bare footage would be 1:1. A pairing still under the floor falls back
to 깔끔하게 at the same anchor when the compiler chose the style (a plan that
still carries one decision per cut has not been through a person) and is refused
`CLIP_LAYOUT_CONTRAST` when a person chose it. Because the whole manifest changes
once a ground is known, it is verified a second time after the cuts are rendered
and before they are joined.

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
(`CLIP_LAYOUT_*`). V3 contrast is implemented on the sampled ground, as described
above. V4, V8 and V11 cover components the manifest does not carry yet; V12 is checked
on the DELIVERED file rather than on the manifest (below). The
kinds a manifest may name are `copy`, `plate`, `bar`, `highlight`, `badge`,
`chip`, `card`, `chip-category` and `scrim`; anything else is refused as
`CLIP_LAYOUT_KIND`. An element carries a copy style exactly when it belongs to a
caption, which is what tells the verifier whether to hold it to a style's own
table and to the two permitted motions, or to treat it as furniture the design
system typeset itself.

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
the binary boundary.
Cumulative frame rounding avoids per-cut rounding drift at constant 30 fps.

## Two copies on one cut

A cut usually carries one copy; a cut of 4.0 s or more whose sentence DESCRIBES
may also state the number it leads to, 120 ms after the description has left and
never beside it (CDS-43). `Cut.Copies` is therefore a list of one or two, and
every per-caption rule — the style's limits, the exposure minimum, the anchor
step, the style frequency — is read on the copy rather than on the cut. A
manifest element carries the copy it belongs to, so the verifier can tell two
captions of one cut apart; a plan stored before this is read back as the one copy
it was (stored version 3).

The compiler never asks the model for a second sentence: it lifts the number out
of the words it already has. CDS-39 reads a number FIRST, so a sentence that
states one is classified `NUM` and is never the description CDS-43 splits — in
practice the second copy is the `short_text` the model wrote beside the
description, and the clause walk is the fallback for a sentence the classifier
reads as a description anyway. The two windows come out of the one CDS-27 window,
split by what each sentence earns under CDS-41 and parted by the same 120 ms
lead. The renderer draws one plate per copy, each enabled only in its own window.

## Transitions

Every cut carries the transition INTO it (`TransitionMS`), and the first cut of a
plan always carries 0: a clip does not fade in from nothing. CDS-36 admits three
values and nothing else — `0` a hard cut, `200` the fade a scene change earns and
`300` a fade-through-black that validation accepts so a manual plan may use it
but no control offers and no compiler chooses. The plan duration is therefore
`sum(cut durations) - sum(transitions)`, never one fade times the boundaries.

The compiler assigns them from what the observer reported: a 200 ms fade wherever
the `scene` of two consecutive cuts differs, a hard cut everywhere else, and then
the fades beyond 40 % of the BOUNDARIES are cleared, keeping the earliest ones —
they sit where the clip is still establishing its scenes. It also holds every cut
to CDS-37's 1.2–6.0 s (a `food` close-up to 4.0 s), trimming what is over and
growing what is under inside its own observed segment; a cut with no room left in
either direction is refused as `plan_cut_length`. CDS-41's exposure extension may
carry a cut past that ceiling, but only as far as the copy's own minimum needs.

In the filter graph a boundary with a transition is an `xfade` (`fade`, or
`fadeblack` at 300 ms) whose offset is the cumulative frame count less the
overlap, and a hard cut is a `concat` — a join, not a zero-length dissolve that
would resample the boundary. The binary composition tree carries each branch's
leading transition up with it, so a merged node overlaps by its own right-hand
boundary and leads in with its left-hand one.

## Audio

An omitted original-audio gain defaults to 1; an explicit normalized zero mutes
the cut. Sources without audio get silence only when another selected cut has
audio. An entirely silent plan has no unnecessary audio track.

The track is assembled once on the video's own timeline and written uncompressed
beside the cuts, so the measurement pass and the final encode read one identical
track. Each boundary cross-fades: where a transition overlaps the picture the
audio `acrossfade`s across that same overlap and the two stay locked, and a HARD
cut — which has no overlap to spend — takes CDS-35's 60 ms cross-fade as a
matched `afade` pair either side of the seam before the `concat`. An `acrossfade`
there would consume 60 ms of track the picture does not, and every later cut
would run early by more than a frame.

Loudness is CDS-35's two passes. The first measures the assembled track
(`loudnorm=I=-16:TP=-1.5:LRA=11:print_format=json`, read from FFmpeg's stderr,
which is the only place a filter prints its report); the second applies the
measured values with `linear=true` inside the single final encode, so the clip is
still produced by one encode. A single dynamic pass drifts on a file this short.
A track of digital silence measures −inf LUFS and is delivered as it is: no gain
makes silence −16.

V12 is read off the delivered file: the canvas, 30 fps, H.264 High (named on the
final encode rather than left to libx264's default), 48 kHz AAC, and a third
`loudnorm` measurement pass whose integrated loudness must sit within 1 LU of
−16. A miss fails the render exactly as a codec or format mismatch does.

Source loading is callback-scoped and sequential; render intermediates are bounded
by the approved 15–90 second timeline. The output remains inside the media workspace
until the worker uploads it; `WithWorkspace` removes it on every terminal path.
No uploaded source, caption text or owner id enters FFmpeg filter syntax.

Official references checked on 2026-09-10:

- [resvg CLI](https://github.com/linebender/resvg/blob/v0.48.1/crates/resvg/src/main.rs): explicit fonts, bbox queries and PNG conversion.
- [Cargo target flags](https://doc.rust-lang.org/cargo/reference/config.html#buildrustflags): target-only static linking keeps host proc macros loadable.
- [FFmpeg filters](https://ffmpeg.org/ffmpeg-filters.html): cover scaling, overlay, trim, xfade, concat, afade and acrossfade.
- [FFmpeg loudnorm](https://ffmpeg.org/ffmpeg-filters.html#loudnorm): the two-pass EBU R128 workflow and its JSON report.
- [Uniseg](https://pkg.go.dev/github.com/rivo/uniseg): Unicode grapheme boundaries; latest resolver pinned v0.4.7 in go.mod/go.sum.
- [SFNT](https://pkg.go.dev/golang.org/x/image/font/sfnt): fixed-font glyph coverage; latest resolver pinned x/image v0.46.0.
- [Paperlogy](https://freesentation.blog/paperlogyfont): the designer's own release page and archive, read on 2026-09-11; checksums in `backend/assets/fonts/paperlogy/README.md`.
- [WCAG 2.1 contrast minimum](https://www.w3.org/TR/WCAG21/#contrast-minimum): the relative-luminance and contrast-ratio definitions V3 computes.
- [image/png](https://pkg.go.dev/image/png): the standard-library decoder the brightness sampler reads its frames with.

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
