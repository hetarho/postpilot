# -*- coding: utf-8 -*-
"""인트로·아웃트로 프리셋 시안 공통부: 번들 폰트로 resvg 측정, 폭 맞춤, SVG 조각."""
import glob, os, subprocess, json, math

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
REPO = os.path.dirname(os.path.dirname(ROOT))
OUT = os.path.join(ROOT, "out")
FONT_ARGS = []
for _f in sorted(glob.glob(os.path.join(REPO, "backend/assets/fonts/*/*.ttf"))):
    FONT_ARGS += ["--use-font-file", _f]

W, H = 1080, 1920
CX = 540
MEASURE = 856          # CDS-9 copy_max_width (9:16)
LEFT = 96              # anchor.left
SAFE_TOP, SAFE_BOTTOM = 40, 1420

# design.json 토큰 그대로
FACE = {
    "paperlogy": "Paperlogy",
    "wantedsans": "Wanted Sans Variable",
    "jua": "Jua",
    "myeongjo": "NanumMyeongjo",
}
ROLE = {
    #            face          size  weight tracking  floor
    "display":  ("paperlogy",  132,  800,   0.04,     80),
    "headline": ("paperlogy",   96,  800,  -0.02,     60),
    "hook":     ("paperlogy",   84,  800,  -0.02,     56),
    "title":    ("paperlogy",   72,  800,  -0.02,     52),
    "body":     ("wantedsans",  56,  700,  -0.01,     48),
    "caption":  ("wantedsans",  44,  600,   0.00,     40),
    "label":    ("wantedsans",  36,  600,   0.02,     34),
}
WHITE, MUTED = ("#FFFFFF", 1.0), ("#FFFFFF", 0.72)
STROKE = ("#0B0B0B", 0.85)
STROKE_TEXT, STROKE_SMALL = 6, 4
HAIR = dict(w=520, h=2, a=0.55)
BAR = dict(w=160, h=6, a=1.0)
PLATE = ("#111111", 0.8)


def esc(s):
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def num(v):
    return ("%.2f" % v).rstrip("0").rstrip(".")


# ------------------------------------------------------------------ 측정
_CACHE_FILE = os.path.join(ROOT, "src", ".measure-cache.json")
try:
    with open(_CACHE_FILE) as fh:
        _CACHE = json.load(fh)
except Exception:
    _CACHE = {}


def _key(text, face, weight, tracking):
    return "%s|%d|%.4f|%s" % (face, weight, tracking, text)


def prefetch(items):
    """(text, face, weight, tracking) 여러 개를 resvg 한 번으로 잰다. 100px 기준 advance 폭."""
    todo = [it for it in dict.fromkeys(items) if _key(*it) not in _CACHE]
    if not todo:
        return
    parts = ['<svg xmlns="http://www.w3.org/2000/svg" width="6000" height="%d">' % (200 * len(todo) + 200)]
    for i, (text, face, weight, tracking) in enumerate(todo):
        parts.append('<text id="m%d" x="100" y="%d" font-family="%s" font-size="100" font-weight="%d" '
                     'letter-spacing="%s" xml:space="preserve">%s</text>'
                     % (i, 200 * i + 150, FACE[face], weight, num(tracking * 100), esc(text)))
    parts.append("</svg>")
    p = subprocess.run(["resvg", "--skip-system-fonts"] + FONT_ARGS + ["--query-all", "-"],
                       input="".join(parts).encode(), capture_output=True)
    got = {}
    for line in p.stdout.decode().splitlines():
        if line.startswith("m"):
            k, x, y, w, h = line.split(",")
            got[int(k[1:])] = float(w)
    for i, it in enumerate(todo):
        _CACHE[_key(*it)] = got.get(i, 0.0)
    with open(_CACHE_FILE, "w") as fh:
        json.dump(_CACHE, fh, ensure_ascii=False)


def adv(text, face, weight, tracking):
    """100px 기준 폭. letter-spacing 이 마지막 글자 뒤에도 붙으므로 잉크 폭으로 쓰려면 한 번 뺀다."""
    k = _key(text, face, weight, tracking)
    if k not in _CACHE:
        prefetch([(text, face, weight, tracking)])
    return _CACHE[k] - tracking * 100


