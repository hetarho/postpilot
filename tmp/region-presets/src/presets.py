# -*- coding: utf-8 -*-
"""인트로 10 · 아웃트로 10 시안 (+ 지금 쓰는 A·B / B·E).

프리셋은 슬롯의 역할(글자 크기)과 위치, 선·테두리 같은 중립 장식만 정한다. 무엇을 쓸지는
정하지 않는다: 고정 문구, 문구 이어 붙이기, 뜻이 담긴 아이콘(별·핀·체크·따옴표)은 없다.
각 슬롯에는 템플릿 문구가 순서대로 한 줄씩 들어가고, 모자라면 그 슬롯은 비운다.
"""
import json, math, os, sys
import lib
from lib import Spec, Stack, text_el, rect, hair, bar, brackets, num, CX, LEFT, MEASURE, OUT, STROKE_TEXT, STROKE_SMALL, MUTED, WHITE

HERE = os.path.dirname(os.path.abspath(__file__))
C = json.load(open(os.path.join(HERE, "content.json")))
FRAMES = os.path.join(OUT, "frames")
os.makedirs(FRAMES, exist_ok=True)
LOG = lib.FIT_LOG


def lab(**kw):
    return Spec("label", stroke=kw.pop("stroke", STROKE_SMALL), **kw)


def at(lines, i):
    return lines[i] if i < len(lines) else ""


def flanked(value, spec, rule_w=56, gap=22):
    """양옆에 짧은 가로선을 단 한 줄 (가운데 정렬)."""
    inner = Spec(spec.role, face=spec.face, size=spec.size, weight=spec.weight, tracking=spec.tracking,
                 floor=spec.floor, max_width=MEASURE - 2 * (rule_w + gap), fill=spec.fill, stroke=spec.stroke)
    f = lib.fit(value, inner)
    LOG.append(f)
    top, bottom = lib.ink(spec.face, spec.weight)
    h = (top + bottom) * f["size"]

    def draw(x, y):
        svg, w = text_el(f["lines"][0], x, y + top * f["size"], f["size"], spec, f["tracking"])
        mid = y + h * 0.52
        return svg + hair(0, mid, rule_w, 0.8, x=x - w / 2 - gap - rule_w) + hair(0, mid, rule_w, 0.8, x=x + w / 2 + gap)
    return h, draw


def dot_row(value, spec, r=8, gap=20):
    inner = Spec(spec.role, face=spec.face, size=spec.size, weight=spec.weight, tracking=spec.tracking,
                 floor=spec.floor, max_width=MEASURE - 2 * r - gap, fill=spec.fill, stroke=spec.stroke)
    f = lib.fit(value, inner)
    LOG.append(f)
    top, bottom = lib.ink(spec.face, spec.weight)
    h = (top + bottom) * f["size"]

    def draw(x, y):
        dot = '<circle cx="%s" cy="%s" r="%d" fill="#FFFFFF" filter="url(#sh)"/>' % (num(x + r), num(y + h * 0.5), r)
        return dot + text_el(f["lines"][0], x + 2 * r + gap, y + top * f["size"], f["size"], spec, f["tracking"], "start")[0]
    return h, draw


# ================================================================ 지금 쓰는 프리셋 (간격 유지로 옮긴 것)
def intro_A(L):
    st = Stack()
    st.text(at(L, 0), Spec("headline", stroke=STROKE_TEXT))
    st.text(at(L, 1), lab(), gap=48)
    return st.render(cy=941.6), None


def intro_B(L):
    st = Stack()
    st.rule("hair")
    st.text(at(L, 0), Spec("hook"), gap=43)
    st.rule("hair", gap=46)
    st.text(at(L, 1), lab(stroke=0), gap=50)
    return st.render(cy=969), None


def outro_B(L):
    st = Stack()
    st.text(at(L, 0), Spec("hook"))
    st.rule("hair", gap=46)
    st.text(at(L, 1), Spec("body", stroke=STROKE_SMALL), gap=44)
    return st.render(cy=937.5), None


def outro_E(L):
    st = Stack()
    st.text(at(L, 0), lab(tracking=0.08))
    st.text(at(L, 1), Spec("display", stroke=STROKE_TEXT, sample="4.8 0123456789"), gap=31)
    st.rule("bar", gap=42)
    st.text(at(L, 2), Spec("caption", stroke=STROKE_SMALL, fill=("#FFFFFF", 0.8)), gap=42)
    return st.render(cy=960.5), None


