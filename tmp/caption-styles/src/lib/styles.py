# -*- coding: utf-8 -*-
"""자막 스타일 22종 (2026 트렌드 반영). 각 함수는 (defs, body). t 는 0~1 진행도."""
import math, random
from common import ff, FONTS, WIDTH, HEIGHT, esc, ease_out_cubic, ease_out_back, clamp01
from measure import measure, width

CY = 1150
LIME, INK, SNOW = "#C8F751", "#0A0C10", "#FFFFFF"

def _fade(t, rise=14, fin=0.18, fout=0.88):
    a_in = clamp01(t / fin); a_out = 1 - clamp01((t - fout) / (1 - fout))
    op = min(ease_out_cubic(a_in), a_out)
    return op, rise * (1 - ease_out_cubic(a_in))

SAFE_W = 906          # 안전영역 952에서 좌우 여유를 뺀 폭

def fit(text, fam, size, wt=400, sp=0, maxw=SAFE_W, minsize=38):
    """문구가 길면 안전영역에 들어올 때까지 줄인다. 짧으면 그대로 크게 쓴다."""
    while size > minsize and measure(text, fam, size, wt, sp)["w"] > maxw:
        size -= 2
    return size

def _base(text, fam, size, wt=400, sp=0):
    m = measure(text, fam, size, wt, sp)
    return m, CY + m["h"] / 2 - (m["h"] + m["top"])

def _words(text, fam, size, wt=400):
    ws = text.split(" ")
    gap = measure(" ", fam, size, wt)["w"] or size * 0.3
    wd = [measure(w, fam, size, wt)["w"] for w in ws]
    return ws, wd, gap, sum(wd) + gap * (len(ws) - 1)

# ============================================================ 1 워드 팝
def word_pop(text, kw, t, seed=0):
    """2026 숏폼의 지배적 스타일. 단어가 차례로 살아나고 나머지는 죽는다."""
    fam, wt = ff("pretendard"), 800
    size = fit(text, fam, 84, wt)
    ws, wd, gap, total = _words(text, fam, size, wt)
    m, y = _base(text, fam, size, wt)
    x = (WIDTH - total) / 2
    op, _ = _fade(t, rise=0, fin=0.08)
    # 단어마다 활성 구간. 2~4단어 청크를 600~900ms 쥐는 요즘 호흡에 맞췄다.
    span = 0.74 / max(len(ws), 1)
    active = int(clamp01((t - 0.10) / 0.74) * len(ws))
    parts = []
    for i, (w, ww) in enumerate(zip(ws, wd)):
        cx = x + ww / 2
        if i < active:   fill, o, sc = SNOW, 0.62, 1.0
        elif i == active: fill, o, sc = LIME, 1.0, 1.07
        else:            fill, o, sc = SNOW, 0.22, 1.0
        g = (f'<text x="{x:.1f}" y="{y:.1f}" font-family="{fam}" font-weight="{wt}" font-size="{size}" '
             f'fill="{fill}" fill-opacity="{o}" stroke="{INK}" stroke-width="9" stroke-opacity="{o*0.9:.2f}" '
             f'paint-order="stroke" stroke-linejoin="round">{esc(w)}</text>')
        if sc != 1.0:
            g = f'<g transform="translate({cx:.1f},{y-size*0.3:.1f}) scale({sc}) translate({-cx:.1f},{-(y-size*0.3):.1f})">{g}</g>'
        parts.append(g)
        x += ww + gap
    return "", f'<g opacity="{op:.3f}">{"".join(parts)}</g>'

# ============================================================ 2 키노트
def keynote(text, kw, t, seed=0):
    """애플 키노트. 얇고 크게, 자간을 좁히고, 아무것도 더하지 않는다."""
    fam, wt, sp = ff("pretendard"), 250, -2.6
    size = fit(text, fam, 108, wt, sp)
    m, y = _base(text, fam, size, wt, sp)
    op, dy = _fade(t, rise=10, fin=0.30)
    lab = "ONE MORE THING"
    lo = clamp01((t - 0.34) / 0.3)
    defs = '''<filter id="knsh" x="-20%" y="-20%" width="140%" height="160%" color-interpolation-filters="sRGB">
      <feOffset in="SourceAlpha" dy="2" result="o"/><feGaussianBlur in="o" stdDeviation="14" result="b"/>
      <feFlood flood-color="#000" flood-opacity="0.5"/><feComposite in2="b" operator="in" result="s"/>
      <feMerge><feMergeNode in="s"/><feMergeNode in="SourceGraphic"/></feMerge></filter>'''
    body = f'''<g opacity="{op:.3f}" transform="translate(0,{dy:.1f})" filter="url(#knsh)">
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-weight="{wt}"
        font-size="{size}" letter-spacing="{sp}" fill="{SNOW}">{esc(text)}</text>
  <text x="{WIDTH/2}" y="{y + 82:.1f}" text-anchor="middle" font-family="{fam}" font-weight="500"
        font-size="26" letter-spacing="6" fill="#BCC6D4" opacity="{lo:.2f}">{lab}</text>
</g>'''
    return defs, body