# 한글 잉크 상·하단 (baseline 기준, em 비율). 글꼴마다 한 번 그려 픽셀로 잰다.
_INK = {}


def ink(face, weight, sample="한국어 맛집 불판"):
    key = "%s|%d|%s" % (face, weight, sample)
    if key in _INK:
        return _INK[key]
    cache_key = "ink|" + key
    if cache_key in _CACHE:
        _INK[key] = tuple(_CACHE[cache_key])
        return _INK[key]
    svg = ('<svg xmlns="http://www.w3.org/2000/svg" width="1400" height="400"><rect width="1400" height="400" fill="#000"/>'
           '<text x="20" y="300" font-family="%s" font-size="200" font-weight="%d" fill="#fff">%s</text></svg>'
           % (FACE[face], weight, esc(sample)))
    png = os.path.join(ROOT, "src", ".ink.png")
    subprocess.run(["resvg", "--skip-system-fonts"] + FONT_ARGS + ["-", png], input=svg.encode(), check=True)
    raw = subprocess.run(["ffmpeg", "-v", "error", "-i", png, "-f", "rawvideo", "-pix_fmt", "gray", "-"],
                         capture_output=True, check=True).stdout
    rows = [i for i in range(400) if max(raw[i * 1400:(i + 1) * 1400]) > 128]
    top, bottom = (300 - rows[0]) / 200.0, (rows[-1] - 300) / 200.0
    _INK[key] = (top, bottom)
    _CACHE[cache_key] = [top, bottom]
    with open(_CACHE_FILE, "w") as fh:
        json.dump(_CACHE, fh, ensure_ascii=False)
    os.remove(png)
    return _INK[key]


# ------------------------------------------------------------------ 폭 맞춤
class Spec:
    """한 슬롯의 타입. role 토큰에서 시작해 필요한 것만 덮어쓴다."""

    def __init__(self, role="headline", **kw):
        face, size, weight, tracking, floor = ROLE[role]
        self.role = role
        self.face = kw.get("face", face)
        self.size = kw.get("size", size)
        self.weight = kw.get("weight", weight)
        self.tracking = kw.get("tracking", tracking)
        self.floor = kw.get("floor", floor if "size" not in kw else round(kw["size"] * 0.62))
        self.lines = kw.get("lines", 2 if role in ("display", "headline", "hook", "title") else 1)
        self.fill = kw.get("fill", MUTED if role == "label" else WHITE)
        self.stroke = kw.get("stroke", 0)
        self.shadow = kw.get("shadow", True)
        self.line_height = kw.get("line_height", 1.12)
        self.max_width = kw.get("max_width", MEASURE)
        self.outline = kw.get("outline")      # (width, alpha): 채우지 않고 흰 외곽선만
        self.sample = kw.get("sample", "한국어 맛집 불판")

    def width(self, text, size=None, tracking=None):
        t = self.tracking if tracking is None else tracking
        return adv(text, self.face, self.weight, t) * (size or self.size) / 100.0


def balanced_split(text, spec, n):
    """어절 경계에서 n줄로 나눠 가장 긴 줄이 가장 짧아지는 조합."""
    words = text.split(" ")
    if n == 1 or len(words) < n:
        return [text]
    best, best_w = None, 1e9
    if n == 2:
        for i in range(1, len(words)):
            lines = [" ".join(words[:i]), " ".join(words[i:])]
            w = max(spec.width(l) for l in lines)
            if w < best_w:
                best, best_w = lines, w
    else:
        for i in range(1, len(words) - 1):
            for j in range(i + 1, len(words)):
                lines = [" ".join(words[:i]), " ".join(words[i:j]), " ".join(words[j:])]
                w = max(spec.width(l) for l in lines)
                if w < best_w:
                    best, best_w = lines, w
    return best


