#!/usr/bin/env python3
"""Fetch the live OpenRouter catalog and print the candidates for postpilot's purposes.

Stdlib only. Mirrors what the app does (MODEL-17 / MODEL-50): the same endpoint and
modality query, `:batch` variants dropped, text-output models only.

  python3 openrouter_models.py                       # all purposes, last 180 days + every :free
  python3 openrouter_models.py --purpose writing     # one purpose
  python3 openrouter_models.py --days 365            # widen the recency window
  python3 openrouter_models.py --all-providers       # not just the featured ones
  python3 openrouter_models.py --json out.json       # also save the raw document

Flags column: V vision(image input) · M video input · R reasoning param · S structured_outputs.
Prices are USD per million tokens.
"""
import argparse
import datetime as dt
import json
import os
import subprocess
import sys
import urllib.request

URL = "https://openrouter.ai/api/v1/models?output_modalities=text,image,video"
FEATURED = ["openai", "anthropic", "google", "x-ai", "deepseek", "qwen", "z-ai",
            "moonshotai", "minimax", "meta", "mistralai", "nvidia", "thinkingmachines"]
PURPOSES = ["photo-analysis", "style-analysis", "writing"]


def fetch(timeout=30):
    req = urllib.request.Request(URL, headers={"User-Agent": "postpilot-recomend-models/1"})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        return json.load(r)["data"]


def price(p, key):
    try:
        return float(p.get(key) or 0) * 1e6
    except (TypeError, ValueError):
        return 0.0


def row(m):
    a = m.get("architecture") or {}
    sp = set(m.get("supported_parameters") or [])
    p = m.get("pricing") or {}
    return {
        "id": m["id"],
        "provider": m["id"].split("/")[0],
        "created": dt.date.fromtimestamp(m.get("created") or 0),
        "in": price(p, "prompt"),
        "out": price(p, "completion"),
        "free": m["id"].endswith(":free"),
        "vision": "image" in (a.get("input_modalities") or []),
        "video": "video" in (a.get("input_modalities") or []),
        "reasoning": bool(sp & {"reasoning", "include_reasoning"}),
        "structured": "structured_outputs" in sp,
        "ctx": m.get("context_length") or 0,
        "max_out": (m.get("top_provider") or {}).get("max_completion_tokens") or 0,
        "expires": m.get("expiration_date"),
        "desc": (m.get("description") or "").replace("\n", " ")[:90],
    }


def eligible(r, purpose):
    if purpose == "photo-analysis":
        return r["vision"]
    return True  # the text purposes take any model (MODEL-15)


def fmt(r):
    flags = "".join(c if ok else "-" for c, ok in (
        ("V", r["vision"]), ("M", r["video"]), ("R", r["reasoning"]), ("S", r["structured"])))
    exp = f" exp={r['expires']}" if r["expires"] else ""
    return (f"{r['created']} {r['id']:52s} in={r['in']:7.3f} out={r['out']:8.3f} {flags} "
            f"ctx={r['ctx']//1000:>5}k max_out={r['max_out']//1000:>4}k{exp}\n"
            f"{'':11}{r['desc']}")


def current_sets():
    """Print the shipped recommendation set from backend/config/providers.yaml, if we are in the repo."""
    try:
        root = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True,
                              text=True, check=True).stdout.strip()
    except Exception:
        return
    path = os.path.join(root, "backend", "config", "providers.yaml")
    if not os.path.exists(path):
        return
    lines = open(path, encoding="utf-8").read().splitlines()
    try:
        start = next(i for i, l in enumerate(lines) if l.strip().startswith("recommendation_sets:"))
    except StopIteration:
        return
    print("\n## 현재 코드에 실린 추천 세트 (backend/config/providers.yaml)")
    for l in lines[start:]:
        if l.strip() and not l.startswith((" ", "-", "\t")) and not l.strip().startswith("recommendation_sets"):
            break
        if "model_id" in l or "stage:" in l or "id:" in l or "label:" in l:
            print("  " + l.strip())


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--purpose", choices=PURPOSES + ["all"], default="all")
    ap.add_argument("--days", type=int, default=180, help="recency window for paid models (free ones always shown)")
    ap.add_argument("--all-providers", action="store_true", help="include every provider, not just the featured list")
    ap.add_argument("--json", metavar="PATH", help="also save the raw OpenRouter document here")
    args = ap.parse_args()

    try:
        data = fetch()
    except Exception as e:  # noqa: BLE001
        print(f"fetch failed: {e}", file=sys.stderr)
        sys.exit(1)
    if args.json:
        with open(args.json, "w", encoding="utf-8") as f:
            json.dump({"data": data}, f)

    rows = []
    for m in data:
        if m["id"].endswith(":batch"):
            continue
        if "text" not in ((m.get("architecture") or {}).get("output_modalities") or []):
            continue
        rows.append(row(m))

    since = dt.date.today() - dt.timedelta(days=args.days)
    today = dt.date.today().isoformat()
    print(f"# OpenRouter 텍스트 출력 모델 {len(rows)}개 (전체 {len(data)}개, :batch 제외) — {today}")
    print(f"# 유료는 {since} 이후 등록분, :free 는 전부. flags: V=vision M=video-in R=reasoning S=structured_outputs\n")

    purposes = PURPOSES if args.purpose == "all" else [args.purpose]
    for purpose in purposes:
        cand = [r for r in rows if eligible(r, purpose)]
        free = sorted([r for r in cand if r["free"]], key=lambda r: r["created"], reverse=True)
        paid = [r for r in cand if not r["free"] and r["created"] >= since]
        if not args.all_providers:
            paid = [r for r in paid if r["provider"] in FEATURED]
        order = {p: i for i, p in enumerate(FEATURED)}
        paid.sort(key=lambda r: (order.get(r["provider"], len(FEATURED)), r["provider"], -r["created"].toordinal()))

        print(f"\n==================== {purpose} ({len(free)} free · {len(paid)} paid) ====================")
        print("\n--- :free ---")
        for r in free:
            print(fmt(r))
        cur = None
        for r in paid:
            if r["provider"] != cur:
                cur = r["provider"]
                print(f"\n--- {cur} ---")
            print(fmt(r))

    current_sets()


if __name__ == "__main__":
    main()
