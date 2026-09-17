# -*- coding: utf-8 -*-
"""resvg --query-all 로 실제 셰이핑된 글자 폭을 잰다 (프로덕션과 같은 방식)."""
import subprocess, os, json, functools

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
RESVG = os.path.join(ROOT, "bin", "resvg")
FONTDIR = os.path.join(ROOT, "fonts")

def font_args():
    a = []
    for f in sorted(os.listdir(FONTDIR)):
        if f.endswith(".ttf"):
            a += ["--use-font-file", os.path.join(FONTDIR, f)]
    return a

_CACHE = {}

def measure(text, family, size, weight=400, spacing=0):
    key = (text, family, size, weight, spacing)
    if key in _CACHE: return _CACHE[key]
    esc = text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
    svg = (f'<svg xmlns="http://www.w3.org/2000/svg" width="4000" height="600">'
           f'<text id="m" x="100" y="400" font-family="&apos;{family}&apos;" font-size="{size}" '
           f'font-weight="{weight}" letter-spacing="{spacing}">{esc}</text></svg>')
    p = subprocess.run([RESVG, "--skip-system-fonts"] + font_args() + ["--query-all", "-"],
                       input=svg.encode(), capture_output=True)
    w = h = 0.0; x = y = 0.0
    for line in p.stdout.decode().splitlines():
        if line.startswith("m,"):
            _, xs, ys, ws, hs = line.split(",")
            x, y, w, h = float(xs), float(ys), float(ws), float(hs)
    res = dict(w=w, h=h, x=x, y=y, left=x - 100, top=y - 400)
    _CACHE[key] = res
    return res

def width(text, family, size, weight=400, spacing=0):
    return measure(text, family, size, weight, spacing)["w"]