# ================================================================ 인트로 시안
def i01_kicker(L):
    st = Stack()
    st.text(at(L, 0), lab(tracking=0.14))
    st.text(at(L, 1), Spec("headline", stroke=STROKE_TEXT), gap=24)
    st.rule("bar", gap=34, w=96)
    return st.render(cy=960), None


def i02_cover(L):
    st = Stack(x=LEFT, anchor="start")
    st.rule("bar", w=64)
    st.text(at(L, 0), lab(tracking=0.12, fill=WHITE), gap=30)
    st.text(at(L, 1), Spec("headline", size=124, floor=76, stroke=0), gap=22)
    st.text(at(L, 2), Spec("caption", fill=MUTED), gap=24)
    return st.render(top=200), "top"


def i03_serif(L):
    st = Stack()
    if at(L, 0):
        h, draw = flanked(at(L, 0), lab(tracking=0.16, fill=WHITE))
        st.custom(h, draw)
    st.text(at(L, 1), Spec("headline", face="myeongjo", size=108, floor=66, tracking=-0.03, stroke=STROKE_SMALL), gap=30)
    st.text(at(L, 2), Spec("caption", fill=MUTED), gap=30)
    return st.render(cy=960), None


def i04_frame(L):
    st = Stack()
    st.text(at(L, 0), Spec("headline", stroke=0, max_width=MEASURE - 112),
            frame=dict(kind="outline", pad_h=56, pad_v=44, stroke=3, alpha=0.95, min_w=420))
    st.text(at(L, 1), lab(), gap=34)
    return st.render(cy=960), None


def i05_outline(L):
    st = Stack()
    st.text(at(L, 0), Spec("display", size=260, floor=150, tracking=-0.02, stroke=0, shadow=True, outline=(4, 1)))
    st.text(at(L, 1), Spec("headline", stroke=STROKE_TEXT), gap=40)
    st.text(at(L, 2), lab(), gap=30)
    return st.render(cy=930), ("radial", 930, 470)


def i06_lower_third(L):
    st = Stack(x=LEFT, anchor="start")
    if at(L, 0):
        h, draw = dot_row(at(L, 0), lab(fill=WHITE, tracking=0.04))
        st.custom(h, draw)
    st.text(at(L, 1), Spec("hook", stroke=0), gap=22)
    st.rule("hair", gap=28, w=360, alpha=0.8)
    st.text(at(L, 2), Spec("caption", fill=MUTED, stroke=0), gap=26)
    return st.render(bottom=1372), "bottom"


def i07_brackets(L):
    st = Stack()
    st.text(at(L, 0), Spec("headline", stroke=STROKE_TEXT, max_width=MEASURE - 96))
    st.text(at(L, 1), lab(), gap=36)
    body = st.render(cy=960)
    top, bottom, w = st.box
    bw = max(w + 96, 520)
    return body + brackets(CX - bw / 2, top - 52, bw, bottom - top + 104, 48, 5), None


def i08_fill(L):
    """슬롯 1이 줄마다 856px 폭을 가득 채운다(최대 240px). 길면 어절로 줄을 나눠 줄마다 따로 맞춘다."""
    spec = Spec("headline", stroke=STROKE_TEXT)
    value = at(L, 0)
    words = value.split(" ")
    size1 = min(240, 100 * MEASURE / spec.width(value, 100))
    groups = [value]
    if size1 < 150 and len(words) > 1:
        groups = lib.balanced_split(value, spec, 2)
        if min(100 * MEASURE / spec.width(g, 100) for g in groups) < 110 and len(words) >= 3:
            groups = lib.balanced_split(value, spec, 3)
    top, bottom = lib.ink("paperlogy", 800)
    rows = [(g, min(240, 100 * MEASURE / spec.width(g, 100))) for g in groups]
    LOG.extend(dict(lines=[g], size=s, over=False) for g, s in rows)
    gap = 18
    total = sum((top + bottom) * s for _, s in rows) + gap * (len(rows) - 1)
    h2, draw2 = flanked(at(L, 1), lab(tracking=0.16, fill=WHITE)) if at(L, 1) else (0, None)
    y = 960 - (total + (40 + h2 if draw2 else 0)) / 2
    out = []
    for g, s in rows:
        out.append(text_el(g, CX, y + top * s, s, spec)[0])
        y += (top + bottom) * s + gap
    if draw2:
        out.append(draw2(CX, y - gap + 40))
    return "".join(out), None


