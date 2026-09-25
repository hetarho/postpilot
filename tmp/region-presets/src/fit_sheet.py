# -*- coding: utf-8 -*-
"""글자 수 고정 대신 폭 맞춤 — 방식별 비교 시트 두 장."""
import os
import lib
from lib import Spec, fit, text_el, hair, bar, CX, OUT

CELLS = os.path.join(OUT, "cells")
os.makedirs(CELLS, exist_ok=True)


def chars(t):
    return len([c for c in t if not c.isspace()])


# ---------------------------------------------------------------- 1. 크기를 정하는 방식 (인트로 A 헤드라인)
HEADS = ["해미 한우", "연남동 숯불 한우 오마카세", "연남동 골목 30년 숯불 한우 구이", "연남동 골목에서 30년째 숯불 한우만 굽는 집"]
LABEL = "서울 연남동 · 숯불 한우 구이"

MODES = [
    ("fixed", "현재 규칙", ["96px 고정 · 8자까지만", "넘으면 저장 자체가 거절된다"]),
    ("shrink", "① 한 줄 연속 축소", ["폭에 닿는 만큼 계속 줄인다", "하한 없음 — 긴 문구는 작아진다"]),
    ("steps", "② 단계 축소", ["96·84·72·64·56·48 중", "들어가는 가장 큰 단계로"]),
    ("tracking_first", "③ 자간 먼저 조이기", ["자간을 −0.06em까지 좁힌 뒤", "축소 → 하한 60px에서 두 줄"]),
    ("wrap_first", "④ 두 줄 먼저", ["넘치면 크기 유지한 채 두 줄", "그래도 넘치면 축소(하한 60)"]),
    ("floor_wrap", "⑤ 축소 → 하한 → 두 줄 (추천)", ["한 줄에서 60px까지 줄이고", "그 아래로 가야 하면 어절로 두 줄"]),
]


def intro_a(head, mode, name):
    hs = Spec("headline", stroke=lib.STROKE_TEXT)
    ls = Spec("label", stroke=lib.STROKE_SMALL)
    f = fit(head, hs, mode)
    body = []
    # 인트로 A는 기준선 고정: 마지막 줄 기준선 940, 두 줄이면 윗줄이 위로 자란다
    lh = f["size"] * hs.line_height
    for i, line in enumerate(f["lines"]):
        y = 940 - (len(f["lines"]) - 1 - i) * lh
        body.append(text_el(line, CX, y, f["size"], hs, f["tracking"])[0])
    lf = fit(LABEL, ls, "floor_wrap")
    body.append(text_el(LABEL, CX, 1020, lf["size"], ls)[0])
    body.append(lib.guides(560, 1300))
    svg = lib.frame("".join(body), "plate.jpg", name)
    png = os.path.join(CELLS, name + ".png")
    lib.bake(svg, png)
    return f, png


def note_for(f, head, mode):
    n = chars(head)
    if mode == "fixed":
        if n > 8:
            return "저장 불가 (%d자 > 8자)" % n, True
        return "96px · 1줄 · %d자" % n, False
    sz = "%dpx" % round(f["size"])
    tr = ""
    if mode == "tracking_first" and abs(f["tracking"] + 0.02) > 1e-3:
        tr = " · 자간 %.3fem" % f["tracking"]
    bad = f["over"] or f["size"] < 52
    msg = "%s · %d줄%s" % (sz, len(f["lines"]), tr)
    if f["size"] < 60 and mode == "shrink":
        msg += "  (위계 무너짐)"
        bad = True
    return msg, bad


import json
DATA = {}
rows = []
for mode, title, desc in MODES:
    cells = []
    for i, head in enumerate(HEADS):
        f, png = intro_a(head, mode, "fit-%s-%d" % (mode, i))
        note, bad = note_for(f, head, mode)
        cells.append(dict(png=png, note=note, bad=bad))
    rows.append(dict(title=title, desc=desc, cells=cells))

heads = ["%d자 · %s" % (chars(h), h if len(h) < 16 else h[:15] + "…") for h in HEADS]
DATA["size"] = dict(heads=["%d자" % chars(h) for h in HEADS], texts=HEADS, rows=[dict(title=r["title"], desc=r["desc"], cells=[dict(name=os.path.basename(c["png"])[:-4], note=c["note"], bad=c["bad"]) for c in r["cells"]]) for r in rows])
p1 = lib.sheet(os.path.join(OUT, "01-fit-size-rules.svg"),
               "제목 폭 맞춤 — 크기를 정하는 방식",
               "인트로 A(헤드라인 96px + 라벨)에 글자 수만 바꿔 넣은 것. 하늘색 점선이 856px 글줄 폭. 빨간 테두리는 저장 불가·가독성 문제.",
               heads, rows, 300)