def fit(text, spec, mode="floor_wrap"):
    """제안 규칙. 반환: dict(lines, size, tracking, over)

    over=True 면 하한 크기로도 폭을 못 맞춘 경우 — 지금처럼 거절(작성 문구) 또는 짧게 고침(생성 문구).
    """
    mw = spec.max_width
    tr = spec.tracking
    base = spec.size
    one = spec.width(text)
    if mode == "fixed":
        return dict(lines=[text], size=base, tracking=tr, over=one > mw)
    if mode == "shrink":            # 하한 없이 한 줄
        return dict(lines=[text], size=min(base, base * mw / one), tracking=tr, over=False)
    if mode == "steps":             # 타입 스케일 단계로만
        steps = [s for s in (132, 96, 84, 72, 64, 56, 48, 44, 40, 36) if s <= base]
        for s in steps:
            if spec.width(text, s) <= mw:
                return dict(lines=[text], size=s, tracking=tr, over=False)
        return dict(lines=[text], size=steps[-1], tracking=tr, over=True)
    if mode == "tracking_first":    # 자간을 먼저 -0.06em 까지 조이고 그다음 축소
        tight = min(tr, -0.06)
        if one <= mw:
            return dict(lines=[text], size=base, tracking=tr, over=False)
        w_t = adv(text, spec.face, spec.weight, tight) * base / 100.0
        if w_t <= mw:
            # 필요한 만큼만 조인다 (선형 보간)
            k = (one - mw) / (one - w_t)
            return dict(lines=[text], size=base, tracking=tr + (tight - tr) * k, over=False)
        s = base * mw / w_t
        if s >= spec.floor or spec.lines == 1:
            return dict(lines=[text], size=max(s, spec.floor), tracking=tight, over=s < spec.floor)
        lines = balanced_split(text, spec, 2)
        w2 = max(adv(l, spec.face, spec.weight, tight) for l in lines) * base / 100.0
        s2 = min(base, base * mw / w2)
        return dict(lines=lines, size=max(s2, spec.floor), tracking=tight, over=s2 < spec.floor)
    if mode == "wrap_first":        # 넘치면 먼저 두 줄, 그래도 넘치면 축소
        if one <= mw or spec.lines == 1 or " " not in text:
            s = min(base, base * mw / one)
            return dict(lines=[text], size=max(s, spec.floor), tracking=tr, over=s < spec.floor)
        lines = balanced_split(text, spec, 2)
        w2 = max(spec.width(l) for l in lines)
        s2 = min(base, base * mw / w2)
        return dict(lines=lines, size=max(s2, spec.floor), tracking=tr, over=s2 < spec.floor)
    # floor_wrap (제안): 한 줄에서 하한까지 줄이고, 그래도 넘치면 어절로 두 줄 나눠 다시 맞춤
    s = min(base, base * mw / one)
    if s >= spec.floor:
        return dict(lines=[text], size=s, tracking=tr, over=False)
    if spec.lines >= 2 and " " in text:
        lines = balanced_split(text, spec, 2)
        w2 = max(spec.width(l) for l in lines)
        s2 = min(base, base * mw / w2)
        if s2 >= spec.floor or spec.lines == 2 or text.count(" ") < 2:
            return dict(lines=lines, size=max(s2, spec.floor), tracking=tr, over=s2 < spec.floor)
        lines = balanced_split(text, spec, 3)
        w3 = max(spec.width(l) for l in lines)
        s3 = min(base, base * mw / w3)
        return dict(lines=lines, size=max(s3, spec.floor), tracking=tr, over=s3 < spec.floor)
    return dict(lines=[text], size=spec.floor, tracking=tr, over=True)


# ------------------------------------------------------------------ SVG 조각
def defs():
    return ('<defs><filter id="sh" x="-20%" y="-40%" width="140%" height="180%" color-interpolation-filters="sRGB">'
            '<feDropShadow dx="0" dy="4" stdDeviation="6" flood-color="#000" flood-opacity="0.55"/></filter>'
            '<linearGradient id="scrimB" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#000" stop-opacity="0"/>'
            '<stop offset="0.47" stop-color="#000" stop-opacity="0.55"/><stop offset="1" stop-color="#000" stop-opacity="0.62"/></linearGradient>'
            '<linearGradient id="scrimT" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#000" stop-opacity="0.5"/>'
            '<stop offset="1" stop-color="#000" stop-opacity="0"/></linearGradient>'
            '<radialGradient id="scrimR" cx="0.5" cy="0.5" r="0.5"><stop offset="0" stop-color="#000" stop-opacity="0.5"/>'
            '<stop offset="0.55" stop-color="#000" stop-opacity="0.3"/><stop offset="1" stop-color="#000" stop-opacity="0"/></radialGradient></defs>')