# ============================================================ 3 리퀴드 글래스
def liquid_glass(text, kw, t, seed=0):
    """iOS 26 결. 유리는 패널이 지고, 글자는 그 위 불투명 레이어에 앉는다."""
    fam, wt = ff("pretendard"), 600
    size = fit(text, fam, 70, wt, maxw=800)
    m, y = _base(text, fam, size, wt)
    pw, ph = m["w"] + 108, m["h"] + 86
    px, py = (WIDTH - pw) / 2, CY - ph / 2
    p = ease_out_cubic(clamp01(t / 0.32))
    op = min(p, 1 - clamp01((t - 0.9) / 0.1))
    sc = 0.94 + 0.06 * p
    cx, cy = WIDTH / 2, CY
    r = ph / 2
    defs = f'''
  <filter id="lgblur" x="-25%" y="-25%" width="150%" height="150%" color-interpolation-filters="sRGB">
    <feGaussianBlur stdDeviation="30"/></filter>
  <clipPath id="lgclip"><rect x="{px:.1f}" y="{py:.1f}" width="{pw:.1f}" height="{ph:.1f}" rx="{r:.1f}"/></clipPath>
  <linearGradient id="lgsheen" x1="0" y1="0" x2="0.25" y2="1">
    <stop offset="0" stop-color="#FFFFFF" stop-opacity="0.30"/>
    <stop offset="0.48" stop-color="#FFFFFF" stop-opacity="0.07"/>
    <stop offset="1" stop-color="#FFFFFF" stop-opacity="0.17"/>
  </linearGradient>'''
    body = f'''<g opacity="{op:.3f}" transform="translate({cx:.1f},{cy:.1f}) scale({sc:.3f}) translate({-cx:.1f},{-cy:.1f})">
  <g clip-path="url(#lgclip)">
    <use href="#bgsrc" filter="url(#lgblur)"/>
    <rect x="{px:.1f}" y="{py:.1f}" width="{pw:.1f}" height="{ph:.1f}" fill="url(#lgsheen)"/>
  </g>
  <rect x="{px:.1f}" y="{py:.1f}" width="{pw:.1f}" height="{ph:.1f}" rx="{r:.1f}" fill="none"
        stroke="#FFFFFF" stroke-opacity="0.42" stroke-width="1.5"/>
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-weight="{wt}"
        font-size="{size}" fill="{SNOW}" letter-spacing="-0.5">{esc(text)}</text>
</g>'''
    return defs, body

# ============================================================ 4 블러 인
def blur_in(text, kw, t, seed=0):
    fam, wt = ff("pretendard"), 700
    size = fit(text, fam, 96, wt)
    m, y = _base(text, fam, size, wt)
    p = ease_out_cubic(clamp01((t - 0.04) / 0.34))
    sd = 26 * (1 - p)
    sc = 1.05 - 0.05 * p
    op = min(p, 1 - clamp01((t - 0.88) / 0.12))
    cx, cy = WIDTH / 2, CY
    defs = f'''<filter id="bi" x="-30%" y="-40%" width="160%" height="180%" color-interpolation-filters="sRGB">
      <feGaussianBlur stdDeviation="{max(sd,0.01):.2f}"/></filter>'''
    body = f'''<g opacity="{op:.3f}" transform="translate({cx:.1f},{cy:.1f}) scale({sc:.3f}) translate({-cx:.1f},{-cy:.1f})">
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-weight="{wt}" font-size="{size}"
        fill="{SNOW}" letter-spacing="-1.5" filter="url(#bi)">{esc(text)}</text>
</g>'''
    return defs, body

# ============================================================ 5 캡션 필
def caption_pill(text, kw, t, seed=0):
    """무음 시청용 기본형. 장식을 빼고 읽히는 것만 남겼다."""
    fam, wt = ff("pretendard"), 600
    size = fit(text, fam, 62, wt, maxw=840)
    m, y = _base(text, fam, size, wt)
    ph = m["h"] + 46
    pw_full = m["w"] + 68
    p = ease_out_cubic(clamp01(t / 0.24))
    pw = pw_full * (0.72 + 0.28 * p)
    px, py = (WIDTH - pw) / 2, CY - ph / 2
    op, _ = _fade(t, rise=0, fin=0.12)
    to = clamp01((t - 0.12) / 0.2)
    body = f'''<g opacity="{op:.3f}">
  <rect x="{px:.1f}" y="{py:.1f}" width="{pw:.1f}" height="{ph:.1f}" rx="{ph/2:.1f}" fill="{INK}" fill-opacity="0.80"/>
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-weight="{wt}" font-size="{size}"
        fill="#F2F5FA" opacity="{to:.2f}">{esc(text)}</text>
</g>'''
    return "", body

# ============================================================ 6 마스크 리빌
def mask_reveal(text, kw, t, seed=0):
    fam, wt = ff("pretendard"), 800
    size = fit(text, fam, 92, wt)
    m, y = _base(text, fam, size, wt)
    x0 = (WIDTH - m["w"]) / 2
    p = ease_out_cubic(clamp01((t - 0.06) / 0.38))
    top, h = y - size * 0.92, size * 1.24
    cy = top + h * (1 - p)
    op = 1 - clamp01((t - 0.9) / 0.1)
    defs = f'''<clipPath id="mr"><rect x="{x0-40:.1f}" y="{cy:.1f}" width="{m["w"]+80:.1f}" height="{h*p+2:.1f}"/></clipPath>'''
    body = f'''<g opacity="{op:.3f}">
  <g clip-path="url(#mr)">
    <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-weight="{wt}" font-size="{size}"
          fill="{SNOW}" letter-spacing="-1.2">{esc(text)}</text>
  </g>
  <rect x="{x0-40:.1f}" y="{cy-3:.1f}" width="{m["w"]+80:.1f}" height="3" fill="{LIME}"
        opacity="{(1-p)*0.95:.2f}"/>
</g>'''
    return defs, body

