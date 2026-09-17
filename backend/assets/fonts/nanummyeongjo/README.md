# NanumMyeongjo 400 and 800

Unmodified official fonts from Google Fonts:
[`ofl/nanummyeongjo`](https://github.com/google/fonts/tree/main/ofl/nanummyeongjo).

| file | weight | SHA-256 | size |
|---|---|---|---|
| `NanumMyeongjo-Regular.ttf` | 400 | `7ed9e8653a8ed04285d51dc343ffea6eb3d9c73afc27383ea8929ee4ffd03205` | 3,058,408 |
| `NanumMyeongjo-ExtraBold.ttf` | 800 | `60c0077fce069ba90ae97c0a3679f6eb3712e0ca637bdd0c15b72d335ec46db7` | 3,180,888 |

- Both files answer to the same CSS family `NanumMyeongjo` and are told apart by
  `font-weight`: Regular declares it as its legacy family name (name ID 1) and
  ExtraBold declares it as its typographic family (name ID 16), whose legacy
  name is the unparsable `NanumMyeongjoExtraBold`.
- Both cover all 11,172 Hangul syllables.
- License: SIL Open Font License 1.1; the official notice is in `LICENSE`.
- Runtime: `/usr/share/postpilot-fonts/nanummyeongjo/`, both loaded explicitly as
  `--use-font-file` with system-font discovery disabled.

CDS-17 names exactly these two weights, so exactly these two files are bundled.
