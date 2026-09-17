# -*- coding: utf-8 -*-
"""스타일 22종을 한 편의 세로 영상으로. 프로덕션과 같은 resvg 0.48.1 + ffmpeg 9.0.1."""
import sys, os, subprocess, math, time, argparse
from multiprocessing import Pool
ROOT = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(ROOT, "lib"))
from common import WIDTH, HEIGHT, BG_DEFS, FONTS, frame
from measure import font_args, RESVG, measure
from styles import CATALOG

FPS, SEC = 30, 3.2
NF = int(FPS * SEC)
FFMPEG = os.path.join(ROOT, "bin", "ffmpeg")
FRAMES = os.path.join(ROOT, "out", "frames")
BG = os.path.join(ROOT, "out", "bg")
OVER = 1.18

def label_svg(idx, cfg):
    no = f"{idx+1:02d}"
    txt = f"{cfg['name']} · {FONTS[cfg['font']][2]}"
    fam = "Pretendard Variable"
    nw = measure(no, fam, 30, 800)["w"] + 14
    tw = measure(txt, fam, 28, 600)["w"]
    w = 26 + nw + tw + 26
    return (f'<g opacity="0.94"><rect x="52" y="92" width="{w:.0f}" height="62" rx="31" fill="#0A0C10" fill-opacity="0.74"/>'
            f'<text x="78" y="134" font-family="{fam}" font-weight="800" font-size="30" fill="#C8F751">{no}</text>'
            f'<text x="{78+nw:.0f}" y="134" font-family="{fam}" font-weight="600" font-size="28" fill="#E9EEF6">{txt}</text></g>')

def frame_svg(idx, cfg, i):
    t = i / (NF - 1)
    defs, body = cfg["fn"](cfg["text"], cfg["kw"], t, idx)
    ow, oh = int(WIDTH * OVER), int(HEIGHT * OVER)
    mx, my = ow - WIDTH, oh - HEIGHT
    bx = -mx * (0.5 + 0.42 * math.sin(t * 1.5 + idx))
    by = -my * (0.5 + 0.30 * math.cos(t * 1.1 + idx * 0.7))
    fin, fout = min(1.0, t / 0.07), min(1.0, (1 - t) / 0.07)
    dim = 1 - min(fin, fout)
    lab = label_svg(idx, cfg)
    if dim > 0.001:
        lab += f'<rect width="{WIDTH}" height="{HEIGHT}" fill="#000" opacity="{dim:.3f}"/>'
    return frame(defs, body, bg_png=f"{BG}/{cfg['bg']}.png", bg_rect=(bx, by, ow, oh), label=lab)

def _one(job):
    idx, i = job
    out = os.path.join(FRAMES, f"{idx*NF + i + 1:05d}.png")
    p = subprocess.run([RESVG, "--skip-system-fonts"] + font_args() + ["--resources-dir", ROOT, "-", out],
                       input=frame_svg(idx, CATALOG[idx], i).encode(), capture_output=True)
    return (idx, i, p.returncode, p.stderr.decode()[:200])

def render_frames(workers=14):
    os.makedirs(FRAMES, exist_ok=True)
    jobs = [(idx, i) for idx in range(len(CATALOG)) for i in range(NF)]
    t0 = time.time(); done = 0; fails = []
    with Pool(workers) as pool:
        for idx, i, rc, err in pool.imap_unordered(_one, jobs, chunksize=4):
            done += 1
            if rc != 0: fails.append((idx, i, err))
            if done % 200 == 0: print(f"  {done}/{len(jobs)}  {time.time()-t0:.0f}s", flush=True)
    print(f"프레임 {done}장, {time.time()-t0:.0f}s, 실패 {len(fails)}")
    for f in fails[:5]: print("   FAIL", f)
    return not fails

def encode(out_mp4):
    cmd = [FFMPEG, "-y", "-hide_banner", "-loglevel", "error",
           "-framerate", str(FPS), "-i", os.path.join(FRAMES, "%05d.png"),
           "-c:v", "libx264", "-preset", "veryfast", "-crf", "21",
           "-pix_fmt", "yuv420p", "-profile:v", "high", "-movflags", "+faststart",
           "-r", str(FPS), out_mp4]
    p = subprocess.run(cmd, capture_output=True)
    print("ffmpeg:", p.returncode, p.stderr.decode()[:400])
    return p.returncode == 0

if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--workers", type=int, default=14)
    ap.add_argument("--skip-frames", action="store_true")
    a = ap.parse_args()
    if not a.skip_frames: render_frames(a.workers)
    out = os.path.join(ROOT, "out", "caption-styles.mp4")
    if encode(out):
        print(f"완성: {out}  {os.path.getsize(out)/1e6:.1f} MB  {len(CATALOG)*SEC:.1f}초")
