#!/usr/bin/env python3
"""Intro/outro design exploration. Emits 1080x1920 artboards tiled into sheets,
using the real CDS tokens and the real bundled faces; resvg rasterizes them."""
import os, sys

W, H = 1080, 1920
GREEN = "#1F7A3D"
PRET = "Pretendard Variable"
PAPER = "Paperlogy"
CORAL = "#FF6B57"
WHITE = "#FFFFFF"
STROKE, STROKE_A = "#0B0B0B", 0.85


def esc(s):
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def text(value, y, size, *, family=PRET, weight=700, fill=WHITE, opacity=1.0,
         tracking=0.0, stroke=0.0, shadow=False, x=W / 2, anchor="middle"):
    a = [f'x="{x:.0f}"', f'y="{y:.0f}"', 'xml:space="preserve"',
         f'font-family="{family}"', f'font-size="{size:.0f}"', f'font-weight="{weight}"',
         f'letter-spacing="{tracking * size:.3f}"', f'text-anchor="{anchor}"', f'fill="{fill}"']
    if opacity < 1:
        a.append(f'fill-opacity="{opacity}"')
    if stroke:
        a += [f'stroke="{STROKE}"', f'stroke-opacity="{STROKE_A}"',
              f'stroke-width="{stroke}"', 'stroke-linejoin="round"', 'paint-order="stroke fill"']
    if shadow:
        a.append('filter="url(#sh)"')
    return f'<text {" ".join(a)}>{esc(value)}</text>'


def rect(x, y, w, h, fill, opacity=1.0, radius=0.0):
    return (f'<rect x="{x:.0f}" y="{y:.0f}" width="{w:.0f}" height="{h:.0f}" rx="{radius:.0f}" '
            f'fill="{fill}" fill-opacity="{opacity}"/>')


def rule(y, width, color=WHITE, opacity=0.55, thickness=2):
    return rect(W / 2 - width / 2, y, width, thickness, color, opacity)


def band(y, h, opacity, radius=0.0, inset=0.0):
    return rect(inset, y, W - 2 * inset, h, "#111111", opacity, radius)


PLACE, REGION, MENU = "해미 연풍 우가", "강릉", "소고기"
VERDICT, SCORE = "또 올 것 같아요", "4"

INTRO = {
    "A 크기만": [
        text(PLACE, 940, 96, family=PAPER, weight=800, tracking=-0.02, stroke=6, shadow=True),
        text(f"{REGION} · {MENU}", 1020, 36, weight=600, opacity=0.72, tracking=0.02, stroke=4, shadow=True),
    ],
    "B 위아래 가로선": [
        rule(846, 520), rule(1010, 520),
        text(PLACE, 960, 84, family=PAPER, weight=800, tracking=-0.02, shadow=True),
        text(f"{REGION} · {MENU}", 1090, 36, weight=600, opacity=0.72, tracking=0.02, shadow=True),
    ],
    "C 반투명 밴드 a0.35": [
        band(820, 260, 0.35),
        text(PLACE, 930, 84, family=PAPER, weight=800, tracking=-0.02),
        text(f"{REGION} · {MENU}", 1010, 36, weight=600, opacity=0.72, tracking=0.02),
    ],
    "D 반투명 박스 a0.55": [
        band(820, 260, 0.55, radius=16, inset=96),
        text(PLACE, 930, 84, family=PAPER, weight=800, tracking=-0.02),
        text(f"{REGION} · {MENU}", 1010, 36, weight=600, opacity=0.72, tracking=0.02),
    ],
    "E accent 가로선": [
        text(PLACE, 930, 96, family=PAPER, weight=800, tracking=-0.02, stroke=6, shadow=True),
        rule(985, 120, CORAL, 1.0, 6),
        text(f"{REGION} · {MENU}", 1060, 36, weight=600, opacity=0.72, tracking=0.02, stroke=4, shadow=True),
    ],
    "F 흐린 라벨 + 선명 값": [
        text(REGION, 870, 40, weight=600, opacity=0.6, tracking=0.08, stroke=4, shadow=True),
        text(PLACE, 970, 104, family=PAPER, weight=800, tracking=-0.02, stroke=6, shadow=True),
    ],
}

OUTRO = {
    "A 숫자 크게": [
        text(SCORE, 940, 200, family=PAPER, weight=800, tracking=-0.02, stroke=8, shadow=True),
        text("/5", 1020, 48, weight=700, opacity=0.6, stroke=4, shadow=True),
        text(PLACE, 1120, 44, weight=600, opacity=0.72, tracking=0.02, stroke=4, shadow=True),
    ],
    "B 가로선 구분": [
        text(PLACE, 900, 84, family=PAPER, weight=800, tracking=-0.02, shadow=True),
        rule(950, 520),
        text(VERDICT, 1040, 56, weight=700, shadow=True, stroke=5),
    ],
    "C 반투명 밴드": [
        band(800, 300, 0.45),
        text(PLACE, 900, 72, family=PAPER, weight=800, tracking=-0.02),
        text(VERDICT, 990, 48, weight=600, opacity=0.82),
        text(f"내 점수 {SCORE}/5", 1060, 36, weight=600, opacity=0.6, tracking=0.02),
    ],
    "D 미니멀": [
        text(PLACE, 930, 88, family=PAPER, weight=800, tracking=-0.02, stroke=6, shadow=True),
        text(VERDICT, 1010, 40, weight=600, opacity=0.72, stroke=4, shadow=True),
    ],
    "E 점수 + accent 선": [
        text("내 점수", 860, 36, weight=600, opacity=0.6, tracking=0.08, stroke=4, shadow=True),
        text(f"{SCORE}/5", 980, 132, family=PAPER, weight=800, tracking=-0.02, stroke=7, shadow=True),
        rule(1030, 160, CORAL, 1.0, 6),
        text(VERDICT, 1110, 44, weight=600, opacity=0.8, stroke=4, shadow=True),
    ],
    "F 라벨 스택": [
        text(PLACE, 880, 92, family=PAPER, weight=800, tracking=-0.02, stroke=6, shadow=True),
        rule(925, 400, WHITE, 0.4),
        text(REGION, 1000, 36, weight=600, opacity=0.6, tracking=0.08, stroke=4, shadow=True),
        text(VERDICT, 1070, 48, weight=700, stroke=5, shadow=True),
    ],
}

DEFS = ('<defs><filter id="sh" x="-30%" y="-30%" width="160%" height="160%">'
        '<feDropShadow dx="0" dy="4" stdDeviation="6" flood-color="#000000" flood-opacity="0.55"/>'
        '</filter></defs>')


def sheet(items, path, scale=0.5):
    cols = len(items)
    sw, sh = int(W * scale), int(H * scale)
    parts = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{cols * sw}" height="{sh}">', DEFS]
    for i, (name, body) in enumerate(items):
        parts.append(f'<g transform="translate({i * sw},0) scale({scale})">')
        parts.append(rect(0, 0, W, H, GREEN))
        parts.append(text(name, 70, 34, weight=600, opacity=0.85, x=40, anchor="start"))
        parts += body
        parts.append("</g>")
    parts.append("</svg>")
    open(path, "w").write("".join(parts))


out = sys.argv[1]
os.makedirs(out, exist_ok=True)
for label, group in (("intro", INTRO), ("outro", OUTRO)):
    items = list(group.items())
    for n in range(0, len(items), 3):
        sheet(items[n:n + 3], os.path.join(out, f"{label}-{n // 3 + 1}.svg"))
print("wrote", sorted(os.listdir(out)))