def text_el(value, x, y, size, spec, tracking=None, anchor="middle", fill=None, extra=""):
    """x 는 anchor=middle 이면 잉크 중심, start 면 잉크 왼쪽."""
    tr = spec.tracking if tracking is None else tracking
    w = adv(value, spec.face, spec.weight, tr) * size / 100.0
    x0 = x - w / 2 if anchor == "middle" else (x - w if anchor == "end" else x)
    hexv, alpha = fill or spec.fill
    stroke = ""
    if spec.outline:
        hexv, alpha = "none", 1
        stroke = (' stroke="#FFFFFF" stroke-opacity="%s" stroke-width="%s" stroke-linejoin="round"'
                  % (num(spec.outline[1]), num(spec.outline[0])))
    elif spec.stroke:
        stroke = (' stroke="%s" stroke-opacity="%s" stroke-width="%s" stroke-linejoin="round" paint-order="stroke fill"'
                  % (STROKE[0], STROKE[1], num(spec.stroke)))
    filt = ' filter="url(#sh)"' if spec.shadow else ""
    return ('<text x="%s" y="%s" xml:space="preserve" font-family="%s" font-size="%s" font-weight="%d" '
            'letter-spacing="%s" fill="%s" fill-opacity="%s"%s%s%s>%s</text>'
            % (num(x0), num(y), FACE[spec.face], num(size), spec.weight, num(tr * size), hexv, num(alpha if hexv != "none" else 1),
               stroke, filt, extra, esc(value))), w


def rect(x, y, w, h, fill="#FFFFFF", alpha=1.0, r=0, extra=""):
    return ('<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="%s" fill-opacity="%s"%s/>'
            % (num(x), num(y), num(w), num(h), num(r), fill, num(alpha), extra))


def hair(cx, y, w=HAIR["w"], alpha=HAIR["a"], h=HAIR["h"], x=None):
    x0 = cx - w / 2 if x is None else x
    return rect(x0, y - h / 2, w, h, "#FFFFFF", alpha)


def bar(cx, y, w=BAR["w"], h=BAR["h"], x=None):
    x0 = cx - w / 2 if x is None else x
    return rect(x0, y - h / 2, w, h, "#FFFFFF", BAR["a"])


# ------------------------------------------------------------------ 스택 레이아웃 (간격 유지)
FIT_LOG = []
class Stack:
    """위에서 아래로 쌓는 블록. 간격은 잉크 사이 거리로 고정하고, 글자가 줄거나 두 줄이 되면
    블록 높이만 바뀐 채 중심(cy)을 지킨다."""

    def __init__(self, x=CX, anchor="middle"):
        self.items = []
        self.x, self.anchor = x, anchor

    def text(self, value, spec, mode="floor_wrap", gap=0, **kw):
        if not value:
            return None
        f = fit(value, spec, mode)
        top, bottom = ink(spec.face, spec.weight, spec.sample)
        lh = f["size"] * spec.line_height
        n = len(f["lines"])
        h = top * f["size"] + (n - 1) * lh + bottom * f["size"]
        frame = kw.pop("frame", None)
        pv = frame["pad_v"] if frame else 0
        self.items.append(dict(kind="text", spec=spec, fit=f, gap=gap, h=h + 2 * pv, top=top, lh=lh, kw=kw, frame=frame))
        return f

    def gap(self, px):
        self.items.append(dict(kind="gap", h=px, gap=0))

    def rule(self, kind="hair", gap=0, w=None, alpha=None):
        t = HAIR if kind == "hair" else BAR
        self.items.append(dict(kind="rule", rule=kind, gap=gap, h=t["h"], w=w or t["w"], alpha=alpha))

    def custom(self, h, draw, gap=0, width=0):
        """draw(x, top) -> svg"""
        self.items.append(dict(kind="custom", h=h, draw=draw, gap=gap, width=width))

    def height(self):
        return sum(it["gap"] + it["h"] for it in self.items)

    def render(self, cy=None, top=None, bottom=None):
        total = self.height()
        if top is None:
            top = cy - total / 2 if bottom is None else bottom - total
        y = top
        out, widths, info = [], [], []
        for it in self.items:
            y += it["gap"]
            if it["kind"] == "text":
                f, spec, fr = it["fit"], it["spec"], it["frame"]
                pv = fr["pad_v"] if fr else 0
                base = y + pv + it["top"] * f["size"]
                lines = []
                for i, line in enumerate(f["lines"]):
                    svg, w = text_el(line, self.x, base + i * it["lh"], f["size"], spec, f["tracking"], self.anchor, **it["kw"])
                    lines.append(svg)
                    widths.append(w)
                if fr:
                    lw = max(spec.width(l, f["size"], f["tracking"]) for l in f["lines"])
                    bw = max(lw + 2 * fr["pad_h"], fr.get("min_w", 0))
                    bx = self.x - bw / 2 if self.anchor == "middle" else self.x - fr["pad_h"]
                    out.append(framed(fr, bx, y, bw, it["h"]))
                    widths.append(bw)
                out.extend(lines)
                info.append(f)
            elif it["kind"] == "rule":
                t = HAIR if it["rule"] == "hair" else BAR
                a = it["alpha"] if it["alpha"] is not None else t["a"]
                if self.anchor == "middle":
                    out.append(rect(self.x - it["w"] / 2, y, it["w"], it["h"], "#FFFFFF", a))
                else:
                    out.append(rect(self.x, y, it["w"], it["h"], "#FFFFFF", a))
            elif it["kind"] == "custom":
                out.append(it["draw"](self.x, y))
                widths.append(it["width"])
            y += it["h"]
        self.box = (top, y, max(widths) if widths else 0)
        self.info = info
        FIT_LOG.extend(info)
        return "".join(out)


