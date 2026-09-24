# Deterministic clip rendering

## API and execution-worker contract

Preparation and final rendering use an execution-only `cmd/media-worker` process.
The API keeps SQLite, authorization, planning, providers, credits and publication.
Workers pull authenticated protocol 1 leases (`cpu-v1`, `assets-v1`), use short-lived
private object grants and return measured immutable receipts. No shared DB or work
directory is required. API restart preserves valid leases and accepted handoffs;
retries cannot replay an uncertain paid provider call. Cancellation and revision
changes fence claims/publication, with acknowledgement or lease expiry before cleanup.

`MEDIA_ACCEL=cpu` and `auto` currently select the validated CPU profile; strict
`nvenc` refuses activation pending CLIP-163 and real NVIDIA acceptance. CPU defaults
remain one job, one encode thread and two decode threads. The existing 8 GiB workspace,
512 MiB prepared-copy and per-input limits remain bounded in the owning context.
Deployment limits are explicit, never calculated from detected core count.

`/media-worker manifest` and `/api media-manifest` inspect actual tool/font/overlay
capabilities offline, without opening the application DB or loading providers.
Worker `status` adds an authenticated compatible API probe; `health` also requires
the running worker's kernel workspace lock. SIGTERM stops new claims and drains for
`MEDIA_DRAIN_TIMEOUT` before cancelling/reaping active subprocesses. Container stop
grace must exceed drain by at least 15 seconds; the default pair is 30/45 seconds.

Independent API/worker SHA pins carry shared protocol/renderer/asset labels and an
asset-input SHA-256. Runtime manifests record actual architecture, binary hashes,
font hashes and bounded execution settings. Git revisions may differ when contracts
match. `scripts/media-assets.mjs --update` refreshes the OCI asset-input label after
an intentional asset edit; `pnpm test:dev` rejects a stale label.