# ============================================================ 7 믹스드 웨이트
def mixed_weight(text, kw, t, seed=0):
    """한 문장 안에서 굵기를 갈라 강조한다. 애플이 자주 쓰는 방식."""
    fam = ff("pretendard")
    size = fit(text, fam, 92, 250, -1.6)
    idx = text.find(kw) if kw else -1
    if idx < 0: idx, kw = 0, text.split(" ")[0]
    pre, post = text[:idx], text[idx + len(kw):]
    wp = measure(pre, fam, size, 250)["w"] if pre else 0
    wk = measure(kw, fam, size, 900)["w"]
    wo = measure(post, fam, size, 250)["w"] if post else 0
    total = wp + wk + wo
    m = measure(text, fam, size, 250)
    y = CY + m["h"] / 2 - (m["h"] + m["top"])
    x = (WIDTH - total) / 2
    op, dy = _fade(t, rise=12, fin=0.22)
    ko = clamp01((t - 0.24) / 0.26)
    kdy = (1 - ease_out_cubic(ko)) * 10
    body = f'''<g opacity="{op:.3f}" transform="translate(0,{dy:.1f})">
  <text x="{x:.1f}" y="{y:.1f}" font-family="{fam}" font-weight="250" font-size="{size}"
        fill="{SNOW}" fill-opacity="0.82" letter-spacing="-1.6">{esc(pre)}</text>
  <text x="{x+wp:.1f}" y="{y+kdy:.1f}" font-family="{fam}" font-weight="900" font-size="{size}"
        fill="{LIME}" letter-spacing="-2.2" opacity="{ko:.2f}">{esc(kw)}</text>
  <text x="{x+wp+wk:.1f}" y="{y:.1f}" font-family="{fam}" font-weight="250" font-size="{size}"
        fill="{SNOW}" fill-opacity="0.82" letter-spacing="-1.6">{esc(post)}</text>
</g>'''
    return "", body

# ============================================================ 8 소프트 앰비언트
def soft_ambient(text, kw, t, seed=0):
    """글자 뒤에 광원을 직접 깔았다. 필터로 퍼뜨리면 알파가 죽어 아무것도 남지 않는다."""
    fam, wt = ff("pretendard"), 700
    size = fit(text, fam, 92, wt)
    m, y = _base(text, fam, size, wt)
    op, dy = _fade(t, rise=10)
    cx, cy = WIDTH / 2, CY - size * 0.25
    br = 0.86 + 0.14 * math.sin(t * math.tau * 0.9)
    rx, ry = m["w"] * 0.62, size * 1.15
    defs = """
  <radialGradient id="ambA" cx="50%" cy="50%" r="50%">
    <stop offset="0" stop-color="#8B5CF6" stop-opacity="0.95"/>
    <stop offset="0.55" stop-color="#6D3BF5" stop-opacity="0.42"/>
    <stop offset="1" stop-color="#4C1D95" stop-opacity="0"/></radialGradient>
  <radialGradient id="ambB" cx="50%" cy="50%" r="50%">
    <stop offset="0" stop-color="#4AD9E8" stop-opacity="0.9"/>
    <stop offset="0.5" stop-color="#22B8CF" stop-opacity="0.34"/>
    <stop offset="1" stop-color="#0E7490" stop-opacity="0"/></radialGradient>
  <filter id="ambsoft" x="-40%" y="-60%" width="180%" height="220%" color-interpolation-filters="sRGB">
    <feGaussianBlur stdDeviation="26"/></filter>
  <filter id="ambtext" x="-30%" y="-40%" width="160%" height="180%" color-interpolation-filters="sRGB">
    <feGaussianBlur in="SourceAlpha" stdDeviation="7" result="b"/>
    <feFlood flood-color="#0A0C10" flood-opacity="0.55"/><feComposite in2="b" operator="in" result="s"/>
    <feMerge><feMergeNode in="s"/><feMergeNode in="SourceGraphic"/></feMerge></filter>"""
    body = f"""<g opacity="{op:.3f}" transform="translate(0,{dy:.1f})">
  <g filter="url(#ambsoft)">
    <ellipse cx="{cx - m['w']*0.18:.1f}" cy="{cy:.1f}" rx="{rx*br:.1f}" ry="{ry*br:.1f}" fill="url(#ambA)"/>
    <ellipse cx="{cx + m['w']*0.22:.1f}" cy="{cy + size*0.18:.1f}" rx="{rx*0.72*br:.1f}" ry="{ry*0.86*br:.1f}" fill="url(#ambB)"/>
  </g>
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-weight="{wt}" font-size="{size}"
        fill="{SNOW}" letter-spacing="-1" filter="url(#ambtext)">{esc(text)}</text>
</g>"""
    return defs, body

# ============================================================ 9 네온 사인
def neon_glow(text, kw, t, seed=0):
    """색을 하나로 줄이고 코어를 살렸다. 예전 보라·시안 이중 글로우보다 덜 요란하다."""
    fam, wt = ff("pretendard"), 800
    size = fit(text, fam, 100, wt)
    m, y = _base(text, fam, size, wt)
    op, dy = _fade(t)
    rnd = random.Random(int(t * 1000) + seed)
    pulse = 0.80 + 0.20 * math.sin(t * math.tau * 3)
    if t > 0.12 and rnd.random() < 0.05: pulse *= 0.42
    defs = f'''
  <filter id="neon" x="-60%" y="-70%" width="220%" height="240%" color-interpolation-filters="sRGB">
    <feGaussianBlur in="SourceAlpha" stdDeviation="{26*pulse:.1f}" result="b1"/>
    <feFlood flood-color="#22D3EE" flood-opacity="{0.9*pulse:.2f}"/><feComposite in2="b1" operator="in" result="n1"/>
    <feGaussianBlur in="SourceAlpha" stdDeviation="{8*pulse:.1f}" result="b2"/>
    <feFlood flood-color="#7DF9FF" flood-opacity="{0.95*pulse:.2f}"/><feComposite in2="b2" operator="in" result="n2"/>
    <feMerge><feMergeNode in="n1"/><feMergeNode in="n1"/><feMergeNode in="n2"/><feMergeNode in="SourceGraphic"/></feMerge>
  </filter>'''
    body = f'''<g opacity="{op:.3f}" transform="translate(0,{dy:.1f})">
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-weight="{wt}" font-size="{size}"
        fill="#EAFEFF" letter-spacing="-1" filter="url(#neon)">{esc(text)}</text>
</g>'''
    return defs, body