def framed(fr, x, y, w, h):
    kind = fr["kind"]
    if kind == "plate":
        fill, alpha = fr.get("fill", PLATE)
        return rect(x, y, w, h, fill, alpha, fr.get("radius", 12))
    if kind in ("outline", "pill"):
        r = h / 2 if kind == "pill" else fr.get("radius", 0)
        return ('<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="none" stroke="#FFFFFF" stroke-opacity="%s" '
                'stroke-width="%s" filter="url(#sh)"/>' % (num(x), num(y), num(w), num(h), num(r),
                                                           num(fr.get("alpha", 0.9)), num(fr.get("stroke", 3))))
    if kind == "brackets":
        return brackets(x, y, w, h, fr.get("arm", 44), fr.get("stroke", 4))
    return ""


def brackets(x, y, w, h, arm=44, sw=4):
    d = []
    for (cx, cy, dx, dy) in ((x, y, 1, 1), (x + w, y, -1, 1), (x, y + h, 1, -1), (x + w, y + h, -1, -1)):
        d.append("M%s %s L%s %s L%s %s" % (num(cx + dx * arm), num(cy), num(cx), num(cy), num(cx), num(cy + dy * arm)))
    return ('<path d="%s" fill="none" stroke="#FFFFFF" stroke-width="%s" stroke-linecap="square" filter="url(#sh)"/>'
            % (" ".join(d), num(sw)))


# ------------------------------------------------------------------ 굽기
def frame(body, bg, name, scrim=None):
    bg_href = os.path.relpath(os.path.join(ROOT, "bg", bg), os.path.join(OUT, "svg"))
    s = ['<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="%d" height="%d">' % (W, H),
         defs(),
         '<image href="%s" x="0" y="0" width="%d" height="%d" preserveAspectRatio="xMidYMid slice"/>' % (bg_href, W, H)]
    if scrim in ("bottom", "both"):
        s.append(rect(0, 980, W, 940, "url(#scrimB)", 1))
    if scrim == "both":
        s.append(rect(0, 0, W, 560, "url(#scrimT)", 1))
    if scrim in ("bottom", "both"):
        pass
    elif scrim == "top":
        s.append(rect(0, 0, W, 560, "url(#scrimT)", 1))
    elif isinstance(scrim, tuple):
        _, cy, ry = scrim
        s.append('<ellipse cx="540" cy="%s" rx="680" ry="%s" fill="url(#scrimR)"/>' % (num(cy), num(ry)))
    elif scrim:
        s.append(scrim)
    s.append(body)
    s.append("</svg>")
    os.makedirs(os.path.join(OUT, "svg"), exist_ok=True)
    path = os.path.join(OUT, "svg", name + ".svg")
    with open(path, "w") as fh:
        fh.write("".join(s))
    return path