The supported rollback floor is API media rollback version 1 (OCI
`org.postpilot.media.rollback-safe=1`, including migrations through 0082 and durable
media recovery). Pre-worker APIs are refused: their boot sweep can fail parked jobs.
The rollout never deletes wait records, rewinds SQLite or mounts the DB in a worker.
See [DEPLOY.md §8](../../DEPLOY.md#8-media-worker-deployment) for layout selection,
bootstrap, private ingress, drain, compatibility checks and saved-env rollback.

## Pinned renderer assets

Overlay drawing is loaded from versioned SVG assets. See the
[preset authoring and deployment guide](../../docs/design/overlay-presets.md)
for file layout, dynamic text fields and `CLIP_OVERLAY_DIR`. The embedded default
assets preserve the existing output; layout and visibility policy stay in the
design configuration and media adapter.

`render-tools.sh` builds the current official resvg release **0.48.1** from
[its source tag](https://github.com/linebender/resvg/releases/tag/v0.48.1).
Archive SHA-256: `40dafea6b4b9d01e9d28b6d49f1e912daf3e9055676ad9179a5a2db6e7386945`.
The Rust Alpine base is pinned by multi-platform digest
`sha256:1716b3aa042d735f4566d14dc54e8037de9d69556e2d5dd58131d93a613d173d`.
The upstream Cargo.lock is used unchanged. The static executable, MIT/Apache-2.0
notices, source archive and exact dependency sources/notices ship in the image.
Readelf rejects a dynamically linked executable. Both development and production
copy the same build output; production remains distroless nonroot.

Four faces ship in five files, and the renderer passes every one of them as its
own `--use-font-file` argument with system fonts off (CDS-17). Wanted Sans
Variable sets every role but two; Paperlogy 8 ExtraBold sets `t.hook` and
`t.title`, which is 크게 강조's face (CDS-25); Jua and NanumMyeongjo, at 400 and
800, are the remaining caption faces a style may name. Wanted Sans replaced
Pretendard Variable on 2026-09-21 (owner decision): its `wght` axis starts at
400 rather than 250, which 키노트 now asks for, and it maps 2,345 fewer
characters — all Hangul and Latin-1 intact, but no Cyrillic, Greek, kana,
full-width forms or CJK compatibility units, each of which the coverage check
now refuses rather than draws (`kg` sets, `㎏` does not). Each file is pinned by
exact size and SHA-256 at construction, so a swapped one fails the renderer's
constructor rather than a render, and the glyph-coverage check runs against the
face the text will actually be set in — a caption reads its STYLE's face
(CDS-18), not its role's. CDS-17's Noto Sans KR fallback never fires here: resvg
runs with system fonts disabled, so that fallback is a frontend preview concern
only.

Wanted Sans comes from the Wanted Lab release archive
([github.com/wanteddev/wanted-sans](https://github.com/wanteddev/wanted-sans/releases/tag/v1.0.3));
the two Google Fonts faces come from the `ofl/` trees of google/fonts; Paperlogy
comes from the designer's own distribution page
([freesentation.blog/paperlogyfont](https://freesentation.blog/paperlogyfont)) and
the release archive it links. Each face's `README.md` under `backend/assets/fonts`
records its checksums, its OFL notice and the family name inside the file. Only
the weights CDS names are bundled: one Paperlogy, one Jua, two NanumMyeongjo.
NanumMyeongjo is the one face whose two weights are two files, and they answer to
one family name — ExtraBold declares `NanumMyeongjo` as its TYPOGRAPHIC family,
its legacy name being the unparsable `NanumMyeongjoExtraBold`.

Coverage is read from each file once at construction, over the ranges Korean
captions are written in, so the per-caption check is a set lookup. Jua carries
2,367 of the 11,172 Hangul syllables; a caption whose style names it and whose
text it cannot set is drawn in the default style instead, with a CLIP-108 notice
naming that caption (CDS-84). The swap is one caption's, never the project's, and
no glyph is ever taken from another family. A glyph even the default face lacks
is `CLIP_INVALID_INPUT`, as before.

The renderer supplies `--skip-system-fonts --use-font-file` explicitly. It queries
shaped glyph bounds with `--query-all` once per caption, selecting only grapheme
breaks and at most two lines. The final SVG uses the same font weight and measured
bounds. The minimum-size failure is `CLIP_COPY_TOO_LONG`; unsupported glyphs and
control characters are invalid input, never substituted images or fonts.

## The approved caption style set

The set lives in `internal/clip/design/caption_style.go` rather than in
`design.json`, because a style carries a filter graph and generated geometry a
constants file cannot express; the numbers the renderer and the preview must
agree on to the pixel stay in `regions.caption`, which both sides read (CDS-80,
CDS-83). Each style fixes its face, its type role, its colour treatment and its
motion, and says whether one rasterisation covers its whole interval (static) or
it draws a layer per output frame (sequence). `크게 강조` is the default, and an
empty selection resolves to it alone (CDS-25).

Two rules hold across every style. Contrast is measured against `stroke.dark`
only where a style actually strokes in it (CDS-44): a coloured outline is
decoration, not a backing the text can be read off, so such a style is measured
against its own sampled ground. And every filter a style emits declares
`color-interpolation-filters="sRGB"`, because resvg computes filters in
linearRGB by default and the colour then differs from what a browser draws —
which CDS-83 forbids. `feDisplacementMap` is admitted to no style: resvg 0.48.1
places its result at the wrong offset.

The retired style ids `clean`, `memo`, `mark` and `simple` are still read off
stored plans and render in the default treatment they already rendered in;
nothing writes one.

### Static and sequence rendering

A static style rasterises once and the overlay chain loops that one plate for
the caption's whole interval, fading it in and out and settling it upward with
`fade` and an `overlay` y expression. That path is unchanged.

A sequence style draws one PNG per OUTPUT frame of its own interval into a
directory of the attempt's workspace, and the chain reads them back with
`image2` at the output frame rate, shifted to the caption's start with `setpts`
and overlaid at the crop origin the frames were drawn in. A window that opens
mid-caption resumes the sequence at the frame it reaches, through
`-start_number`, rather than replaying an entrance the owner already saw. It
carries no `loop`
and no `fade`: its motion is already in every frame, which is the whole reason
the extra rasterisations are paid for. Frames enter through `image2` rather than
`image2pipe` because the chain already takes several inputs — feeding pipes
concurrently makes scheduling and partial-failure retry much harder, while files
let one failed caption re-render alone — and decoder threads stay limited on the
sequence input exactly as they are on a single-frame one, because an unbounded
image demuxer can leave the scheduler waiting after an overlay stops consuming.

Each frame is cropped to the caption's own box grown by the style's declared
bleed, never to the whole canvas: a full-canvas layer would reserve about ten
megabytes of workspace budget for every frame of every caption. The frames count
against CLIP-33's temporary-disk accounting like every other intermediate — the
workspace total descends into the one directory it holds — and they are deleted
as soon as the overlay pass that read them is encoded, so a long sequence never
sits beside the output it helped make. Every frame is a pure function of the
plan and the frame index, so one plan delivers one clip byte for byte.

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

Per ratio the card is centred on the shared copy grid: 9:16 x 96–888
(midpoint 492), centred y 840 (hook) and y 1040 (ending); 16:9 width 1120
centred at (960, 540); 1:1 width 880 centred at (540, 540)/(540, 560).
The padded text band stays inside the safe area. The compiler can use the card
regions to select a free caption anchor; any remaining overlap is advisory.

Disclosure and information now share a top edge (80 on 9:16, 112 on the other
ratios) and a row height derived from font sizes and padding. Each text role is
measured with its own weight and tracking, including a shortened value after
truncation. The SVG adapter centres the actual glyph bounds vertically and
accounts for the left bearing. Chips reserve the badge width and one stack gap
before fitting a value, including long disclosure phrases. Vertical cards and
centred captions use the same x 492 midpoint between LEFT 96 and RIGHT 888.

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
adds `scrim.top` (0, 40, 1080, 310) or `scrim.bottom` (0, 1040, 1080, 380) —
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
where the bare footage would be 1:1. A pairing still under the floor is rung 1
of the repair ladder below — 깔끔하게 at the same anchor where that style may
stand, else at its own — when the compiler chose the style (a plan that still
carries one decision per cut has not been through a person) and is refused
`CLIP_LAYOUT_CONTRAST` when a person chose it. Because the whole manifest changes
once a ground is known, it is verified a second time after the cuts are rendered
and before they are joined.

There are two verify points and they behave differently on purpose. BEFORE any
download, a compiled plan whose manifest fails a blocking check walks CDS-55's
caption repair ladder: the failing caption's style falls back to 깔끔하게 (whose plate
answers V2, V3 and V5 by itself), then its anchor falls back to the style's own
default, then the copy is dropped and the cut shows its footage; the plan is laid
out and verified again after every rung, at most one ladder per caption and one
layout per rung, so a plan of N captions costs at most 3N layout passes and no
source byte. The verifier names WHICH caption failed (cut, copy) — for the
sequence rules V13 and V14 the later of the pair, the one whose style or anchor
can change without invalidating what came before — and a failure the design
system's own furniture caused (badge, chip, card: the furniture slot), other than
overlap, fails at once as a renderer defect. Every rung taken is recorded on the cut's composition
(`contrast`, `style`, `anchor`, `dropped`) so step ② can say what happened. A plan
a PERSON corrected never walks the ladder: blocking failures name their check
and the plan is never silently moved (CDS-52). AFTER the cuts are rendered the
same delivery checks run: a blocking failure still fails because repairing it
would mean re-rendering.

Overlap is advisory at BOTH validation points (CDS-56), for generated and manual
plans alike. Captions, chips, badges and cards may share pixels and timing without
stopping layout or rendering. In particular, chips on adjacent cuts overlap during
a crossfade; that previously caused a furniture-slot failure no caption repair
could fix. An overlap alone now preserves the words, position and timing so the
owner can preview/download the clip and revise it in step ②. `design.Verify` and
`VerifyApproved` retain the strict diagnostic; `clip.VerifyLayout` uses
`design.VerifyRenderable`, which skips only V7 and still runs every other check,
including the sequence checks AFTER the overlap check. Swallowing the first
overlap error would incorrectly hide those later failures.

`TestRenderOriginalsOverlap` is the offline regression for the eight short MP4s
reported with this failure. It uses all eight originals in a 30-second vertical
plan with chips across fades and captions under cards, then renders it as both a
generated and a manual plan. `CLIP_ORIGINALS_DURATION_MS=20000` exercises the
20-second target of the production failure. Set `CLIP_ORIGINALS_DIR` to a read-only source mount
and `CLIP_ORIGINALS_OUTPUT` to a separate writable artifact mount when running
`/media.test -test.run=^TestRenderOriginalsOverlap$ -test.v -test.timeout=15m`
in the `media-smoke` image, using production's 15-minute operation timeout.
T117's 2 GiB success did not qualify the shared 909 MiB production host; T118
requires the originals to complete at 512 MiB / 2 CPU with swap disabled. The separate synthetic
release gate still runs at 1 GiB / 2 CPU. It makes no model calls and writes `generated.mp4`
and `manual.mp4` only to the artifact mount. Ordinary test and image-build runs
skip it; no user footage is included in the repository or image.

PNG layers are decoded exactly once, with decoder threads explicitly limited on
every input. The fixed overlay repeats that frame; animated copies and cards use
FFmpeg's `loop` filter with a one-frame cache and exactly `cutFrames - 1` repeats.
This preserves every alpha-fade frame while avoiding repeated PNG decoding and
unbounded image-demuxer inputs. An infinite image input can leave the scheduler
waiting after an overlay has stopped consuming frames; limiting its decoder
threads alone reproduces that stall. The sampler also bounds its input decoder
and simple filter threads, separately from complex-filter threads.

Lossless 4:4:4 composition nodes use x264's `ultrafast` preset at CRF 0. They
trade temporary file size for less compression work; the same per-file and
workspace limits still apply. Cut and final output encodes retain `veryfast`,
CRF 20, H.264 High and yuv420p. The decoded pixels of an intermediate remain
lossless regardless of its compression preset.

The delivered audio is measured before the final format probe. If AAC or the
normaliser's dynamic fallback moved a short track outside ±1 LU, up to two
audio-only corrections apply the measured gain difference, bounded by measured
true-peak headroom. Each correction reuses the original assembled PCM and
normalisation filter, encodes a fresh AAC track and copies the finished H.264
video without encoding it again. The corrected file must pass the original
loudness and format gates. The 20-second originals fixture exercises this path
with its initial -14.97 LUFS measurement; the accepted tolerance remains ±1 LU.

Worker logs record stage changes, stage elapsed milliseconds and total elapsed
milliseconds. A media failure additionally carries code-owned operation and class
labels (for example `render_cut` / `timeout` or `process_signal`) and its elapsed
time; paths, captions, raw stderr and provider bodies remain excluded. These
diagnostics preserve wrapped error identities and do not change settlement or
the existing failed-attempt contract.

Official option references: [FFmpeg input option scope and thread controls](https://ffmpeg.org/ffmpeg.html)
and [the single-frame loop filter](https://ffmpeg.org/ffmpeg-filters.html#loop).

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
the motion values) and `design.VerifyRenderable` gates the render on it BEFORE
any source byte is fetched and before FFmpeg runs, so a plan that breaks the design system
costs neither a download nor an encode. It implements V1 safe area, V2 size
floors, V5 lines and characters, V9 the two permitted motions, V13 one anchor step
between consecutive cuts
and V14 style frequency, and names the failing check as its own stable reason
(`CLIP_LAYOUT_*`). V7 overlap remains a diagnostic only. V3 contrast is implemented on the sampled ground, as described
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
byte for byte; nothing here is a literal. 9:16 uses the design
bounds (64, 40, 856, 1380), 16:9 is (96, 72, 1728, 936) and 1:1 is (64, 72, 952,
936). A caption resolves one of four vertical anchors (top, upper_mid, lower_mid,
bottom) and one alignment (center, left, right) to a plate region, and a region
that would leave the safe area by any pixel is refused rather than nudged — 9:16's
safe area is deliberately off-centre; captions now follow its shared content grid. An
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
measured values with `linear=true` inside the final encode. A single dynamic
pass drifts on a file this short.
A track of digital silence measures −inf LUFS and is delivered as it is: no gain
makes silence −16.

V12 is read off the delivered file: the canvas, 30 fps, H.264 High (named on the
final encode rather than left to libx264's default), 48 kHz AAC, and a third
`loudnorm` measurement pass whose integrated loudness must sit within 1 LU of
−16. A miss first gets at most two audio-only corrections from the assembled PCM,
with the already encoded video stream-copied. The same measurement and format
checks run on the corrected file; an unrecoverable miss still fails the render.

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
- [Jua](https://github.com/google/fonts/tree/main/ofl/jua) and [NanumMyeongjo](https://github.com/google/fonts/tree/main/ofl/nanummyeongjo): the Google Fonts `ofl/` trees, read on 2026-09-17; checksums in each face's own `README.md` under `backend/assets/fonts`.
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

### CPU release and host capacity

`pnpm smoke:media-worker` checks same-machine direct/worker output identity in all
three ratios, including original audio, transitions and sequence captions.
`pnpm smoke:media-release` extends the `cmd/api` release fixtures through a real
API child process, private MinIO and a standalone worker, in colocated and isolated
remote networks. The fixture registry is explicitly synthetic; production `/api`
contains no fixture control routes. A restart loses API memory, and the worker
has no SQLite mount or shared work directory.

The fixture caps API+driver at 256MiB/1CPU and the worker at 512MiB/1CPU, leaving
256MiB of the historical 1GiB envelope for other colocated services. Private MinIO
and the test network relay are separately capped infrastructure outside that
execution envelope, replacing external R2/network services. Reports include the
sum of container memory peaks (a conservative upper bound), sampled disk peaks,
project RPC latency, and claim-to-reservation / reservation-to-completion timings
(download+compute / reserve+upload). No result is a current VPS measurement or a
worst-case 20-source benchmark. `DEPLOY.md` §8.6 owns the read-only host preflight
and optional later disposable staging smoke.

API media dependencies remain for authored-plan validation, preview rasterization
and browser caption assets. Server preparation and final rendering always use the
durable worker protocol. Embedded execution
in the old release/identity fixtures is only a comparison baseline, never a worker
failure fallback in the production composition root.

## NVIDIA candidate diagnostics (not a production profile)

The supported CPU worker remains the production executor. `MEDIA_ACCEL=auto`
resolves to CPU; strict `nvenc` refuses readiness until CLIP-163 qualification and
implementation. Placement and image/device selection are explicit, not detected
from a hosting provider. The operational source is [DEPLOY.md §8](../../DEPLOY.md#media-environments),
including three layouts, pinned images, private connectivity, drain and rollback.

`media-worker-nvidia-candidate` copies the same static CPU tools, fonts, resvg and
worker into a pinned glibc Debian image. A separate `/opt/nvidia/bin/ffmpeg` uses
FFmpeg 9.0.1 with nv-codec-headers n13.0.19.0; Linux driver 570.0 or newer and a
supported device are prerequisites, not evidence of successful execution.
`nvidia-tools.sh` pins source hashes and Debian package snapshots and bundles
source archives, licenses, build configuration and package provenance. The build
checks declared NVENC/CUVID names; only the later hardware probe can prove execution.
No CUDA SDK, NPP or `--enable-nonfree` is included. CPU Compose does not reserve GPUs.

The independent `media-nvidia-candidate.yml` workflow publishes only on explicit
dispatch. API rollouts retain a candidate worker's independently selected image
pin, including on a colocated host. Remote API hosts never reserve a GPU.

`media-worker gpu-probe --output NEW_DIR` and `benchmark --manifest FILE --output
NEW_DIR [--cpu-only]` load tool settings only, without a worker client, database or
provider. The separate diagnostic Compose service has no job credential or network.
Reports always state `ProductionApproved=false`. Missing hardware produces an error
report; CPU-only runs leave GPU timings null/unverified. Device/driver, tools/assets,
parameters, cgroup resource observations, output hashes/measurements, sampled decoded
frame differences and decoded audio equality accompany the timing results.

`CLIP_DIAGNOSTIC_FIXTURES` on the test-only `TestWorkerExecutionParity` exports four
synthetic delivered MP4s in all three ratios, with motion, audio, transitions and
sequence captions, plus a local manifest and CPU composition+encode times. The
candidate comparison re-encodes those already compressed, CPU-composed inputs.
It isolates delivery encoding; it is not a lossless-master quality comparison or
an end-to-end speedup. CPU composition, intermediate representation, resvg and
existing audio processing remain unchanged. NVENC p4/hq/VBR/CQ20 and x264
veryfast/CRF20 are explicit candidate parameters, not equivalent quality claims.
CUVID decode is probed independently; CUDA scaling is not built/qualified.

Local CPU-only candidate builds, actual execution, missing-device behavior and
Compose checks can pass without a GPU. Real NVIDIA execution, representative
end-to-end timings, reviewed caption/motion/transition/audio output, resource bounds
and failure policy remain unverified activation gates. Follow the [isolated
hardware procedure](../../DEPLOY.md#media-gpu-diagnostics) later; never interpret a
passing packaging test or an encoder name as GPU production approval.
