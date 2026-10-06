#!/usr/bin/env python3
"""Reproduce OFL ink instances and outline coverage from fixed product fonts.

Install fonttools==4.60.2 in an isolated Python environment; this is a build
tool, not a browser dependency. Original fonts remain unchanged. Each instance
is the same Wanted Sans 1.0.3 outlines at a fixed wght, with an internal family
name that uses no reserved font name and contains only valid CSS identifiers.
"""
import hashlib
import json
import unicodedata
from pathlib import Path

import fontTools
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont

ROOT = Path(__file__).resolve().parent.parent
FONT_ROOT = ROOT / "frontend/public/fonts/clip"
FONTTOOLS_VERSION = "4.60.2"
SOURCES = {
    "wantedsans": ("WantedSansVariable.ttf", "9953a7cfc4a3cba4ef1242abaf89779b3cd15fd9729c2d67d9e9d37a0da967f5"),
    "paperlogy": ("Paperlogy-8ExtraBold.ttf", "fb0324f8ac057e50f4f4632331617e347bfe5a04184f7b0db514be682fb6b25c"),
    "jua": ("Jua-Regular.ttf", "769677aef240bfc3b9965f2b50748075bff885e6c6992fc591a3fb268279f898"),
    "nanummyeongjo:400": ("NanumMyeongjo-Regular.ttf", "7ed9e8653a8ed04285d51dc343ffea6eb3d9c73afc27383ea8929ee4ffd03205"),
    "nanummyeongjo:800": ("NanumMyeongjo-ExtraBold.ttf", "60c0077fce069ba90ae97c0a3679f6eb3712e0ca637bdd0c15b72d335ec46db7"),
}


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def coverage(font):
    glyf, cache = font["glyf"], {}

    def drawn(name):
        if name not in cache:
            glyph = glyf[name]
            cache[name] = glyph.numberOfContours > 0 or (
                glyph.isComposite() and any(drawn(c.glyphName) for c in glyph.components)
            )
        return cache[name]

    ranges = []
    for code, name in sorted(font.getBestCmap().items()):
        c = chr(code)
        if name == ".notdef" or not (c.isspace() or unicodedata.category(c) == "Cf" or drawn(name)):
            continue
        if ranges and code == ranges[-1][1] + 1:
            ranges[-1][1] = code
        else:
            ranges.append([code, code])
    return ranges


def main():
    if fontTools.__version__ != FONTTOOLS_VERSION:
        raise SystemExit(f"Expected fonttools {FONTTOOLS_VERSION}, got {fontTools.__version__}")
    fonts, source_metadata = {}, {}
    for face, (filename, expected) in SOURCES.items():
        path = FONT_ROOT / filename
        if digest(path) != expected:
            raise SystemExit(f"Changed source font: {filename}")
        font = TTFont(path, recalcTimestamp=False)
        fonts[face] = font
        source_metadata[face] = {
            "file": filename,
            "sha256": expected,
            "units": font["head"].unitsPerEm,
            "ascent": font["hhea"].ascent,
            "descent": font["hhea"].descent,
            "coverage": coverage(font),
        }
    resources = []
    for weight in (400, 600, 700, 800):
        font = instantiateVariableFont(fonts["wantedsans"], {"wght": weight}, updateFontNames=False)
        font.recalcTimestamp = False
        family = f"Postpilot Wanted Ink W{weight}"
        names = {
            1: family, 2: "Regular", 3: f"PostpilotWantedInk{weight}-1.0.3-ink1",
            4: family + " Regular", 6: f"PostpilotWantedInk{weight}", 16: family, 17: "Regular",
        }
        for name in font["name"].names:
            if name.nameID in names:
                name.string = names[name.nameID].encode(name.getEncoding())
        filename = f"PostpilotWantedInk-{weight}.ttf"
        path = FONT_ROOT / filename
        font.save(path)
        resources.append({"face": "wantedsans", "weight": weight, "family": family,
                          "file": filename, "bytes": path.stat().st_size, "sha256": digest(path),
                          "source": "wantedsans", "license": "LICENSE"})
        print(f"Fixed same-face ink {weight}: {digest(path)}", flush=True)
    for source, weight, face, family, license_file in (
        ("paperlogy", 800, "paperlogy", "Paperlogy", "LICENSE-Paperlogy"),
        ("jua", 400, "jua", "Jua", "LICENSE-Jua"),
        ("nanummyeongjo:400", 400, "nanummyeongjo", "NanumMyeongjo", "LICENSE-NanumMyeongjo"),
        ("nanummyeongjo:800", 800, "nanummyeongjo", "NanumMyeongjo", "LICENSE-NanumMyeongjo"),
    ):
        metadata = source_metadata[source]
        resources.append({"face": face, "weight": weight, "family": family,
                          "file": metadata["file"], "bytes": (FONT_ROOT / metadata["file"]).stat().st_size,
                          "sha256": metadata["sha256"], "source": source, "license": license_file})
    document = {"version": 1, "generator": f"fonttools-{FONTTOOLS_VERSION}",
                "wantedSourceVersion": "1.0.3", "sources": source_metadata, "resources": resources}
    output = ROOT / "frontend/src/entities/clip-design/config/clip-ink-fonts.json"
    output.write_text(json.dumps(document, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
