# Bundled browser static ink

T594 ports native copy fitting, logical font metrics, glyph substitution and SVG
templates into `BrowserLocalComponents`. Call `prepare(evaluateBrowserFrame(...))`,
draw each leased resource and close it. The cache retains reusable ink until
eviction or lifecycle destruction; it reserves at most 64 MiB and 64 entries and
allows one compositor GPU owner per entry. It stores no original footage.

The bundled resvg WASM 2.6.2 cannot select the native Wanted variable weights.
Deterministic fontTools 4.60.2 instances of the same Wanted Sans Variable 1.0.3
source provide weights 400, 600, 700 and 800. Original source and OFL files remain
bundled. Internal nonreserved family aliases select those instances; product face
identity remains Wanted Sans. `scripts/build-clip-ink-fonts.py` checks source
hashes and reproduces the files. Outline coverage excludes mapped empty glyphs.
Caption substitution uses Wanted per character, followed by the existing bold
fallback and notice; unsupported glyphs refuse before rasterization.

Native logical advance is recovered from the displacement between start/end
shaped anchors, independently from tight ink bounds. Native hhea ascent/descent
and three-decimal query rounding provide baseline geometry. The catalog is
exported from native design definitions, and a regression detects drift. Fixed
font files, their hashes, coverage/metrics manifest, catalog and WASM hash bind the
frozen snapshot, cache definitions and server asset version. Changed definitions
cannot replay under the same snapshot identity.

The [recorded comparison](browser-static-ink-2026-10-07.json) covers 72 synthetic
fixtures: bold/keynote/film with both paces, all 15 intro/outro presets, disclosure,
blank slots and combined header/info in all three ratios. Caption samples include
rare Hangul, mixed Latin, kerning, numbers and units. Native resvg 0.48.1 renders
the reference from the immutable portable elements, without account, provider,
database or original footage. Chrome 154 uses ANGLE Metal on Apple M1 Max.
Twenty comparisons are byte exact. Maximum normalized RGBA error over the union
of visible pixels is 0.00005740802020529412; the remaining renderer differences are
recorded rather than treated as byte equality. Warm preparation adds no raster,
every lease returns to zero, destruction clears entries/bytes, and the largest
fixture uses 2,025,304 bitmap bytes. Timings include competing work and are not a
throughput qualification.

Reproduce the reference with:

```sh
cd backend
CLIP_RESVG_PATH=/opt/homebrew/bin/resvg CLIP_BROWSER_INK_FIXTURES=/absolute/new/output go test ./internal/clip/media -run TestExportBrowserStaticInkFixtures -count=1
```

Then run `node scripts/browser-ink-benchmark.mjs --fixtures /absolute/new/output
--output /absolute/browser/output` from the repository root. `--chrome` selects the
physical installed browser. Fixture references are an allowlisted localhost-only
test input; product rendering makes no server PNG/frame-sheet request.

The read-only `effective_position` projection preserves a saved native automatic
anchor. After editing clears native placement, missing automatic anchor geometry
refuses; T600 must port observed subject/readable/caption-safe geometry and
`SelectAnchor`, including header/previous overlaps. Dynamic effects belong to
T595–T597. Browser distribution remains disabled pending T604 covered-source graph,
whole-output accuracy and sustained performance gates. This static comparison
does not approve Docker images, whole-output export or source distribution.
