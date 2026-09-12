# File-based SVG overlay presets

T119 separates overlay drawing from clip layout and video composition. T119 preserved the
shipped appearance; T120 subsequently aligned the shared grid and header text. Caption, disclosure/information and card SVGs
now live in `backend/internal/clip/overlay/presets/`, embedded into the API binary.
The same loader accepts a complete operator-provided directory at startup.

## Add a preset

Copy the built-in directory to a dedicated asset directory, for example
`/srv/postpilot/overlays`. Add a folder and change the relevant binding:

```text
overlays/
  bindings.json
  caption/preset.json
  caption/overlay.svg
  furniture/preset.json
  furniture/overlay.svg
  card/preset.json
  card/overlay.svg
  editorial/preset.json
  editorial/overlay.svg
```

`editorial/preset.json` declares the view contract, not a new product style:

```json
{"id":"editorial","view":"copy-v1","template":"overlay.svg"}
```

`bindings.json` selects drawings for existing product slots:

```json
{
  "version": 1,
  "bindings": {
    "copy.clean": "editorial",
    "copy.simple": "caption",
    "copy.memo": "caption",
    "copy.bold": "caption",
    "copy.mark": "caption",
    "furniture": "furniture",
    "card.hook": "card",
    "card.end": "card"
  }
}
```

Start by copying `caption/overlay.svg` into the new folder, then edit the SVG
drawing. No Go preset registration or switch is needed. A static SVG exported
from a design tool can supply paths, groups, gradients and decoration; replace
its variable text and placement with template fields. Keep variable text as
`<text>`, rather than converting it to paths.

The following small example shows the syntax, not a proposed product design:

```svg
<svg xmlns="http://www.w3.org/2000/svg"
     width="{{.Width}}" height="{{.Height}}">
  {{with .Plate}}
  <rect x="{{.X}}" y="{{.Y}}" width="{{.Width}}" height="{{.Height}}"
        rx="{{.Radius}}" fill="{{.Fill}}" fill-opacity="{{.Opacity}}"/>
  {{end}}
  {{range .Lines}}
  <text x="{{.X}}" y="{{.Y}}" xml:space="preserve"
        font-family="{{.Family}}" font-size="{{.Size}}"
        font-weight="{{.Weight}}" letter-spacing="{{.Tracking}}"
        fill="{{.Fill}}">{{.Value}}</text>
  {{end}}
</svg>
```

Templates use Go's `html/template` syntax (`with`, `if`, `range`, `printf`).
Plain text and attribute values are escaped automatically: a caption containing
`&` or `<` remains text. Do not pre-escape values or inject raw markup. The
complete built-in caption asset also handles keyword spans, highlights, scrims
and shadows; preserve those slots when a selected style requires them.

## Version 1 view contracts

The field definitions live in
[`view.go`](../../backend/internal/clip/overlay/view.go). All views receive
full-frame `Width` and `Height`. Geometry is in output pixels, text `Y` is the
baseline, and tracking is already converted to pixels. Layout has already
measured and wrapped text using the pinned fonts.

| View | Fields |
|---|---|
| `copy-v1` | Optional `Plate`, `Bar`, `Dot`, `Scrim`, `Shadow`; `Lines` of `Text` |
| `furniture-v1` | optional `Badge` box and `Label` text (nil when hidden); `Chips` with a box and `Label`/`Value` text |
| `card-v1` | `Empty`, `Plate`, `Shadow`; `Lines` with optional `Chip` box and `Text` |

| Shared value | Fields |
|---|---|
| `Box` | `X`, `Y`, `Width`, `Height`, `Radius`, `Fill`, `Opacity` |
| `Circle` | `X`, `Y`, `Radius`, `Fill` |
| `Shadow` | `DX`, `DY`, `Deviation` (Gaussian standard deviation), `Fill`, `Opacity` |
| `Scrim` | Box fields plus gradient opacity `From`, `To` |
| `Text` | `X`, `Y`, `Size`, `Weight`, `Family`, `Tracking`, `Value`, `Fill`, `Opacity` |
| Text effects | `Stroke`, `StrokeOpacity`, `StrokeWidth`, `Shadow` flag, optional `Highlight` box |
| Keyword text | `Colored` flag, `Prefix`, `Keyword`, `Suffix`, `Accent` |
| Chip value fitting | `Length` for `textLength` with `lengthAdjust="spacingAndGlyphs"` |

Use `with`/`if` for optional slots and `range` for lines/chips. An empty opacity
means the existing drawing omits that attribute; it is not an opacity of zero.
`card.hook` and `card.end` can use different files with the same `card-v1` view.
Incompatible future view shapes need another version. Preset IDs are lowercase
letters, digits and hyphens, start with a letter and have at most 64 characters.

## Loading and deployment

`CLIP_OVERLAY_DIR` defaults to empty, selecting embedded assets. To use files,
set it to the complete directory's absolute path **inside the API container**,
for example `/config/overlays`, and mount the host directory read-only:

```yaml
services:
  api:
    environment:
      CLIP_OVERLAY_DIR: /config/overlays
    volumes:
      - /srv/postpilot/overlays:/config/overlays:ro
```

Use the API service name from the actual stack; this is only the override
fragment. Files must be readable and directories traversable by the nonroot API
user. The entire catalog is loaded once. Restart or roll out the API to activate
an edit; in-flight jobs keep their startup snapshot. Rollback restores the prior
asset directory or clears the variable to use embedded assets. Edits to the
built-in repository assets require rebuilding the API.

A frontend `public/` directory alone does not reach the renderer. It can be the
authoring source if the deployment copies or mounts that catalog into the API;
no HTTP fetching is needed. Only trusted deployment files are accepted, with no
request-controlled paths or upload endpoint.