# ============================================================ 10 이리데센트
def iridescent(text, kw, t, seed=0):
    """금색 베벨 대신 홀로그램 쪽으로. 색이 흐르며 각도가 바뀐다."""
    fam = ff("paperlogy")
    size = fit(text, fam, 108, 400)
    m, y = _base(text, fam, size)
    op, dy = _fade(t)
    sh = (t * 0.9) % 1.0
    def st(o, c, a=1.0):
        return f'<stop offset="{clamp01(o):.3f}" stop-color="{c}" stop-opacity="{a}"/>'
    stops = "".join([st(-0.45 + sh, "#5EEAD4"), st(-0.18 + sh, "#818CF8"),
                     st(0.1 + sh, "#F472B6"), st(0.38 + sh, "#FDBA74"),
                     st(0.66 + sh, "#5EEAD4"), st(0.95 + sh, "#818CF8")])
    defs = f'''
  <linearGradient id="irid" x1="0" y1="0.15" x2="1" y2="0.85">{stops}</linearGradient>
  <filter id="iridsh" x="-30%" y="-30%" width="160%" height="170%" color-interpolation-filters="sRGB">
    <feGaussianBlur in="SourceAlpha" stdDeviation="16" result="b"/>
    <feFlood flood-color="#7C3AED" flood-opacity="0.55"/><feComposite in2="b" operator="in" result="g"/>
    <feMerge><feMergeNode in="g"/><feMergeNode in="SourceGraphic"/></feMerge></filter>'''
    body = f'''<g opacity="{op:.3f}" transform="translate(0,{dy:.1f})" filter="url(#iridsh)">
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-size="{size}"
        fill="url(#irid)" stroke="#FFFFFF" stroke-opacity="0.55" stroke-width="2"
        paint-order="stroke" stroke-linejoin="round">{esc(text)}</text>
</g>'''
    return defs, body

# ============================================================ 11 형광펜
def highlighter(text, kw, t, seed=0):
    fam = ff("paperlogy")
    size = fit(text, fam, 98, 400)
    m, y = _base(text, fam, size)
    x0 = (WIDTH - m["w"]) / 2
    idx = text.find(kw) if kw else -1
    if idx < 0: kw, idx = text.split(" ")[-1], len(text) - len(text.split(" ")[-1])
    pre_w = measure(text[:idx], fam, size)["w"] if idx else 0.0
    kw_w = measure(kw, fam, size)["w"]
    op, dy = _fade(t)
    mw = (kw_w + 14) * ease_out_cubic(clamp01((t - 0.22) / 0.34))
    mh = size * 0.46
    mx, my = x0 + pre_w - 7, y - size * 0.40
    defs = '<filter id="hlsoft" color-interpolation-filters="sRGB"><feGaussianBlur stdDeviation="1.6"/></filter>'
    body = f'''<g opacity="{op:.3f}" transform="translate(0,{dy:.1f})">
  <g transform="rotate(-1.1 {mx:.1f} {my:.1f})" filter="url(#hlsoft)">
    <rect x="{mx:.1f}" y="{my:.1f}" width="{mw:.1f}" height="{mh:.1f}" rx="{mh/2:.1f}" fill="{LIME}" fill-opacity="0.92"/>
  </g>
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-size="{size}"
        fill="{SNOW}" stroke="{INK}" stroke-width="9" paint-order="stroke" stroke-linejoin="round">{esc(text)}</text>
</g>'''
    return defs, body

# ============================================================ 12 글리치
def glitch(text, kw, t, seed=0):
    fam = ff("paperlogy")
    size = fit(text, fam, 104, 400)
    m, y = _base(text, fam, size)
    x0 = (WIDTH - m["w"]) / 2
    op, _ = _fade(t, rise=0, fin=0.06)
    rnd = random.Random(int(t * 900) * 7 + seed)
    burst = 1.0 if (rnd.random() < 0.40 or t < 0.14) else 0.5
    ox = (rnd.uniform(7, 19) * (1 if rnd.random() < 0.5 else -1)) * burst
    oy = rnd.uniform(-3, 3) * burst
    common = f'y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-size="{size}"'
    clips, slices = [], []
    for i in range(6):
        sy = y - size * 0.88 + i * (size * 1.1 / 6)
        sh = size * 1.1 / 6 + 1
        sx = rnd.uniform(-36, 36) * burst if rnd.random() < 0.68 else 0
        clips.append(f'<clipPath id="gs{i}"><rect x="0" y="{sy:.1f}" width="{WIDTH}" height="{sh:.1f}"/></clipPath>')
        slices.append(f'<g clip-path="url(#gs{i})"><text x="{WIDTH/2 + sx:.1f}" {common} fill="{SNOW}">{esc(text)}</text></g>')
    body = f'''<g opacity="{op:.3f}">
  <text x="{WIDTH/2 - ox:.1f}" y="{y + oy:.1f}" text-anchor="middle" font-family="{fam}" font-size="{size}"
        fill="#FF2D55" opacity="0.9">{esc(text)}</text>
  <text x="{WIDTH/2 + ox:.1f}" y="{y - oy:.1f}" text-anchor="middle" font-family="{fam}" font-size="{size}"
        fill="#00E5FF" opacity="0.9">{esc(text)}</text>
  {"".join(slices)}
  <rect x="{x0-30:.1f}" y="{y - size*0.86 + rnd.uniform(0,size):.1f}" width="{m['w']+60:.1f}" height="{3*burst:.1f}"
        fill="{SNOW}" opacity="{0.6*burst:.2f}"/>
</g>'''
    return "".join(clips), body

