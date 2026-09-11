# Paperlogy 8 ExtraBold 1.001

Unmodified official font, taken from the designer's own distribution:
[freesentation.blog/paperlogyfont](https://freesentation.blog/paperlogyfont) →
`https://github.com/Freesentation/paperlogy/raw/refs/heads/main/Paperlogy-1.001.zip`.

- Archive SHA-256: `6ffa5c8fc7539c61f419dcd2c4dd714556412f2455d26399e83792968c7b23d6` (5,609,426 bytes, version 1.001, 2024-10-07).
- Bundled file: `Paperlogy-8ExtraBold.ttf` from that archive.
- SHA-256: `fb0324f8ac057e50f4f4632331617e347bfe5a04184f7b0db514be682fb6b25c`.
- Size: 1,304,560 bytes.
- Family name inside the file: `Paperlogy 8 ExtraBold` (subfamily `Regular`), 900 units/em, 14,198 glyphs.
- License: SIL Open Font License 1.1. `LICENSE` is the official notice the
  distribution page links (`designptn.com/wp-content/uploads/2024/08/OFL-license.txt`),
  whose copyright line names the Paperlogy authors; the archive itself ships no
  license file.
- Runtime: `/usr/share/postpilot-fonts/paperlogy/Paperlogy-8ExtraBold.ttf`, loaded
  explicitly as a second `--use-font-file` with system-font discovery disabled.

Only the ExtraBold weight is bundled: the design system uses Paperlogy for the
hook title and 크게 강조 and for nothing else, both at weight 800.

The renderer verifies the size and checksum, and refuses a glyph this face lacks
rather than substituting one — the Noto Sans KR fallback the design system names
never fires here, because resvg runs with system fonts off.