def bake(svg_path, png_path, width=None):
    args = ["resvg", "--skip-system-fonts"] + FONT_ARGS + ["--resources-dir", os.path.dirname(svg_path)]
    if width:
        args += ["-w", str(width)]
    subprocess.run(args + [svg_path, png_path], check=True)


def to_jpg(png, jpg, q=3):
    subprocess.run(["ffmpeg", "-v", "error", "-y", "-i", png, "-q:v", str(q), jpg], check=True)


# ------------------------------------------------------------------ 비교 시트
def sheet(path, title, subtitle, col_heads, rows, cell_w, view=None, head_w=380, note_h=64, gap=18):
    """rows = [dict(title, desc, cells=[dict(png, note, bad)])]. view=(y0, h) 면 프레임의 그 띠만 보여 준다."""
    y0, vh = view if view else (0, H)
    scale = cell_w / float(W)
    cell_h = vh * scale
    pad = 56
    top = 200
    col_head_h = 70 if col_heads else 0
    ncol = max([len(col_heads or [])] + [len(r["cells"]) for r in rows])
    width = pad * 2 + head_w + ncol * (cell_w + gap) - gap
    height = top + col_head_h + len(rows) * (cell_h + note_h + gap) + pad
    sub = SubSpec()
    s = ['<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">' % (width, height),
         rect(0, 0, width, height, "#F3F2EF", 1),
         sub.t(title, pad, 96, 52, 800, "#141414", "paperlogy"),
         sub.t(subtitle, pad, 150, 26, 500, "#555555")]
    x0 = pad + head_w
    for c, head in enumerate(col_heads or []):
        s.append(sub.t(head, x0 + c * (cell_w + gap), top + 34, 24, 700, "#222222"))
    y = top + col_head_h
    for r in rows:
        s.append(sub.t(r["title"], pad, y + 40, 30, 800, "#141414"))
        for i, line in enumerate(r.get("desc", [])):
            s.append(sub.t(line, pad, y + 84 + i * 32, 22, 500, "#555555"))
        for c, cell in enumerate(r["cells"]):
            x = x0 + c * (cell_w + gap)
            href = os.path.relpath(cell["png"], os.path.dirname(path))
            s.append('<svg x="%s" y="%s" width="%s" height="%s" viewBox="0 %d %d %d" preserveAspectRatio="none">'
                     '<image href="%s" x="0" y="0" width="%d" height="%d"/></svg>'
                     % (num(x), num(y), num(cell_w), num(cell_h), y0, W, vh, href, W, H))
            if cell.get("bad"):
                s.append('<rect x="%s" y="%s" width="%s" height="%s" fill="none" stroke="#E5484D" stroke-width="5"/>'
                         % (num(x + 2.5), num(y + 2.5), num(cell_w - 5), num(cell_h - 5)))
            colour = "#D03A3F" if cell.get("bad") else "#333333"
            for i, line in enumerate(cell.get("note", "").split("\n")):
                s.append(sub.t(line, x, y + cell_h + 30 + i * 28, 22, 600 if i == 0 else 500, colour if i == 0 else "#666666"))
        y += cell_h + note_h + gap
    s.append("</svg>")
    with open(path, "w") as fh:
        fh.write("".join(s))
    png = path[:-4] + ".png"
    bake(path, png)
    return png


class SubSpec:
    """시트 주석용 글자 (측정 없이 그리기만)."""

    def t(self, value, x, y, size, weight, fill, face="wantedsans"):
        return ('<text x="%s" y="%s" font-family="%s" font-size="%s" font-weight="%d" fill="%s">%s</text>'
                % (num(x), num(y), FACE[face], num(size), weight, fill, esc(value)))


def guides(y0=0, y1=H):
    """856px 글줄 폭 표시 (진단용)."""
    return ''.join('<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#3FD0FF" stroke-opacity="0.85" stroke-width="3" '
                   'stroke-dasharray="14 10"/>' % (x, y0, x, y1) for x in (CX - MEASURE / 2, CX + MEASURE / 2))