# ============================================================ 13 엠버 글로우
def ember(text, kw, t, seed=0):
    """불길이 글자 안쪽에서 솟아 글자를 감싼다. 위에만 얹으면 촛불처럼 떠 보인다."""
    fam = ff("paperlogy")
    size = fit(text, fam, 104, 400)
    m, y = _base(text, fam, size)
    op, dy = _fade(t)
    top = y - m["h"] * 0.90
    mid = top + m["h"] * 0.58          # 불길이 솟는 기준선을 글자 안으로
    x0 = (WIDTH - m["w"]) / 2
    rnd = random.Random(seed)

    def tongue(px, base, h, w, ph):
        hs = h * (1.0 + 0.30 * math.sin(t * math.tau * 2.4 + ph))
        ws = w * (1.0 + 0.16 * math.cos(t * math.tau * 1.5 + ph))
        lean = math.sin(t * math.tau * 1.8 + ph) * hs * 0.17
        return (f'<path d="M{px:.1f} {base:.1f} C {px-ws:.1f} {base-hs*0.32:.1f} '
                f'{px-ws*0.5+lean:.1f} {base-hs*0.7:.1f} {px+lean:.1f} {base-hs:.1f} '
                f'C {px+ws*0.6+lean:.1f} {base-hs*0.68:.1f} {px+ws:.1f} {base-hs*0.3:.1f} '
                f'{px:.1f} {base:.1f} Z" fill="url(#embt)"/>')

    n = max(9, int(m["w"] / 88))
    back = [tongue(x0 - 16 + (m["w"] + 32) * (i + 0.5) / n + rnd.uniform(-14, 14),
                   mid + m["h"] * 0.26, rnd.uniform(210, 320), rnd.uniform(44, 68),
                   rnd.uniform(0, math.tau)) for i in range(n)]
    nf = int(n * 1.6)
    front = [tongue(x0 - 22 + (m["w"] + 44) * (i + 0.5) / nf + rnd.uniform(-10, 10),
                    mid, rnd.uniform(105, 200), rnd.uniform(19, 34),
                    rnd.uniform(0, math.tau)) for i in range(nf)]
    sparks = []
    for i in range(16):
        sp = random.Random(seed * 131 + i * 17)
        bx = x0 + sp.random() * m["w"]
        life = (t * 1.3 + sp.random()) % 1.0
        r = 4.6 * (1 - life) * sp.uniform(0.5, 1.1)
        if r > 0.6:
            sparks.append(f'<circle cx="{bx + math.sin(life*7+i)*20:.1f}" cy="{top - 30 - life*250:.1f}" '
                          f'r="{r:.1f}" fill="#FFC978" opacity="{(1-life)*0.85:.2f}"/>')
    defs = """
  <radialGradient id="embt" cx="50%" cy="88%" r="68%">
    <stop offset="0" stop-color="#FFF6DC"/><stop offset="0.28" stop-color="#FFB443"/>
    <stop offset="0.62" stop-color="#FF6A2B" stop-opacity="0.85"/><stop offset="1" stop-color="#E0431B" stop-opacity="0"/>
  </radialGradient>
  <filter id="embsoft" color-interpolation-filters="sRGB"><feGaussianBlur stdDeviation="12"/></filter>
  <filter id="embsharp" color-interpolation-filters="sRGB"><feGaussianBlur stdDeviation="4.5"/></filter>
  <filter id="embglow" x="-60%" y="-70%" width="220%" height="250%" color-interpolation-filters="sRGB">
    <feGaussianBlur in="SourceAlpha" stdDeviation="30" result="b0"/>
    <feFlood flood-color="#FF5A1F" flood-opacity="0.8"/><feComposite in2="b0" operator="in" result="g0"/>
    <feGaussianBlur in="SourceAlpha" stdDeviation="9" result="b1"/>
    <feFlood flood-color="#FFB443" flood-opacity="0.8"/><feComposite in2="b1" operator="in" result="g1"/>
    <feMerge><feMergeNode in="g0"/><feMergeNode in="g0"/><feMergeNode in="g1"/><feMergeNode in="SourceGraphic"/></feMerge></filter>"""
    body = f"""<g opacity="{op:.3f}" transform="translate(0,{dy:.1f})">
  <g filter="url(#embsoft)" opacity="0.9">{''.join(back)}</g>
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-size="{size}"
        fill="#FFF8EC" stroke="#7A2400" stroke-width="7" paint-order="stroke" stroke-linejoin="round"
        filter="url(#embglow)">{esc(text)}</text>
  <g filter="url(#embsharp)" opacity="0.86">{''.join(front)}</g>
  {''.join(sparks)}
</g>"""
    return defs, body

# ============================================================ 14 스택 블록
def stack_block(text, kw, t, seed=0):
    """줄마다 제 배경을 갖고 왼쪽에서 밀려 들어온다. 인스타그램 결."""
    fam = ff("paperlogy")
    lines = text.split("|")
    size = min(fit(l, fam, 70, 400, maxw=760) for l in lines)
    cols = [(INK, SNOW), (LIME, "#10130A"), (INK, SNOW)]
    y0 = CY - (len(lines) - 1) * 46
    parts = []
    for i, ln in enumerate(lines):
        m = measure(ln, fam, size)
        bh = m["h"] + 30
        by = y0 + i * (bh + 12) - bh / 2
        bx = 120
        p = ease_out_cubic(clamp01((t - 0.06 - i * 0.12) / 0.34))
        bg, fg = cols[i % len(cols)]
        ty = by + bh / 2 + m["h"] / 2 - (m["h"] + m["top"])
        parts.append(
            f'<g opacity="{p:.2f}" transform="translate({(1-p)*-70:.1f},0)">'
            f'<rect x="{bx:.1f}" y="{by:.1f}" width="{m["w"]+52:.1f}" height="{bh:.1f}" rx="10" fill="{bg}" fill-opacity="0.94"/>'
            f'<text x="{bx+26:.1f}" y="{ty:.1f}" font-family="{fam}" font-size="{size}" fill="{fg}">{esc(ln)}</text></g>')
    op = 1 - clamp01((t - 0.9) / 0.1)
    return "", f'<g opacity="{op:.3f}">{"".join(parts)}</g>'

