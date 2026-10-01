#!/usr/bin/env python3
"""Apply a reviewed, compare-and-swap update to existing SQLite post templates.

The before file is a JSON list of template rows read from this database. The after file
maps template names to {description, body, required_labels}. Run without --apply to
inspect eligibility; --apply first takes a consistent SQLite backup, then updates only
changed rows in one transaction. An explicit --answer-plan file can carry the saved text of
questions this update removes into a question it adds on the same post; the original rows
are kept.
"""

import argparse
import datetime as dt
import hashlib
import json
import os
import pathlib
import re
import sqlite3


FIELDS = ("id", "name", "description", "title_area", "body", "target_length", "tag_count", "updated_at")
ASK = re.compile(r'<ask\s+label="([^"]+)"(?:\s+required="true")?(?:\s*/>|>)')
REQUIRED = re.compile(r'<ask\s+label="([^"]+)"\s+required="true"(?:\s*/>|>)')


def digest(value):
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def load_inputs(before_path, after_path):
    before_list = json.loads(before_path.read_text(encoding="utf-8"))
    after = json.loads(after_path.read_text(encoding="utf-8"))
    if not isinstance(before_list, list) or len(before_list) != 6:
        raise ValueError("the before snapshot must contain exactly six rows")
    before = {row["name"]: row for row in before_list}
    if len(before) != 6 or set(before) != set(after):
        raise ValueError("the six template names differ between snapshots")
    for name, candidate in after.items():
        body = candidate["body"]
        description = candidate["description"]
        labels = ASK.findall(body)
        required = REQUIRED.findall(body)
        if not body.rstrip().endswith("</ask>"):
            raise ValueError(f"{name}: body must close with an author answer")
        if len(body) > 4000 or len(description) > 200 or len(labels) > 10:
            raise ValueError(f"{name}: a template length or question count exceeds its limit")
        if len(labels) != len(set(labels)) or set(required) != set(candidate["required_labels"]):
            raise ValueError(f"{name}: duplicate labels or required attribute mismatch")
        if not required or "평점 / 한줄평" not in body or body.rfind("평점 / 한줄평") < len(body) // 2:
            raise ValueError(f"{name}: required input or final rating is missing")
        if not set(required).issubset(labels):
            raise ValueError(f"{name}: an unknown label is required")
        if any(len(label) > 40 for label in labels):
            raise ValueError(f"{name}: a question label exceeds its limit")
    return before, after


def load_answer_plan(path, before, after, value_limit):
    """Check an answer plan's shape against the two snapshots, before any database read."""
    if path is None:
        return []
    plan = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(plan, list) or not plan:
        raise ValueError("an answer plan must be a non-empty list")
    seen = set()
    for entry in plan:
        name, label, answer, sources = entry["template"], entry["label"], entry["answer"], entry["sources"]
        if name not in before:
            raise ValueError(f"answer plan names an unknown template: {name}")
        old_labels = set(ASK.findall(before[name]["body"]))
        new_labels = set(ASK.findall(after[name]["body"]))
        if label in old_labels or label not in new_labels:
            raise ValueError(f"{name}: an answer plan may fill only a question this update adds")
        if not sources or any(source in new_labels or source not in old_labels for source in sources):
            raise ValueError(f"{name}: answer plan sources must be questions this update removes")
        if not answer.strip() or len(answer) > value_limit:
            raise ValueError(f"{name}: a planned answer is blank or exceeds {value_limit} characters")
        key = (entry["post_slug"], label)
        if key in seen:
            raise ValueError(f"the answer plan fills {key} twice")
        seen.add(key)
    return plan


def database_rows(connection):
    connection.row_factory = sqlite3.Row
    rows = [dict(row) for row in connection.execute(
        "SELECT id,name,description,title_area,body,target_length,tag_count,updated_at "
        "FROM templates ORDER BY name"
    )]
    if len(rows) != len({row["name"] for row in rows}):
        raise RuntimeError("template names are not globally unique in this database")
    return {row["name"]: row for row in rows}


def check_current(connection, before):
    current = database_rows(connection)
    if set(current) != set(before):
        raise RuntimeError("the production template set changed; no rows were written")
    for name, expected in before.items():
        if any(current[name][field] != expected[field] for field in FIELDS):
            raise RuntimeError(f"{name}: the row changed after the snapshot; no rows were written")


def dependent_data_fingerprints(connection):
    queries = (
        "SELECT slug,template_id,content FROM posts ORDER BY slug",
        "SELECT post_slug,label,answer,enabled,updated_at FROM post_template_answers "
        "ORDER BY post_slug,label",
    )
    fingerprints = []
    for query in queries:
        hashed = hashlib.sha256()
        for row in connection.execute(query):
            hashed.update(json.dumps(tuple(row), ensure_ascii=False).encode("utf-8"))
            hashed.update(b"\n")
        fingerprints.append(hashed.hexdigest())
    return tuple(fingerprints)