def i09_split(L):
    """상하 분리: 슬롯 1은 화면 위, 슬롯 2는 화면 아래. 가운데는 피사체에 비워 둔다."""
    top = Stack()
    top.text(at(L, 0), Spec("headline", stroke=0))
    bottom = Stack()
    bottom.text(at(L, 1), Spec("caption", weight=700, stroke=0))
    body = top.render(top=200) + bottom.render(bottom=1372)
    return body, "both"


def i10_sticker(L):
    st = Stack()
    st.text(at(L, 0), Spec("headline", face="jua", weight=400, size=128, floor=78, tracking=-0.01, stroke=14,
                           max_width=MEASURE - 40))
    st.text(at(L, 1), Spec("label", weight=700, fill=WHITE, stroke=0, shadow=False, max_width=MEASURE - 56),
            gap=34, frame=dict(kind="plate", pad_h=28, pad_v=18, radius=12))
    return '<g transform="rotate(-4 540 960)">%s</g>' % st.render(cy=960), None


# ================================================================ 아웃트로 시안
def o01_pill(L):
    st = Stack()
    st.text(at(L, 0), Spec("hook", stroke=0))
    st.text(at(L, 1), Spec("caption", weight=700, size=42, floor=36, max_width=MEASURE - 2 * 48),
            gap=48, frame=dict(kind="pill", pad_h=48, pad_v=26, stroke=3, alpha=1))
    return st.render(cy=960), None


def o02_list(L):
    """판 위 목록: 슬롯 1은 머리글, 슬롯 2~5는 왼쪽 맞춤 한 줄씩, 줄 사이 가는 선."""
    pw = 800
    head = Spec("label", tracking=0.14, stroke=0, shadow=False, fill=WHITE, max_width=pw - 96)
    row = Spec("caption", weight=700, size=40, floor=32, stroke=0, shadow=False, max_width=pw - 96)
    rows = [v for v in L[1:5] if v]
    rh = 78
    ph = 40 + 36 + 34 + len(rows) * rh + 20
    y0, x0 = 960 - ph / 2, CX - pw / 2
    out = [rect(x0, y0, pw, ph, lib.PLATE[0], lib.PLATE[1], 16)]
    t, _ = lib.ink("wantedsans", 600)
    fh = lib.fit(at(L, 0), head)
    LOG.append(fh)
    out.append(text_el(fh["lines"][0], CX, y0 + 40 + t * fh["size"], fh["size"], head)[0])
    y = y0 + 40 + 36 + 34
    out.append(hair(CX, y, pw - 96, 0.5))
    for i, v in enumerate(rows):
        f = lib.fit(v, row)
        LOG.append(f)
        out.append(text_el(v, x0 + 48, y + rh / 2 + 14, f["size"], row, anchor="start")[0])
        y += rh
        if i < len(rows) - 1:
            out.append(hair(CX, y, pw - 96, 0.18))
    return "".join(out), None


def o03_columns(L):
    """두 단: 왼쪽에 큰 슬롯 1, 세로 가는 선, 오른쪽에 작은 슬롯 2·3."""
    col = (MEASURE - 64) / 2
    left = Stack(x=CX - 32, anchor="end")
    left.text(at(L, 0), Spec("title", size=76, floor=52, stroke=0, max_width=col, lines=3))
    right = Stack(x=CX + 32, anchor="start")
    right.text(at(L, 1), Spec("caption", weight=700, stroke=0, max_width=col, lines=2))
    right.text(at(L, 2), lab(stroke=0, max_width=col, lines=2), gap=18)
    h = max(left.height(), right.height())
    body = left.render(cy=960) + right.render(cy=960)
    return body + rect(CX - 1, 960 - h / 2 - 12, 2, h + 24, "#FFFFFF", 0.8), None