# ============================================================ 15 아웃라인
def outline(text, kw, t, seed=0):
    fam = ff("paperlogy")
    size = fit(text, fam, 104, 400)
    m, y = _base(text, fam, size)
    x0 = (WIDTH - m["w"]) / 2
    idx = text.find(kw) if kw else -1
    if idx < 0: kw, idx = text.split(" ")[0], 0
    pre_w = measure(text[:idx], fam, size)["w"] if idx else 0.0
    kw_w = measure(kw, fam, size)["w"]
    op, dy = _fade(t, rise=8)
    fillp = clamp01((t - 0.28) / 0.28)
    defs = f'''<clipPath id="olc"><rect x="{x0+pre_w-6:.1f}" y="{y-size:.1f}" width="{(kw_w+12)*ease_out_cubic(fillp):.1f}" height="{size*1.6:.1f}"/></clipPath>'''
    common = f'x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-size="{size}"'
    body = f'''<g opacity="{op:.3f}" transform="translate(0,{dy:.1f})">
  <text {common} fill="none" stroke="{SNOW}" stroke-width="3" stroke-linejoin="round">{esc(text)}</text>
  <g clip-path="url(#olc)"><text {common} fill="{LIME}">{esc(text)}</text></g>
</g>'''
    return defs, body

# ============================================================ 16 듀오톤 스플릿
def duotone(text, kw, t, seed=0):
    fam = ff("paperlogy")
    size = fit(text, fam, 108, 400)
    m, y = _base(text, fam, size)
    x0 = (WIDTH - m["w"]) / 2
    op, _ = _fade(t, rise=0, fin=0.14)
    top, h = y - size * 0.92, size * 1.24
    split = top + h * (0.34 + 0.30 * math.sin(t * math.tau * 0.85))
    defs = (f'<clipPath id="dtA"><rect x="{x0-40:.1f}" y="{top-40:.1f}" width="{m["w"]+80:.1f}" height="{split-top+40:.1f}"/></clipPath>'
            f'<clipPath id="dtB"><rect x="{x0-40:.1f}" y="{split:.1f}" width="{m["w"]+80:.1f}" height="{top+h-split+40:.1f}"/></clipPath>')
    common = f'x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-size="{size}"'
    body = f'''<g opacity="{op:.3f}">
  <text {common} fill="none" stroke="{INK}" stroke-width="10" paint-order="stroke" stroke-linejoin="round">{esc(text)}</text>
  <g clip-path="url(#dtA)"><text {common} fill="{SNOW}">{esc(text)}</text></g>
  <g clip-path="url(#dtB)"><text {common} fill="{LIME}">{esc(text)}</text></g>
  <rect x="{x0-30:.1f}" y="{split-1:.1f}" width="{m["w"]+60:.1f}" height="2" fill="{LIME}" opacity="0.5"/>
</g>'''
    return defs, body

# ============================================================ 17 팝 바운스
def pop_bounce(text, kw, t, seed=0):
    fam = ff("jua")
    size = fit(text, fam, 108, 400)
    ws, wd, gap, total = _words(text, fam, size)
    m, y = _base(text, fam, size)
    x = (WIDTH - total) / 2
    op = min(1.0, t / 0.08) * (1 - clamp01((t - 0.9) / 0.1))
    parts = []
    for i, (w, ww) in enumerate(zip(ws, wd)):
        p = clamp01((t - (0.05 + i * 0.13)) / 0.30)
        sc = ease_out_back(p) if p > 0 else 0.0
        cx, cy = x + ww / 2, y - size * 0.32
        rot = (1 - p) * (7 if i % 2 == 0 else -7)
        parts.append(
            f'<g transform="translate({cx:.1f},{cy:.1f}) rotate({rot:.1f}) scale({max(sc,0.001):.3f}) translate({-cx:.1f},{-cy:.1f})" opacity="{min(1,p*2.2):.2f}">'
            f'<text x="{x:.1f}" y="{y:.1f}" font-family="{fam}" font-size="{size}" fill="{SNOW}" '
            f'stroke="#FF3B6B" stroke-width="13" paint-order="stroke" stroke-linejoin="round">{esc(w)}</text>'
            f'<text x="{x:.1f}" y="{y:.1f}" font-family="{fam}" font-size="{size}" fill="{SNOW}">{esc(w)}</text></g>')
        x += ww + gap
    return "", f'<g opacity="{op:.3f}">{"".join(parts)}</g>'

# ============================================================ 18 스티커
def sticker(text, kw, t, seed=0):
    """붙였다 뗀 스티커처럼. 톡 튀어나오고 살짝 기울어 있다."""
    fam = ff("jua")
    size = fit(text, fam, 88, 400, maxw=800)
    m = measure(text, fam, size)
    pw, ph = m["w"] + 72, m["h"] + 46
    px, py = (WIDTH - pw) / 2, CY - ph / 2
    y = CY + m["h"] / 2 - (m["h"] + m["top"])
    p = clamp01(t / 0.26)
    sc = ease_out_back(p) if p > 0 else 0.001
    op = min(1.0, p * 2) * (1 - clamp01((t - 0.9) / 0.1))
    rot = -2.4 + (1 - p) * 8
    cx, cy = WIDTH / 2, CY
    defs = '''<filter id="stksh" x="-30%" y="-30%" width="160%" height="170%" color-interpolation-filters="sRGB">
      <feOffset in="SourceAlpha" dx="0" dy="8" result="o"/><feGaussianBlur in="o" stdDeviation="9" result="b"/>
      <feFlood flood-color="#000" flood-opacity="0.38"/><feComposite in2="b" operator="in" result="s"/>
      <feMerge><feMergeNode in="s"/><feMergeNode in="SourceGraphic"/></feMerge></filter>'''
    body = f'''<g opacity="{op:.3f}" filter="url(#stksh)"
     transform="translate({cx:.1f},{cy:.1f}) rotate({rot:.1f}) scale({max(sc,0.001):.3f}) translate({-cx:.1f},{-cy:.1f})">
  <rect x="{px:.1f}" y="{py:.1f}" width="{pw:.1f}" height="{ph:.1f}" rx="{ph*0.34:.1f}" fill="#FFFFFF"/>
  <rect x="{px+6:.1f}" y="{py+6:.1f}" width="{pw-12:.1f}" height="{ph-12:.1f}" rx="{ph*0.29:.1f}"
        fill="none" stroke="#FF3B6B" stroke-width="3" stroke-opacity="0.85"/>
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-size="{size}" fill="#1A1C22">{esc(text)}</text>
</g>'''
    return defs, body

