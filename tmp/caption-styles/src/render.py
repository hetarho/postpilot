# -*- coding: utf-8 -*-
import sys, os, subprocess, argparse, time
from multiprocessing import Pool
ROOT = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(ROOT, "lib"))
from common import frame, FONTS, WIDTH, HEIGHT
from measure import font_args, RESVG
from styles import CATALOG

BG = os.path.join(ROOT, "out", "bg")
OVER = 1.18

def bg_rect(i, t=0.5):
    import math
    ow, oh = int(WIDTH * OVER), int(HEIGHT * OVER)
    mx, my = ow - WIDTH, oh - HEIGHT
    bx = -mx * (0.5 + 0.42 * math.sin(t * 1.5 + i))
    by = -my * (0.5 + 0.30 * math.cos(t * 1.1 + i * 0.7))
    return (bx, by, ow, oh)

def build(i, cfg, t):
    defs, body = cfg["fn"](cfg["text"], cfg["kw"], t, i)
    return frame(defs, body, bg_png=f"{BG}/{cfg['bg']}.png", bg_rect=bg_rect(i, t))

def _one(job):
    i, t, out = job
    cfg = CATALOG[i]
    p = subprocess.run([RESVG, "--skip-system-fonts"] + font_args() + ["--resources-dir", ROOT, "-", out],
                       input=build(i, cfg, t).encode(), capture_output=True)
    return (cfg["id"], p.returncode, p.stderr.decode()[:220])

if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--t", type=float, default=0.55)
    ap.add_argument("--out", default=os.path.join(ROOT, "out", "stills"))
    ap.add_argument("--workers", type=int, default=12)
    a = ap.parse_args()
    os.makedirs(a.out, exist_ok=True)
    jobs = [(i, a.t, os.path.join(a.out, f"{i:02d}_{c['id']}.png")) for i, c in enumerate(CATALOG)]
    t0 = time.time()
    with Pool(a.workers) as pool:
        for cid, rc, err in pool.imap(_one, jobs):
            print(f"  {cid:16s} {'OK' if rc == 0 else 'FAIL ' + err}")
    print(f"{len(jobs)}컷 {time.time()-t0:.1f}s")