def refuse_hidden_saved_answers(connection, before, after, plan):
    carried = {(entry["template"], source, entry["post_slug"]) for entry in plan for source in entry["sources"]}
    for name, old in before.items():
        removed = set(ASK.findall(old["body"])) - set(ASK.findall(after[name]["body"]))
        for label in sorted(removed):
            hidden = [slug for (slug,) in connection.execute(
                "SELECT a.post_slug FROM post_template_answers a "
                "JOIN posts p ON p.slug=a.post_slug "
                "WHERE p.template_id=? AND a.label=? AND trim(a.answer)<>''",
                (old["id"], label),
            ) if (name, label, slug) not in carried]
            if hidden:
                raise RuntimeError(
                    f"{name}: {label} has {len(hidden)} saved answers the plan does not carry; no rows were written"
                )


def check_answer_plan(connection, before, plan):
    """Re-read every row the plan depends on and return their digest for compare-and-swap."""
    if not plan:
        return None
    snapshot = []
    for entry in plan:
        slug, label = entry["post_slug"], entry["label"]
        owner = connection.execute("SELECT template_id FROM posts WHERE slug=?", (slug,)).fetchone()
        if owner is None or owner[0] != before[entry["template"]]["id"]:
            raise RuntimeError(f"{slug}: the post does not use {entry['template']}; no rows were written")
        if connection.execute(
            "SELECT 1 FROM post_template_answers WHERE post_slug=? AND label=?", (slug, label)
        ).fetchone() is not None:
            raise RuntimeError(f"{slug}: {label} already has a saved row; no rows were written")
        for source in sorted(entry["sources"]):
            row = connection.execute(
                "SELECT answer,enabled,updated_at FROM post_template_answers WHERE post_slug=? AND label=?",
                (slug, source),
            ).fetchone()
            if row is None or row[0] != entry["sources"][source] or row[1] != 1:
                raise RuntimeError(f"{slug}: {source} is not the planned enabled answer; no rows were written")
            snapshot.append([slug, source, row[0], row[1], row[2]])
    return digest(json.dumps(snapshot, ensure_ascii=False, sort_keys=True, separators=(",", ":")))


def answer_rows(connection):
    return [tuple(row) for row in connection.execute(
        "SELECT post_slug,label,answer,enabled,updated_at "
        "FROM post_template_answers ORDER BY post_slug,label"
    )]