# ============================================================ 19 버블 챗
def bubble(text, kw, t, seed=0):
    fam = ff("jua")
    size = fit(text, fam, 70, 400, maxw=740)
    m = measure(text, fam, size)
    pw, ph = m["w"] + 64, m["h"] + 40
    px = 130
    py = CY - ph / 2
    y = CY + m["h"] / 2 - (m["h"] + m["top"])
    p = ease_out_cubic(clamp01(t / 0.28))
    op = min(1.0, p * 1.6) * (1 - clamp01((t - 0.9) / 0.1))
    dy = (1 - p) * 34
    tail = f'M{px+34:.1f} {py+ph:.1f} L{px+30:.1f} {py+ph+26:.1f} L{px+72:.1f} {py+ph:.1f} Z'
    defs = '''<filter id="bbsh" x="-30%" y="-30%" width="160%" height="180%" color-interpolation-filters="sRGB">
      <feOffset in="SourceAlpha" dy="6" result="o"/><feGaussianBlur in="o" stdDeviation="10" result="b"/>
      <feFlood flood-color="#000" flood-opacity="0.42"/><feComposite in2="b" operator="in" result="s"/>
      <feMerge><feMergeNode in="s"/><feMergeNode in="SourceGraphic"/></feMerge></filter>'''
    body = f'''<g opacity="{op:.3f}" transform="translate(0,{dy:.1f})" filter="url(#bbsh)">
  <path d="{tail}" fill="{LIME}"/>
  <rect x="{px:.1f}" y="{py:.1f}" width="{pw:.1f}" height="{ph:.1f}" rx="{ph*0.42:.1f}" fill="{LIME}"/>
  <text x="{px+32:.1f}" y="{y:.1f}" font-family="{fam}" font-size="{size}" fill="#10130A">{esc(text)}</text>
</g>'''
    return defs, body

# ============================================================ 20 필름 자막
def film_caption(text, kw, t, seed=0):
    fam, wt = ff("nanummj"), 800
    size = fit(text, fam, 70, wt)
    m, y = _base(text, fam, size, wt)
    op = clamp01(t / 0.2) * (1 - clamp01((t - 0.86) / 0.14))
    defs = '''<filter id="filmsh" x="-25%" y="-25%" width="150%" height="170%" color-interpolation-filters="sRGB">
      <feOffset in="SourceAlpha" dy="2" result="o"/><feGaussianBlur in="o" stdDeviation="5" result="b"/>
      <feFlood flood-color="#000" flood-opacity="0.92"/><feComposite in2="b" operator="in" result="s"/>
      <feMerge><feMergeNode in="s"/><feMergeNode in="s"/><feMergeNode in="SourceGraphic"/></feMerge></filter>'''
    body = f'''<g opacity="{op:.3f}">
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-weight="{wt}" font-size="{size}"
        fill="#F2F2EE" letter-spacing="0.5" filter="url(#filmsh)">{esc(text)}</text>
</g>'''
    return defs, body

# ============================================================ 21 세리프 미니멀
def serif_minimal(text, kw, t, seed=0):
    """얇은 명조를 크게 쓰고 자간을 벌렸다. 장식은 위아래 선 두 줄뿐."""
    fam, wt, sp = ff("nanummj"), 400, 5
    size = fit(text, fam, 84, wt, sp)
    m, y = _base(text, fam, size, wt, sp)
    op, dy = _fade(t, rise=16, fin=0.28)
    lw = (m["w"] + 40) * ease_out_cubic(clamp01((t - 0.2) / 0.42))
    defs = '''<filter id="smsh" x="-25%" y="-25%" width="150%" height="170%" color-interpolation-filters="sRGB">
      <feOffset in="SourceAlpha" dy="2" result="o"/><feGaussianBlur in="o" stdDeviation="9" result="b"/>
      <feFlood flood-color="#000" flood-opacity="0.72"/><feComposite in2="b" operator="in" result="s"/>
      <feMerge><feMergeNode in="s"/><feMergeNode in="SourceGraphic"/></feMerge></filter>'''
    body = f'''<g opacity="{op:.3f}" transform="translate(0,{dy:.1f})" filter="url(#smsh)">
  <rect x="{WIDTH/2 - lw/2:.1f}" y="{y - size*1.15:.1f}" width="{lw:.1f}" height="1.4" fill="#E9E3D6" opacity="0.72"/>
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-weight="{wt}" font-size="{size}"
        letter-spacing="{sp}" fill="#F7F3EA">{esc(text)}</text>
  <rect x="{WIDTH/2 - lw/2:.1f}" y="{y + 44:.1f}" width="{lw:.1f}" height="1.4" fill="#E9E3D6" opacity="0.72"/>
</g>'''
    return defs, body