def o04_credits(L):
    st = Stack()
    st.text(at(L, 0), lab(tracking=0.2, fill=MUTED, stroke=0))
    st.text(at(L, 1), Spec("headline", face="myeongjo", size=104, floor=64, tracking=-0.03, stroke=STROKE_SMALL), gap=30)
    st.rule("hair", gap=36, w=120, alpha=0.9)
    st.text(at(L, 2), lab(stroke=0, fill=WHITE), gap=36)
    st.text(at(L, 3), lab(stroke=0), gap=16)
    return st.render(cy=960), None


def o05_band(L):
    """풀폭 띠: 화면 가로 전체에 어두운 띠를 깔고 그 안에 슬롯 1·2."""
    st = Stack()
    st.text(at(L, 0), Spec("hook", stroke=0, shadow=False))
    st.text(at(L, 1), Spec("caption", fill=MUTED, stroke=0, shadow=False), gap=26)
    h = st.height()
    band = rect(0, 960 - h / 2 - 64, 1080, h + 128, lib.PLATE[0], 0.72)
    edges = hair(CX, 960 - h / 2 - 64, 1080, 0.5) + hair(CX, 960 + h / 2 + 64, 1080, 0.5)
    return band + edges + st.render(cy=960), "none"


def o06_side_bar(L):
    """아래·왼쪽 세로 막대: 블록 왼쪽에 흰 막대, 오른쪽에 슬롯 1~3 왼쪽 맞춤."""
    st = Stack(x=LEFT + 34, anchor="start")
    wide = MEASURE - 34
    st.text(at(L, 0), Spec("hook", stroke=0, max_width=wide))
    st.text(at(L, 1), Spec("caption", weight=700, stroke=0, max_width=wide), gap=22)
    st.text(at(L, 2), lab(stroke=0, max_width=wide), gap=20)
    body = st.render(bottom=1372)
    top, bottom, _ = st.box
    return rect(LEFT, top, 6, bottom - top, "#FFFFFF", 1) + body, "bottom"


def o07_chips(L):
    """슬롯 1 한 줄 + 슬롯 2~6을 알약 칩으로. 칩이 넘치면 다음 줄로 넘긴다."""
    st = Stack()
    st.text(at(L, 0), Spec("hook", stroke=0))
    chips = [v for v in L[1:6] if v]
    chip = Spec("label", size=36, floor=30, stroke=0, shadow=True, fill=WHITE)
    ph, pv, g = 26, 14, 14
    t, b = lib.ink("wantedsans", 600)
    size = chip.size
    lines, cur = [], []
    for v in chips:
        w = chip.width(v, size) + 2 * ph
        if cur and sum(x for _, x in cur) + g * len(cur) + w > MEASURE:
            lines.append(cur)
            cur = []
        cur.append((v, w))
    if cur:
        lines.append(cur)
    ch = (t + b) * size + 2 * pv
    total = len(lines) * ch + max(0, len(lines) - 1) * 14

    def draw(x, y):
        out = []
        for i, line in enumerate(lines):
            lw = sum(w for _, w in line) + g * (len(line) - 1)
            cx, yy = x - lw / 2, y + i * (ch + 14)
            for v, w in line:
                out.append('<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="#000" fill-opacity="0.28" stroke="#FFFFFF" '
                           'stroke-opacity="0.85" stroke-width="2"/>' % (num(cx), num(yy), num(w), num(ch), num(ch / 2)))
                out.append(text_el(v, cx + w / 2, yy + pv + t * size, size, chip)[0])
                cx += w + g
        return "".join(out)
    if lines:
        st.custom(total, draw, gap=44, width=MEASURE)
    return st.render(cy=960), ("radial", 970, 380)


def o08_huge(L):
    """초대형 한 줄: 가는 선 사이에 작은 슬롯 1, 화면 폭을 채우는 슬롯 2, 아래 슬롯 3."""
    st = Stack()
    st.rule("hair")
    st.text(at(L, 0), lab(tracking=0.08, fill=WHITE), gap=40)
    st.text(at(L, 1), Spec("display", size=150, floor=84, tracking=0.0, stroke=STROKE_TEXT, lines=1,
                           sample="0123456789 한우"), gap=30)
    st.rule("hair", gap=40)
    st.text(at(L, 2), Spec("caption", stroke=STROKE_SMALL, fill=("#FFFFFF", 0.85)), gap=36)
    return st.render(cy=960), None