def backup_live(connection, backup_path):
    backup_path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    if backup_path.exists():
        raise FileExistsError(f"backup already exists: {backup_path}")
    old_umask = os.umask(0o077)
    try:
        backup = sqlite3.connect(str(backup_path))
    finally:
        os.umask(old_umask)
    try:
        connection.backup(backup)
        if backup.execute("PRAGMA quick_check").fetchone()[0] != "ok":
            raise RuntimeError("the SQLite backup failed quick_check")
    finally:
        backup.close()
    backup_path.chmod(0o600)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--db", required=True, type=pathlib.Path)
    parser.add_argument("--before", required=True, type=pathlib.Path)
    parser.add_argument("--after", required=True, type=pathlib.Path)
    parser.add_argument("--backup", type=pathlib.Path)
    parser.add_argument("--apply", action="store_true")
    parser.add_argument("--expected-changes", type=int, metavar="N")
    parser.add_argument("--answer-plan", type=pathlib.Path, metavar="FILE")
    parser.add_argument("--expected-answer-inserts", type=int, metavar="N")
    parser.add_argument("--answer-plan-cas", metavar="SHA256")
    parser.add_argument("--answer-value-limit", type=int, default=500, metavar="N")
    args = parser.parse_args()
    if not 1 <= args.answer_value_limit <= 4000:
        parser.error("--answer-value-limit must be between 1 and 4000")
    before, after = load_inputs(args.before, args.after)
    plan = load_answer_plan(args.answer_plan, before, after, args.answer_value_limit)
    if not plan and (args.expected_answer_inserts is not None or args.answer_plan_cas is not None):
        parser.error("answer insert count and CAS require --answer-plan")
    if plan and args.expected_answer_inserts != len(plan):
        parser.error(f"--answer-plan holds {len(plan)} inserts; confirm them with --expected-answer-inserts")
    if args.answer_plan_cas is not None and not re.fullmatch(r"[0-9a-f]{64}", args.answer_plan_cas):
        parser.error("--answer-plan-cas must be a lowercase SHA-256 digest")
    if args.apply and plan and args.answer_plan_cas is None:
        parser.error("--apply with --answer-plan requires a dry-run --answer-plan-cas")
    changed_names = sorted(
        name for name in after
        if any(before[name][field] != after[name][field] for field in ("description", "body"))
    )
    if args.expected_changes is not None and args.expected_changes != len(changed_names):
        parser.error(
            f"expected {args.expected_changes} changed template rows, "
            f"found {len(changed_names)}"
        )
    if args.apply and args.backup is None:
        parser.error("--apply requires --backup")

    connection = sqlite3.connect(str(args.db), timeout=15)
    try:
        connection.execute("PRAGMA busy_timeout=15000")
        check_current(connection, before)
        plan_cas = check_answer_plan(connection, before, plan)
        if args.answer_plan_cas is not None and plan_cas != args.answer_plan_cas:
            raise RuntimeError("saved answers changed since the answer-plan dry run; no rows were written")
        refuse_hidden_saved_answers(connection, before, after, plan)
        # Exercise the same dependent-row scan on a read-only dry run so a schema or
        # serialization surprise cannot first appear after the production backup.
        dependent_data_fingerprints(connection)
        if connection.execute("PRAGMA quick_check").fetchone()[0] != "ok":
            raise RuntimeError("the source database failed quick_check; no rows were written")
        if args.apply:
            data_version = connection.execute("PRAGMA data_version").fetchone()[0]
            backup_live(connection, args.backup)
            connection.execute("BEGIN IMMEDIATE")
            try:
                if connection.execute("PRAGMA data_version").fetchone()[0] != data_version:
                    raise RuntimeError("the database changed after its backup; no rows were written")
                check_current(connection, before)
                if check_answer_plan(connection, before, plan) != args.answer_plan_cas:
                    raise RuntimeError("saved answers changed since the answer-plan dry run; no rows were written")
                refuse_hidden_saved_answers(connection, before, after, plan)
                post_count = connection.execute("SELECT COUNT(*) FROM posts").fetchone()[0]
                dependent_data = dependent_data_fingerprints(connection)
                previous_answers = answer_rows(connection)
                timestamp = dt.datetime.now(dt.timezone.utc).isoformat(timespec="microseconds").replace("+00:00", "Z")
                for name in changed_names:
                    row = before[name]
                    result = connection.execute(
                        "UPDATE templates SET description=?,body=?,updated_at=? WHERE id=?",
                        (after[name]["description"], after[name]["body"], timestamp, row["id"]),
                    )
                    if result.rowcount != 1:
                        raise RuntimeError(f"{name}: update did not match exactly one row")
                for entry in plan:
                    connection.execute(
                        "INSERT INTO post_template_answers (post_slug,label,answer,enabled,updated_at) "
                        "VALUES (?,?,?,1,?)",
                        (entry["post_slug"], entry["label"], entry["answer"], timestamp),
                    )
                if post_count != connection.execute("SELECT COUNT(*) FROM posts").fetchone()[0]:
                    raise RuntimeError("a post count changed")
                if dependent_data[0] != dependent_data_fingerprints(connection)[0]:
                    raise RuntimeError("post content changed")
                expected_answers = sorted(previous_answers + [
                    (entry["post_slug"], entry["label"], entry["answer"], 1, timestamp) for entry in plan
                ])
                if expected_answers != answer_rows(connection):
                    raise RuntimeError("a saved answer changed outside the exact answer plan")
                for name, row in database_rows(connection).items():
                    if row["body"] != after[name]["body"] or row["description"] != after[name]["description"]:
                        raise RuntimeError(f"{name}: read-back failed")
                    if any(row[field] != before[name][field] for field in ("id", "name", "title_area", "target_length", "tag_count")):
                        raise RuntimeError(f"{name}: an unrelated template field changed")
                    expected_updated_at = timestamp if name in changed_names else before[name]["updated_at"]
                    if row["updated_at"] != expected_updated_at:
                        raise RuntimeError(f"{name}: updated_at changed unexpectedly")
                connection.commit()
            except Exception:
                connection.rollback()
                raise
            if connection.execute("PRAGMA quick_check").fetchone()[0] != "ok":
                raise RuntimeError("the updated database failed quick_check; restore from backup")
        print(json.dumps({
            "applied": args.apply,
            "changed_rows": len(changed_names),
            "changed_names": changed_names,
            "answer_plan": (
                {"inserts": [[entry["post_slug"], entry["label"]] for entry in plan], "cas": plan_cas}
                if plan else None
            ),
            "templates": [
                {"name": name, "sha256": digest(after[name]["body"]),
                 "questions": len(ASK.findall(after[name]["body"])),
                 "required": len(after[name]["required_labels"])}
                for name in sorted(after)
            ],
            "backup": str(args.backup) if args.apply else None,
        }, ensure_ascii=False, indent=2))
    finally:
        connection.close()


if __name__ == "__main__":
    main()