# ============================================================ 22 엘레강트 인용
def elegant_quote(text, kw, t, seed=0):
    fam, wt = ff("nanummj"), 800
    size = fit(text, fam, 76, wt)
    m, y = _base(text, fam, size, wt)
    op, dy = _fade(t, rise=20, fin=0.24)
    qp = ease_out_cubic(clamp01((t - 0.05) / 0.3))
    sub = "그 자리에 오래 머물렀다"
    defs = '''<filter id="eqsh" x="-25%" y="-30%" width="150%" height="180%" color-interpolation-filters="sRGB">
      <feOffset in="SourceAlpha" dy="3" result="o"/><feGaussianBlur in="o" stdDeviation="10" result="b"/>
      <feFlood flood-color="#000" flood-opacity="0.78"/><feComposite in2="b" operator="in" result="s"/>
      <feMerge><feMergeNode in="s"/><feMergeNode in="SourceGraphic"/></feMerge></filter>'''
    body = f'''<g opacity="{op:.3f}" transform="translate(0,{dy:.1f})" filter="url(#eqsh)">
  <text x="{WIDTH/2 - m["w"]/2 - 46:.1f}" y="{y - 30:.1f}" font-family="{fam}" font-weight="{wt}"
        font-size="{130*qp:.0f}" fill="{LIME}" opacity="{0.8*qp:.2f}">&#8220;</text>
  <text x="{WIDTH/2}" y="{y:.1f}" text-anchor="middle" font-family="{fam}" font-weight="{wt}" font-size="{size}"
        fill="#F8F5EE" letter-spacing="1">{esc(text)}</text>
  <text x="{WIDTH/2}" y="{y + 68:.1f}" text-anchor="middle" font-family="{ff('pretendard')}" font-weight="400"
        font-size="32" fill="#EAE2D4" letter-spacing="2" opacity="{clamp01((t-0.34)/0.3):.2f}">{esc(sub)}</text>
</g>'''
    return defs, body

# ============================================================ 목록
CATALOG = [
 dict(id="word_pop", fn=word_pop, name="워드 팝", font="pretendard", bg="warm_food",
      text="한 입 먹고 바로 인정", kw="인정",
      note="말하는 단어만 살아나고 나머지는 죽는다. 2026 숏폼에서 가장 많이 쓰이는 방식."),
 dict(id="keynote", fn=keynote, name="키노트", font="pretendard", bg="dark_studio",
      text="생각보다 조용한 가게", kw="조용한",
      note="얇게, 크게, 자간을 좁혀서. 애플 키노트가 쓰는 그 절제."),
 dict(id="blur_in", fn=blur_in, name="블러 인", font="pretendard", bg="dark_studio",
      text="여기서부터 진짜다", kw="진짜",
      note="흐림에서 선명으로. 요즘 브이로그 인트로에서 가장 흔한 등장."),
 dict(id="soft_ambient", fn=soft_ambient, name="소프트 앰비언트", font="pretendard", bg="dark_studio",
      text="분위기가 다 했다", kw="분위기",
      note="글자 뒤에서 색이 숨 쉬듯 번진다. 글자 자체는 건드리지 않는다."),
 dict(id="neon_glow", fn=neon_glow, name="네온 사인", font="pretendard", bg="night_neon",
      text="밤에만 여는 가게", kw="밤에만",
      note="색을 하나로 줄이고 코어를 살렸다. 이중 글로우보다 덜 요란하다."),
 dict(id="iridescent", fn=iridescent, name="이리데센트", font="paperlogy", bg="night_neon",
      text="이번 주 신메뉴", kw="신메뉴",
      note="금색 베벨 대신 홀로그램. 색이 글자를 따라 흐른다."),
 dict(id="glitch", fn=glitch, name="글리치", font="paperlogy", bg="dark_studio",
      text="이건 못 참지", kw="못 참지",
      note="적청이 어긋나고 가로 띠가 튄다. 프레임마다 다르게 깨진다."),
 dict(id="ember", fn=ember, name="엠버 글로우", font="paperlogy", bg="dark_studio",
      text="불맛이 미쳤다", kw="불맛",
      note="불길을 덜어내고 윗변에만 남겼다. 글자는 깨끗하게 둔다."),
 dict(id="stack_block", fn=stack_block, name="스택 블록", font="paperlogy", bg="busy_market",
      text="을지로 골목|3대째 이어온|노포 국밥", kw="노포",
      note="줄마다 제 배경을 갖고 차례로 밀려 들어온다."),
 dict(id="outline", fn=outline, name="아웃라인", font="paperlogy", bg="dark_studio",
      text="설명이 필요 없다", kw="설명",
      note="채우지 않고 외곽선만. 키워드에만 색이 들어찬다."),
 dict(id="pop_bounce", fn=pop_bounce, name="팝 바운스", font="jua", bg="bright_cafe",
      text="여기 진짜 좋아요", kw="진짜",
      note="단어가 하나씩 튀어나온다. 브이로그 톤."),
 dict(id="sticker", fn=sticker, name="스티커", font="jua", bg="bright_cafe",
      text="이거 꼭 먹어요", kw="",
      note="붙였다 뗀 스티커처럼 톡 튀어나오고 살짝 기울어 있다."),
 dict(id="bubble", fn=bubble, name="버블 챗", font="jua", bg="busy_market",
      text="사장님 이거 뭐예요", kw="",
      note="꼬리 달린 말풍선이 아래에서 올라온다."),
 dict(id="film_caption", fn=film_caption, name="필름 자막", font="nanummj", bg="busy_market",
      text="서울 을지로 골목 안쪽", kw="을지로",
      note="장식 없이 그림자만. 다큐 자막의 기본기."),
 dict(id="serif_minimal", fn=serif_minimal, name="세리프 미니멀", font="nanummj", bg="dark_studio",
      text="계절이 바뀌면 맛도 바뀐다", kw="계절",
      note="얇은 명조를 크게 쓰고 자간을 벌렸다. 장식은 선 두 줄뿐."),
]
