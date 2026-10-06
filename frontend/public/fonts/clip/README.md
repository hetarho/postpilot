# Bundled faces

- Wanted Sans Variable 1.0.3: unchanged from `backend/assets/fonts/wantedsans/`, license `LICENSE`.
- Paperlogy ExtraBold: unchanged from `backend/assets/fonts/paperlogy/`, license `LICENSE-Paperlogy`.
- Jua Regular: unchanged from `backend/assets/fonts/jua/`, license `LICENSE-Jua`.
- NanumMyeongjo Regular and ExtraBold: unchanged from `backend/assets/fonts/nanummyeongjo/`, license `LICENSE-NanumMyeongjo`.

Every file here is byte-identical to the one the renderer pins by SHA-256, and
the family names match `design.json`'s `faces` exactly: a caption style's
served fragment names its family and the browser has to shape it with the same
file resvg does, or the preview would show a placement the render will not
produce (CDS-83).

The SVG preview loads these same bundled faces and measures their glyph bounds
before placing information, captions and preset slots. Export disables system-font discovery.

Local WASM ink uses fixed same-face Wanted 400/600/700/800 instances with
CSS-valid internal families `Postpilot Wanted Ink W400/W600/W700/W800`.
`INK-NOTICE.txt` records the exact modifications, source and OFL obligations;
`scripts/build-clip-ink-fonts.py` reproduces them with fontTools 4.60.2.
The source/derived manifest and frozen asset digest are kept in clip-design.
The original variable face above remains unchanged for the interface/native
renderer. No 1000-weight instance is packaged or exposed as a component.

Wanted Sans is also the INTERFACE's face — `--font-sans` in `app/styles/index.css`
names it first — so the app declares it once, from this directory, and the UI and
the preview share that one file. That is why this directory is no longer only the
renderer's: splitting it into a second, lighter build for the UI would put two
`@font-face` rules under one family name, and the preview could end up shaped by
whichever the cascade picked rather than by the file resvg loads.
