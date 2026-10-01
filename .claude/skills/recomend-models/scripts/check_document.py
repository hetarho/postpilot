#!/usr/bin/env python3
"""Check a postpilot models document before the operator pastes it, and diff it against the export.

Stdlib only. Mirrors what 일괄 편집's preview refuses (MODEL-51~53, 59, 72~73) closely enough
that a document passing here should preview with no rejected line. The server stays the
judge: this exists so the skill never hands over a document that fails on paste.

  python3 check_document.py proposed.txt --baseline export.txt --catalog raw.json
  python3 check_document.py proposed.txt --baseline export.txt     # fetches the live catalog

`raw.json` is what `openrouter_models.py --json raw.json` saved. Exit code 1 when any line
would be rejected; the change list is printed either way.
"""
import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from openrouter_models import fetch  # noqa: E402

VERSION = "# postpilot models v1"
PURPOSES = ["photo-analysis", "style-analysis", "writing", "image-generation", "video-generation"]
LEVELS = ["free", "value", "balanced", "premium", "top"]
STAGES = {"observe": ("photo-analysis", 3), "analyze": ("style-analysis", 1), "write": ("writing", 3)}
MAX_SETS, MAX_LABEL = 10, 60


def looks_like_id(token):
    if not token or any(c in token for c in " \t|`\"',[]"):
        return False
    slug, _, rest = token.partition("/")
    return bool(slug and rest)


def parse(text):
    """Return ({purpose: {id: (level, line)}}, [(label, line, {stage: (ids, line)})] or None, issues)."""
    sections, sets, issues = {}, None, []
    current, in_recs, open_set = None, False, None
    versioned = False
    for number, raw in enumerate(text.replace("\r\n", "\n").split("\n"), 1):
        line = raw.strip()
        if not line:
            continue
        if not versioned:
            if line != VERSION:
                return {}, None, [(number, line, "bad_version")]
            versioned = True
            continue
        if line.startswith("#"):
            continue
        if line.startswith("["):
            if not line.endswith("]"):
                issues.append((number, line, "malformed_line"))
                continue
            name = line[1:-1].strip()
            if name == "recommendations":
                if sets is not None:
                    issues.append((number, line, "duplicate_section"))
                    continue
                sets, in_recs, current, open_set = [], True, None, None
                continue
            in_recs = False
            if name not in PURPOSES:
                issues.append((number, line, "unknown_purpose"))
            elif name in sections:
                issues.append((number, line, "duplicate_section"))
            else:
                sections[name], current = {}, name
            continue
        fields = line.split()
        if in_recs:
            if fields[0] == "set":
                label = line[len("set"):].strip()
                open_set = (label, number, {})
                if not label or len(label) > MAX_LABEL:
                    issues.append((number, line, "set_label_invalid"))
                elif any(s[0] == label for s in sets):
                    issues.append((number, line, "duplicate_set"))
                elif len(sets) >= MAX_SETS:
                    issues.append((number, line, "set_limit"))
                else:
                    sets.append(open_set)
                continue
            stage, ids = fields[0], fields[1:]
            if stage not in STAGES or len(ids) != STAGES[stage][1] or not all(map(looks_like_id, ids)):
                issues.append((number, line, "malformed_line"))
            elif open_set is None:
                issues.append((number, line, "stage_before_set"))
            elif stage in open_set[2]:
                issues.append((number, line, "duplicate_stage"))
            else:
                open_set[2][stage] = (ids, number)
            continue
        if len(fields) > 2 or not looks_like_id(fields[0]):
            issues.append((number, line, "malformed_line"))
        elif len(fields) == 2 and fields[1] not in LEVELS:
            issues.append((number, line, "unknown_level"))
        elif current is None:
            issues.append((number, line, "id_before_section"))
        elif fields[0] in sections[current]:
            issues.append((number, line, "duplicate_id"))
        else:
            sections[current][fields[0]] = (fields[1] if len(fields) == 2 else "", number)
    for label, number, stages in sets or []:
        for stage in STAGES:
            if stage not in stages:
                issues.append((number, stage, "missing_stage"))
    return sections, sets, issues