def o09_list(L):
    """세 줄 목록: 슬롯 1 제목 + 슬롯 2~4를 작은 네모 표시와 함께 왼쪽 맞춤, 묶음은 가운데."""
    st = Stack()
    st.text(at(L, 0), Spec("hook", stroke=0))
    items = [v for v in L[1:4] if v]
    row = Spec("body", size=50, floor=40, stroke=STROKE_SMALL, max_width=MEASURE - 56)
    fits = [lib.fit(v, row) for v in items]
    LOG.extend(fits)
    if items:
        size = min(f["size"] for f in fits)
        t, b = lib.ink("wantedsans", 700)
        rh = (t + b) * size
        gw = max(row.width(v, size) for v in items) + 56
        total = len(items) * rh + (len(items) - 1) * 30

        def draw(x, y):
            out = []
            x0 = x - gw / 2
            for i, v in enumerate(items):
                yy = y + i * (rh + 30)
                out.append(rect(x0, yy + rh / 2 - 7, 14, 14, "#FFFFFF", 1, 2, ' filter="url(#sh)"'))
                out.append(text_el(v, x0 + 56, yy + t * size, size, row, anchor="start")[0])
            return "".join(out)
        st.custom(total, draw, gap=48, width=gw)
    return st.render(cy=960), ("radial", 980, 420)


def o10_stamp(L):
    """원형 도장: 슬롯 1은 위쪽 띠를 따라, 슬롯 2는 가운데, 슬롯 3은 아래쪽 띠, 슬롯 4는 도장 아래."""
    R1, R2, cy = 250, 192, 900
    big = Spec("hook", size=92, floor=56, stroke=0, max_width=2 * R2 - 64)
    ring = Spec("label", size=30, floor=22, tracking=0.14, stroke=0, shadow=False, fill=WHITE, max_width=560)
    f = lib.fit(at(L, 1) or " ", big)
    fs, fp = lib.fit(at(L, 0) or " ", ring), lib.fit(at(L, 2) or " ", ring)
    LOG.extend([f, fs, fp])
    t, b = lib.ink("paperlogy", 800)
    lh = f["size"] * big.line_height
    th = (t + b) * f["size"] + (len(f["lines"]) - 1) * lh
    rt, rb = R2 + 14, R1 - 14
    out = ['<defs><path id="arcTop" d="M%s %s A%s %s 0 0 1 %s %s"/><path id="arcBot" d="M%s %s A%s %s 0 0 0 %s %s"/></defs>'
           % (num(CX - rt), cy, rt, rt, num(CX + rt), cy, num(CX - rb), cy, rb, rb, num(CX + rb), cy),
           '<g filter="url(#sh)"><circle cx="540" cy="%d" r="%d" fill="#000" fill-opacity="0.22" stroke="#FFFFFF" stroke-width="5"/>' % (cy, R1),
           '<circle cx="540" cy="%d" r="%d" fill="none" stroke="#FFFFFF" stroke-width="2.5"/></g>' % (cy, R2)]
    y = cy - th / 2 + t * f["size"]
    for i, line in enumerate(f["lines"] if at(L, 1) else []):
        out.append(text_el(line, CX, y + i * lh, f["size"], big, f["tracking"])[0])
    for pid, ff, value in (("arcTop", fs, at(L, 0)), ("arcBot", fp, at(L, 2))):
        if value:
            out.append('<text font-family="%s" font-size="%s" font-weight="600" letter-spacing="%s" fill="#FFFFFF" '
                       'text-anchor="middle" filter="url(#sh)"><textPath href="#%s" startOffset="50%%">%s</textPath></text>'
                       % (lib.FACE["wantedsans"], num(ff["size"]), num(0.14 * ff["size"]), pid, lib.esc(value)))
    for a in (180, 0):
        out.append('<circle cx="%s" cy="%d" r="5" fill="#FFFFFF"/>' % (num(CX + (R1 + R2) / 2 * math.cos(math.radians(a))), cy))
    stamp = '<g transform="rotate(-8 540 %d)">%s</g>' % (cy, "".join(out))
    st = Stack()
    st.text(at(L, 3), Spec("caption", weight=700, stroke=STROKE_SMALL))
    return stamp + st.render(top=cy + R1 + 64), ("radial", 960, 480)


