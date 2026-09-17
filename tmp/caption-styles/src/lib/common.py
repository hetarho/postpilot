# -*- coding: utf-8 -*-
"""9:16 자막 스타일 데모 - 공통 유틸과 배경."""
import math, random

WIDTH, HEIGHT = 1080, 1920

FONTS = {
    "pretendard": ("Pretendard Variable", "PretendardVariable.ttf", "프리텐다드", "산세리프", 11172),
    "paperlogy":  ("Paperlogy", "Paperlogy-8ExtraBold.ttf", "페이퍼로지 ExtraBold", "산세리프", 11172),
    "jua":        ("Jua", "Jua.ttf", "주아", "산세리프", 2367),
    "nanummj":    ("NanumMyeongjo", "NanumMyeongjo-EB.ttf", "나눔명조", "세리프", 11172),
}

def ff(key):
    """resvg가 받아들이는 타이포그래픽 패밀리명."""
    return FONTS[key][0]

def ease_out_cubic(t): return 1 - (1 - t) ** 3
def ease_out_back(t, s=1.70158): return 1 + (s + 1) * (t - 1) ** 3 + s * (t - 1) ** 2
def clamp01(t): return max(0.0, min(1.0, t))

def esc(s):
    return (s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;"))

# ---------------------------------------------------------------- 배경
BACKGROUNDS = {
    "warm_food":   dict(label="따뜻한 음식", lum="어두움"),
    "night_neon":  dict(label="밤 거리 네온", lum="어두움"),
    "bright_cafe": dict(label="밝은 카페", lum="밝음"),
    "dark_studio": dict(label="어두운 스튜디오", lum="어두움"),
    "busy_market": dict(label="복잡한 시장", lum="중간·복잡"),
}

_SCENE = {
  "warm_food": dict(
     base=[("#2a1206",0),("#5c2a0c",0.45),("#180a03",1)], angle=(0,0,0.35,1),
     mass=[("#c2410c",520,0.85),("#f59e0b",380,0.7),("#7c2d12",640,0.8),("#b45309",300,0.75)],
     detail=[("#fbbf24",150,0.55),("#ea580c",190,0.5),("#78350f",220,0.6)],
     bokeh=["#fde68a","#fb923c","#fbbf24"], nbokeh=14, grain=0.13),
  "night_neon": dict(
     base=[("#05070f",0),("#0b1224",0.5),("#020309",1)], angle=(0,0,0.6,1),
     mass=[("#1d4ed8",460,0.75),("#7c3aed",380,0.7),("#0ea5e9",300,0.65),("#be185d",340,0.6)],
     detail=[("#22d3ee",120,0.8),("#f472b6",140,0.7),("#818cf8",160,0.6)],
     bokeh=["#67e8f9","#f0abfc","#fde047","#60a5fa"], nbokeh=22, grain=0.16),
  "bright_cafe": dict(
     base=[("#fbf6ee",0),("#e8dcc9",0.55),("#cdbda6",1)], angle=(0.2,0,0.8,1),
     mass=[("#ffffff",520,0.9),("#f5e9d5",420,0.8),("#d8c7ad",360,0.7),("#fffaf0",300,0.85)],
     detail=[("#ffffff",170,0.9),("#e4d4ba",150,0.6),("#b9a68a",130,0.45)],
     bokeh=["#ffffff","#fff7e0"], nbokeh=10, grain=0.08),
  "dark_studio": dict(
     base=[("#0a0c11",0),("#141821",0.5),("#05070a",1)], angle=(0,0,0.5,1),
     mass=[("#1e293b",560,0.8),("#334155",380,0.6),("#0f172a",620,0.9)],
     detail=[("#64748b",150,0.35),("#475569",190,0.3)],
     bokeh=["#94a3b8"], nbokeh=6, grain=0.14),
  "busy_market": dict(
     base=[("#3a2a18",0),("#6b4a24",0.5),("#1c1309",1)], angle=(0.1,0,0.7,1),
     mass=[("#b45309",420,0.8),("#eab308",360,0.75),("#57534e",400,0.7),("#78350f",480,0.8),("#a16207",320,0.7)],
     detail=[("#fcd34d",140,0.7),("#dc2626",120,0.6),("#84cc16",130,0.55),("#e7e5e4",150,0.5)],
     bokeh=["#fde68a","#fca5a5","#fef3c7"], nbokeh=18, grain=0.18),
}

def background_svg(name, seed=0):
    """실제 촬영 프레임을 흉내 낸 배경: 베이스 그라디언트 + 큰 덩어리 + 초점 디테일 + 보케 + 그레인."""
    import random as _r
    cfg = _SCENE[name]
    rnd = _r.Random(seed * 977 + (hash(name) % 9973))
    p = []
    x1, y1, x2, y2 = cfg["angle"]
    stops = "".join(f'<stop offset="{o}" stop-color="{c}"/>' for c, o in cfg["base"])
    p.append(f'<linearGradient id="bgbase" x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}">{stops}</linearGradient>')
    defs = "<defs>" + p.pop() + "</defs>"
    out = [defs, f'<rect width="{WIDTH}" height="{HEIGHT}" fill="url(#bgbase)"/>']
    # 큰 덩어리 (강한 블러)
    out.append('<g filter="url(#bgblur_lg)">')
    for c, r, op in cfg["mass"]:
        for _ in range(2):
            out.append(f'<ellipse cx="{rnd.randint(-120, WIDTH+120)}" cy="{rnd.randint(-160, HEIGHT+160)}" '
                       f'rx="{int(r*rnd.uniform(0.8,1.3))}" ry="{int(r*rnd.uniform(0.7,1.25))}" fill="{c}" opacity="{op}"/>')
    out.append('</g>')
    # 초점 디테일 (중간 블러)
    out.append('<g filter="url(#bgblur_md)">')
    for c, r, op in cfg["detail"]:
        for _ in range(3):
            out.append(f'<ellipse cx="{rnd.randint(0, WIDTH)}" cy="{rnd.randint(0, HEIGHT)}" '
                       f'rx="{int(r*rnd.uniform(0.7,1.4))}" ry="{int(r*rnd.uniform(0.6,1.3))}" fill="{c}" opacity="{op}"/>')
    out.append('</g>')
    # 보케 (약한 블러, 링 형태)
    out.append('<g filter="url(#bgblur_sm)">')
    for i in range(cfg["nbokeh"]):
        c = cfg["bokeh"][i % len(cfg["bokeh"])]
        r = rnd.randint(16, 62)
        out.append(f'<circle cx="{rnd.randint(0, WIDTH)}" cy="{rnd.randint(0, HEIGHT)}" r="{r}" fill="{c}" '
                   f'opacity="{round(rnd.uniform(0.18,0.62),2)}"/>')
    out.append('</g>')
    out.append(f'<rect width="{WIDTH}" height="{HEIGHT}" filter="url(#bggrain)" opacity="{cfg["grain"]}"/>')
    out.append(f'<rect width="{WIDTH}" height="{HEIGHT}" fill="url(#vignette)"/>')
    return "\n".join(out)

BG_DEFS = '''
  <filter id="bgblur_lg" x="-25%" y="-25%" width="150%" height="150%" color-interpolation-filters="sRGB">
    <feGaussianBlur stdDeviation="78"/></filter>
  <filter id="bgblur_md" x="-25%" y="-25%" width="150%" height="150%" color-interpolation-filters="sRGB">
    <feGaussianBlur stdDeviation="26"/></filter>
  <filter id="bgblur_sm" x="-25%" y="-25%" width="150%" height="150%" color-interpolation-filters="sRGB">
    <feGaussianBlur stdDeviation="7"/></filter>
  <filter id="bggrain" x="0" y="0" width="100%" height="100%" color-interpolation-filters="sRGB">
    <feTurbulence type="fractalNoise" baseFrequency="0.75" numOctaves="4" seed="9"/>
    <feColorMatrix type="matrix" values="0 0 0 0 0.55  0 0 0 0 0.55  0 0 0 0 0.55  0.9 0.6 0 0 -0.25"/>
  </filter>
  <radialGradient id="vignette" cx="50%" cy="44%" r="76%">
    <stop offset="0.40" stop-color="#000" stop-opacity="0"/>
    <stop offset="1" stop-color="#000" stop-opacity="0.66"/>
  </radialGradient>
'''

def frame(defs, body, bg_png=None, bg_rect=None, label=None, bg=None, seed=0):
    """bg_png 를 <defs> 안 <g id="bgsrc"> 로 두어 스타일이 <use href="#bgsrc"> 로 다시 쓸 수 있게 한다.
    리퀴드 글래스처럼 배경을 블러해서 깔아야 하는 스타일이 이 참조를 쓴다."""
    bgdef = layer = ""
    if bg_png:
        x, y, w, h = bg_rect or (0, 0, WIDTH, HEIGHT)
        bgdef = f'<g id="bgsrc"><image href="{bg_png}" x="{x:.1f}" y="{y:.1f}" width="{w:.0f}" height="{h:.0f}"/></g>'
        layer = '<use href="#bgsrc"/>'
    elif bg:
        layer = background_svg(bg, seed)
    lab = label or ""
    return f'''<svg xmlns="http://www.w3.org/2000/svg" width="{WIDTH}" height="{HEIGHT}" viewBox="0 0 {WIDTH} {HEIGHT}">
<defs>{BG_DEFS}{bgdef}{defs}</defs>
{layer}
{body}
{lab}
</svg>'''