def check(sections, sets, catalog, baseline):
    issues = []
    for purpose, entries in sections.items():
        for model_id, (level, number) in entries.items():
            m = catalog.get(model_id)
            if m is None:
                issues.append((number, model_id, "unknown_model"))
                continue
            arch = m.get("architecture") or {}
            gate = {"photo-analysis": ("input_modalities", "image"),
                    "image-generation": ("output_modalities", "image"),
                    "video-generation": ("output_modalities", "video")}.get(purpose)
            if gate and gate[1] not in (arch.get(gate[0]) or []):
                issues.append((number, model_id, "purpose_ineligible"))
            p = m.get("pricing") or {}
            paid = any(float(p.get(k) or 0) for k in ("prompt", "completion"))
            if level == "free" and (paid or not model_id.endswith(":free")):
                issues.append((number, model_id, "free_path_ineligible"))
            if level == "":
                issues.append((number, model_id, "level_missing (등급이 지워진다, MODEL-59)"))
    # A set is checked against what each stage's purpose holds after the paste: this
    # document's section where it has one, the export's otherwise (MODEL-73).
    for label, _, stages in sets or []:
        for stage, (ids, number) in stages.items():
            purpose = STAGES[stage][0]
            holds = sections.get(purpose, baseline[0].get(purpose, {}))
            for index, model_id in enumerate(ids):
                if index == 2 and model_id == ids[1]:
                    issues.append((number, model_id, "slot_duplicate"))
                elif model_id not in holds:
                    issues.append((number, model_id, "slot_unregistered"))
                elif holds[model_id][0] == "":
                    issues.append((number, model_id, "slot_unclassified"))
    return issues


def diff(sections, sets, baseline):
    before, before_sets = baseline
    for purpose, entries in sections.items():
        old = before.get(purpose, {})
        added = [f"{i} {entries[i][0]}" for i in entries if i not in old]
        removed = [f"{i} {old[i][0]}" for i in old if i not in entries]
        relevel = [f"{i} {old[i][0] or '-'}→{entries[i][0] or '-'}"
                   for i in entries if i in old and old[i][0] != entries[i][0]]
        print(f"[{purpose}] {len(old)}→{len(entries)}")
        for title, items in (("추가", added), ("해제", removed), ("등급 변경", relevel)):
            for item in sorted(items):
                print(f"  {title}: {item}")
    for purpose in before:
        if purpose not in sections:
            print(f"[{purpose}] 섹션 없음 — 그대로 남는다")
    if sets is None:
        print("[recommendations] 섹션 없음 — 조합은 그대로 남는다")
        return
    shape = lambda s: {k: v[0] for k, v in s[2].items()}
    old = {s[0]: shape(s) for s in before_sets or []}
    new = {s[0]: shape(s) for s in sets}
    print(f"[recommendations] {len(old)}→{len(new)}")
    for label in new:
        if label not in old:
            print(f"  추가: {label}")
        elif new[label] != old[label]:
            print(f"  교체: {label}")
    for label in old:
        if label not in new:
            print(f"  삭제: {label}")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("proposed", help="the document the skill is about to hand over")
    ap.add_argument("--baseline", help="the export the operator pasted (MODEL-55)")
    ap.add_argument("--catalog", help="raw.json saved by openrouter_models.py --json; fetched live when omitted")
    args = ap.parse_args()

    if args.catalog:
        with open(args.catalog, encoding="utf-8") as f:
            data = json.load(f)["data"]
    else:
        try:
            data = fetch()
        except Exception as e:  # noqa: BLE001
            print(f"fetch failed: {e}", file=sys.stderr)
            sys.exit(2)
    catalog = {m["id"]: m for m in data if not m["id"].endswith(":batch")}

    with open(args.proposed, encoding="utf-8") as f:
        sections, sets, issues = parse(f.read())
    baseline = ({}, None)
    if args.baseline:
        with open(args.baseline, encoding="utf-8") as f:
            b_sections, b_sets, b_issues = parse(f.read())
        if b_issues:
            print(f"baseline itself does not parse: {b_issues[:3]}", file=sys.stderr)
            sys.exit(2)
        baseline = (b_sections, b_sets)
    issues += check(sections, sets, catalog, baseline)

    if args.baseline:
        diff(sections, sets, baseline)
    if issues:
        print(f"\n거절될 줄 {len(issues)}개:")
        for number, text, cause in sorted(issues):
            print(f"  line {number}: {cause}  {text}")
        sys.exit(1)
    print("\nOK — 거절될 줄 없음")


if __name__ == "__main__":
    main()