# 프리셋: id, 이름, 그리기, 슬롯(역할, px), 위치 설명, 예시 문구 키(슬롯 순서)
P = lambda **kw: kw
INTROS = [
    P(id="A", title="현재 A · 크기만", fn=intro_A, slots=[("헤드라인", 96), ("라벨", 36)], where="가운데 · 블록 중심 y 942",
      keys=["name", "sub"]),
    P(id="B", title="현재 B · 위아래 가로선", fn=intro_B, slots=[("훅", 84), ("라벨", 36)], where="가운데 · 가는 선 두 줄 사이 · 중심 y 969",
      keys=["name", "sub"]),
    P(id="i01", title="킥커 + 바", fn=i01_kicker, slots=[("라벨", 36), ("헤드라인", 96)], where="가운데 · 아래 짧은 흰 바 · 중심 y 960",
      keys=["place", "name"]),
    P(id="i02", title="매거진 커버", fn=i02_cover, slots=[("라벨", 36), ("헤드라인", 124), ("캡션", 44)], where="위·왼쪽 x 96 · 위 끝 y 200 · 위쪽 스크림",
      keys=["kicker", "name", "what"]),
    P(id="i03", title="명조 매거진", fn=i03_serif, slots=[("라벨", 36), ("명조 헤드라인", 108), ("캡션", 44)], where="가운데 · 슬롯 1 양옆 짧은 선 · 중심 y 960",
      keys=["place", "name", "what"]),
    P(id="i04", title="프레임 박스", fn=i04_frame, slots=[("헤드라인", 96), ("라벨", 36)], where="가운데 · 슬롯 1을 감싸는 테두리 · 중심 y 960",
      keys=["name", "sub"]),
    P(id="i05", title="큰 외곽선 글자", fn=i05_outline, slots=[("외곽선 디스플레이", 260), ("헤드라인", 96), ("라벨", 36)], where="가운데 · 중심 y 930",
      keys=["big", "name", "sub"]),
    P(id="i06", title="로어서드", fn=i06_lower_third, slots=[("라벨", 36), ("훅", 84), ("캡션", 44)], where="아래·왼쪽 x 96 · 아래 끝 y 1372 · 아래쪽 스크림",
      keys=["live", "name", "sub"]),
    P(id="i07", title="코너 브래킷", fn=i07_brackets, slots=[("헤드라인", 96), ("라벨", 36)], where="가운데 · 네 귀퉁이 선 · 중심 y 960",
      keys=["name", "sub"]),
    P(id="i08", title="꽉 찬 제목", fn=i08_fill, slots=[("폭 채움 헤드라인", 240), ("라벨", 36)], where="가운데 · 슬롯 1이 줄마다 856px 폭을 채움 · 중심 y 960",
      keys=["name", "place"]),
    P(id="i09", title="상하 분리", fn=i09_split, slots=[("헤드라인", 96), ("캡션", 44)], where="슬롯 1 위(y 200부터) · 슬롯 2 아래(y 1372까지) · 위아래 스크림",
      keys=["name", "sub"]),
    P(id="i10", title="주아 스티커", fn=i10_sticker, slots=[("주아 헤드라인", 128), ("라벨", 36)], where="가운데 · −4° 기울임 · 슬롯 2는 어두운 판 · 중심 y 960",
      keys=["name", "sub"]),
]
OUTROS = [
    P(id="B", title="현재 B · 가로선 구분", fn=outro_B, slots=[("훅", 84), ("본문", 56)], where="가운데 · 사이 가는 선 · 중심 y 938",
      keys=["hook", "cta"]),
    P(id="E", title="현재 E · 점수 강조", fn=outro_E, slots=[("라벨", 36), ("디스플레이", 132), ("캡션", 44)], where="가운데 · 슬롯 2 아래 흰 바 · 중심 y 961",
      keys=["score_label", "score", "e_caption"]),
    P(id="o01", title="알약 테두리", fn=o01_pill, slots=[("훅", 84), ("캡션", 42)], where="가운데 · 슬롯 2를 감싸는 알약 테두리 · 중심 y 960",
      keys=["hook", "cta"]),
    P(id="o02", title="판 위 목록", fn=o02_list, slots=[("라벨", 36), ("캡션", 40), ("캡션", 40), ("캡션", 40), ("캡션", 40)],
      where="가운데 · 어두운 판 800px · 슬롯 2~5 왼쪽 맞춤 · 중심 y 960", keys=["kicker", "name", "place", "price_line", "hours"]),
    P(id="o03", title="두 단", fn=o03_columns, slots=[("타이틀", 76), ("캡션", 44), ("라벨", 36)], where="가운데 세로 선 기준 · 왼쪽 슬롯 1 · 오른쪽 슬롯 2·3 · 중심 y 960",
      keys=["hook", "name", "place"]),
    P(id="o04", title="엔딩 크레딧", fn=o04_credits, slots=[("라벨", 36), ("명조 헤드라인", 104), ("라벨", 36), ("라벨", 36)], where="가운데 · 슬롯 2 아래 짧은 선 · 중심 y 960",
      keys=["kicker", "name", "place", "hours"]),
    P(id="o05", title="풀폭 띠", fn=o05_band, slots=[("훅", 84), ("캡션", 44)], where="화면 가로 전체 어두운 띠 · 중심 y 960",
      keys=["hook", "cta"]),
    P(id="o06", title="세로 막대 로어서드", fn=o06_side_bar, slots=[("훅", 84), ("캡션", 44), ("라벨", 36)], where="아래·왼쪽 · 왼쪽 흰 막대 · 아래 끝 y 1372 · 아래쪽 스크림",
      keys=["name", "place", "place_cta"]),
    P(id="o07", title="칩 줄", fn=o07_chips, slots=[("훅", 84)] + [("칩 라벨", 36)] * 5, where="가운데 · 슬롯 2~6 알약 칩, 넘치면 다음 줄 · 중심 y 960",
      keys=["summary", "tags"]),
    P(id="o08", title="초대형 한 줄", fn=o08_huge, slots=[("라벨", 36), ("디스플레이", 150), ("캡션", 44)], where="가운데 · 가는 선 두 줄 사이 · 중심 y 960",
      keys=["small_top", "big_line", "name"]),
    P(id="o09", title="세 줄 목록", fn=o09_list, slots=[("훅", 84), ("본문", 50), ("본문", 50), ("본문", 50)], where="가운데 · 슬롯 2~4 네모 표시 + 왼쪽 맞춤 묶음 · 중심 y 960",
      keys=["title", "points"]),
    P(id="o10", title="원형 도장", fn=o10_stamp, slots=[("띠 라벨", 30), ("훅", 92), ("띠 라벨", 30), ("캡션", 44)], where="도장 중심 y 900 · −8° · 슬롯 1·3은 원 띠를 따라 · 슬롯 4 도장 아래",
      keys=["name", "stamp", "place", "cta"]),
]