print(p1)


# ---------------------------------------------------------------- 2. 줄어들 때 세로 배치 (아웃트로 E)
DISPLAYS = ["4.8", "해미 한우", "해미 한우 연남 본점", "연남동 골목 30년 해미 한우"]


def outro_e(display, layout, name):
    label_s = Spec("label", stroke=lib.STROKE_SMALL, tracking=0.08)
    disp_s = Spec("display", stroke=lib.STROKE_TEXT)
    cap_s = Spec("caption", stroke=lib.STROKE_SMALL, fill=("#FFFFFF", 0.8))
    label, caption = "직접 먹어본 평점", "다시 가고 싶은 불판"
    body = []
    if layout == "fixed":
        f = fit(display, disp_s, "fixed")
    else:
        f = fit(display, disp_s, "floor_wrap")
    if layout in ("fixed", "baseline"):
        body.append(text_el(label, CX, 836, 36, label_s)[0])
        lh = f["size"] * disp_s.line_height
        for i, line in enumerate(f["lines"]):
            y = 980 - (len(f["lines"]) - 1 - i) * lh
            body.append(text_el(line, CX, y, f["size"], disp_s, f["tracking"])[0])
        body.append(bar(CX, 1030))
        body.append(text_el(caption, CX, 1110, 44, cap_s)[0])
    else:
        # 간격 유지: 잉크 사이 간격을 9:16 원본 값(31·42·42)으로 고정하고 블록 중심 960 유지
        st = lib.Stack()
        st.text(label, label_s)
        st.text(display, disp_s, gap=31)
        st.rule("bar", gap=42)
        st.text(caption, cap_s, gap=42)
        body.append(st.render(cy=960.5))
    body.append(lib.guides(560, 1360))
    svg = lib.frame("".join(body), "grill.jpg", name)
    png = os.path.join(CELLS, name + ".png")
    lib.bake(svg, png)
    return f, png


LAYOUTS = [
    ("fixed", "현재 규칙", ["132px 고정 · 6자까지만", "넘으면 저장 거절"]),
    ("baseline", "A. 기준선 고정", ["⑤로 크기만 바꾸고 각 슬롯 기준선은", "그대로 — 두 줄이면 윗줄이 라벨을 덮는다"]),
    ("stack", "B. 간격 유지 (추천)", ["슬롯 사이 간격을 고정하고 블록 중심만", "지킨다 — 줄면 조이고, 두 줄이면 벌어진다"]),
]
rows = []
for layout, title, desc in LAYOUTS:
    cells = []
    for i, d in enumerate(DISPLAYS):
        f, png = outro_e(d, layout, "vert-%s-%d" % (layout, i))
        n = chars(d)
        if layout == "fixed":
            note, bad = ("저장 불가 (%d자 > 6자)" % n, True) if n > 6 else ("132px · 1줄 · %d자" % n, False)
        else:
            note = "%dpx · %d줄" % (round(f["size"]), len(f["lines"]))
            bad = layout == "baseline" and len(f["lines"]) > 1
            if bad:
                note += "  (라벨과 겹침)"
        cells.append(dict(png=png, note=note, bad=bad))
    rows.append(dict(title=title, desc=desc, cells=cells))

DATA["vert"] = dict(heads=["%d자" % chars(d) for d in DISPLAYS], texts=DISPLAYS, rows=[dict(title=r["title"], desc=r["desc"], cells=[dict(name=os.path.basename(c["png"])[:-4], note=c["note"], bad=c["bad"]) for c in r["cells"]]) for r in rows])
json.dump(DATA, open(os.path.join(OUT, "fit.json"), "w"), ensure_ascii=False, indent=1)
p2 = lib.sheet(os.path.join(OUT, "02-fit-vertical-rules.svg"),
               "제목 폭 맞춤 — 줄어들 때 세로 배치",
               "아웃트로 E(라벨 · 디스플레이 132px · 바 · 캡션)에서 가운데 줄만 길어질 때. 크기는 ⑤ 규칙(하한 80px).",
               ["%d자 · %s" % (chars(d), d) for d in DISPLAYS], rows, 300)
print(p2)
