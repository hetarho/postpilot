# Clip preview fonts

- Pretendard Variable 1.3.9: unchanged from `backend/assets/fonts/pretendard/`, license `LICENSE`.
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