The constructor checks every discovered preset and required binding against
representative view data before accepting render work. Unknown fields, invalid
bindings, malformed templates/SVG and oversized files fail startup. Dynamic
output is checked again on each render; startup probes cannot exercise every
conditional branch. Limits: 64 presets, 512 KiB per file, 8 MiB per catalog and
2 MiB rendered SVG. Symlinks and nested preset directories are rejected.

Use one standalone static SVG root with the SVG namespace. Local fragment
references such as `url(#shadow)` are supported. Remove XML/DOCTYPE declarations,
scripts, event handlers, `foreignObject`, animation elements and external
href/src references from exports. Keep styles and assets self-contained; the
checks enforce a trusted-asset contract, not a general untrusted-SVG sanitizer.

## Boundaries and verification

The catalog owns markup and asset selection. The media adapter prepares measured
views; existing design configuration still owns layout, typography, safe regions,
visibility and time windows. FFmpeg still composes bounded static PNG layers.
A new drawing does not add a UI style, font, animation or semantic content field.
Changing those policies remains a separate change at the corresponding layer.

Draw within the supplied layout bounds and use the measured font metrics. The
manifest describes the layout input; it does not reverse-engineer arbitrary
paths or text metrics from the SVG. Moving/resizing text or changing its font in
markup alone can make validation disagree with the pixels. A future layout
change belongs in the view adapter/design configuration as well as the asset.

Tests cover discovery, binding/version errors, escaping, read/output limits,
immutable snapshots and constructor failures. Existing SVG goldens remain
byte-identical. The production image gate additionally rasterizes a new
file-only preset with real resvg and checks its changed pixels. The renderer
regression suite and eight original owner videos check unchanged delivery.

```sh
cd backend
go test ./internal/clip/overlay ./internal/clip/media ./internal/platform/config
```

From the repository root, `docker build --target production -f backend/Dockerfile .`
includes the real media/preset smoke gate. Test-only sample designs never become
the shipped bindings.

## Shorts example

`backend/examples/overlays/shorts-editorial/` is a complete opt-in catalog with
caption, header, opening and ending SVGs. It uses the existing version 1 views,
compact dark labels, a clear unaccented disclosure, outlined white captions and
unboxed opening/ending titles over the footage. The demonstration selects the existing lime
accent; the fonts and measured text positions remain those supplied by the
renderer. Default embedded bindings do not select this sample theme.

Mount that directory as the API's overlay directory to preview it. The local
`TestShortsSVGExample` accepts `CLIP_EXAMPLE_ORIGINALS`, `CLIP_EXAMPLE_OUTPUT` and
`CLIP_EXAMPLE_ASSETS` inside the production media runtime. It uses all eight
owner originals, checks a collision-free manifest before and after rendering,
and exports the 20 s video and edit plan. Its title is editorial sample copy,
not a verified merchant name. No AI provider is involved.

## Vertical top placement

CDS r7 sets the 1080×1920 design inset to 40 px plus a further 40 px gap:
the first information row, disclosure and TOP copy start at y 80. Layout lives
in `backend/internal/clip/design/design.json`, mirrored in the frontend config.
The top scrim begins at y 40; the bottom bound stays y 1420. Historical platform
UI estimates are retained separately and do not control this design inset.

The sample's `header/overlay.svg` draws the menu and disclosure; `caption/overlay.svg`
draws body captions; `opening/overlay.svg` and `ending/overlay.svg` draw the two
title windows. Runtime text and measured coordinates fill the template fields,
resvg converts the SVG to a transparent PNG, and FFmpeg overlays it on footage.

## Lightweight text and caption pace

CDS r8 adds `simple` (가벼운 텍스트): the bundled Pretendard Variable face at
56/600, white fill, dark 4 px outline and a text shadow, without a plate or accent.
The browser's simple preview self-hosts the same original font and OFL license
from `frontend/public/fonts/clip/`. Existing catalogs without `copy.simple`
fall back to their `copy.clean` binding; preserve the supplied outline and shadow
fields when that binding also draws the simple style. Its contrast proof relies
on the fixed outline rather than sampling the footage for every phrase.

Caption pace is independent of drawing. Empty/`steady` preserves existing plans;
`rapid` uses explicit 300–1000 ms windows and immediate replacement. A cut may
carry up to 24 rapid phrases, each one line of at most 14 characters (or its
style's tighter limit). The shared splitter preserves a short first word as an
opening beat, then groups words without another provider call. The correction
screen can split/merge, add/remove and edit each phrase's exact timing.

Rapid PNGs are cropped to their measured drawing region plus stroke/shadow
padding and overlaid at the original coordinates; a scrim retains its full
extent. Presets must keep drawing inside the measured bounds. Every cue has a
unique raster path, so multiple captions cannot overwrite one another.

`TestRenderSmokeRapidCaptions` checks 24 phrases and the 300 ms boundary using
real video frames. `TestCaptionPaceExample`, enabled with `CLIP_EXAMPLE_PACE=1`
and the three example directory variables above, exports a 20 s comparison from
the eight local originals. This is editorial timing, not forced speech alignment.

Disclosure visibility is a saved project setting, independent of campaign type.
Keep the header's badge markup inside `{{with .Badge}}` and its text inside
`{{with .Label}}` so disabling it leaves the information chips visible with no
empty plate. The supplied catalog and built-in preset both follow this contract.
The header uses equal outer margins: vertical x=96/984, horizontal x=96/1824,
and square x=64/1016. Caption anchors and the 80 px vertical header top stay unchanged.