def lines_for(preset, content):
    out = []
    for k in preset["keys"]:
        v = content[k]
        out.extend(v if isinstance(v, list) else [v])
    return out[:len(preset["slots"])]


def render(kind, presets, bg, which):
    cells = []
    for p in presets:
        del LOG[:]
        if which == "slots":
            lines = ["슬롯 %d" % (i + 1) for i in range(len(p["slots"]))]
        else:
            lines = lines_for(p, C[kind][which])
        body, scrim = p["fn"](lines)
        if scrim is None:
            scrim = ("radial", 960, 400)
        elif scrim == "none":
            scrim = None
        name = "%s-%s-%s" % (kind, p["id"], which)
        svg = lib.frame(body, bg, name, scrim)
        png = os.path.join(FRAMES, name + ".png")
        lib.bake(svg, png)
        over = any(f.get("over") for f in LOG if f)
        cells.append(dict(id=p["id"], title=p["title"], slots=p["slots"], where=p["where"], png=png, over=over,
                          lines=lines, sizes=[round(f["size"]) for f in LOG if f]))
        print(name, "over" if over else "", [(round(f["size"]), len(f["lines"])) for f in LOG if f])
    return cells


if __name__ == "__main__":
    only = sys.argv[1:] or ["intro", "outro"]
    try:
        result = json.load(open(os.path.join(OUT, "frames.json")))
    except Exception:
        result = {}
    for kind in only:
        presets = INTROS if kind == "intro" else OUTROS
        bg = "plate.jpg" if kind == "intro" else "grill.jpg"
        for which in ("short", "long", "slots"):
            result["%s-%s" % (kind, which)] = render(kind, presets, bg, which)
    with open(os.path.join(OUT, "frames.json"), "w") as fh:
        json.dump(result, fh, ensure_ascii=False, indent=1)
