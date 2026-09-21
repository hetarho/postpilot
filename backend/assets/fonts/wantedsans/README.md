# Wanted Sans Variable 1.0.3

Unmodified official font: [Wanted Sans v1.0.3](https://github.com/wanteddev/wanted-sans/releases/tag/v1.0.3).

- Source: `variable/WantedSansVariable.ttf` in the release archive `WantedSans-1.0.3.zip`.
- SHA-256: `9953a7cfc4a3cba4ef1242abaf89779b3cd15fd9729c2d67d9e9d37a0da967f5`.
- Size: 4,669,352 bytes.
- Declared family: `Wanted Sans Variable`; `wght` axis 400–1000.
- License: SIL Open Font License 1.1, reserved font name "Wanted Sans Variable"; the official notice is in `LICENSE`.
- Runtime: `/usr/share/postpilot-fonts/wantedsans/WantedSansVariable.ttf`; loaded explicitly with system-font discovery disabled.

It replaced Pretendard Variable 1.3.9 as the primary face on 2026-09-21 (owner
decision). Two consequences are worth stating, because both are checked rather
than assumed:

- The axis STARTS at 400, where Pretendard's started at 250. The 키노트 caption
  style asked for 250 and now asks for 400, the lightest weight this face has.
- It maps 12,006 characters against Pretendard's 14,333. All 11,172 Hangul
  syllables and the whole Latin-1 range are there, so Korean and English copy is
  unaffected; what is gone is Cyrillic, Greek, kana, full-width forms and the CJK
  compatibility units (`㎏` `㎝` `㎜` `㎡` `ℓ` `№` `▲` `▼` among them). The renderer
  refuses a glyph it cannot draw rather than substituting one (CLIP-13), so copy
  carrying one of those is `ErrInvalid` — `kg` sets, `㎏` does not.

The renderer verifies the size and checksum and rejects missing glyphs; it never
silently substitutes a system font. Go's SFNT reader checks glyph coverage only;
all layout measurement and rasterization use the same pinned resvg engine.
