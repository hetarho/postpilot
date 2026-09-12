# Shorts editorial example

A complete, optional SVG catalog for T120. Copy this directory or mount it
read-only and set `CLIP_OVERLAY_DIR` to its container path. It is not selected by
the default embedded bindings.

- `header`: aligned information and disclosure, compact 6 px corner boxes.
- `opening`: a category tag and bold outlined title over real footage.
- `caption`: outlined white captions without an opaque panel.
- `ending`: an unboxed final title/information stack with a lime rule.

The example video uses the existing lime accent, a curated 20 s plan and all
eight owner videos. Original audio is retained. The first and last cuts use only
their title card, avoiding duplicate captions. No unknown merchant, price or
location is asserted; “철판 한 끼” is a sample editorial heading.

See `docs/design/overlay-presets.md` in the repository for the view contracts and
runtime configuration. Edit the SVG markup and binding JSON to iterate on this
design; keep text inside its measured slots.
