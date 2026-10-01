#!/usr/bin/env python3
"""Apply a reviewed, compare-and-swap update to existing SQLite post templates.

The before file is a JSON list of template rows read from this database. The after file
maps template names to {description, body, required_labels}. Run without --apply to
inspect eligibility; --apply first takes a consistent SQLite backup, then updates all
rows in one transaction. This script never changes posts or their saved answers.
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
        if not body.lstrip().startswith("<ask ") or not body.rstrip().endswith("</ask>"):
            raise ValueError(f"{name}: body must open and close with an author answer")
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


def refuse_hidden_saved_answers(connection, before, after):
    for name, old in before.items():
        removed = set(ASK.findall(old["body"])) - set(ASK.findall(after[name]["body"]))
        for label in sorted(removed):
            filled = connection.execute(
                "SELECT COUNT(*) FROM post_template_answers a "
                "JOIN posts p ON p.slug=a.post_slug "
                "WHERE p.template_id=? AND a.label=? AND trim(a.answer)<>''",
                (old["id"], label),
            ).fetchone()[0]
            if filled:
                raise RuntimeError(f"{name}: {label} has {filled} saved answers; no rows were written")


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
    args = parser.parse_args()
    before, after = load_inputs(args.before, args.after)
    if args.apply and args.backup is None:
        parser.error("--apply requires --backup")

    connection = sqlite3.connect(str(args.db), timeout=15)
    try:
        connection.execute("PRAGMA busy_timeout=15000")
        check_current(connection, before)
        refuse_hidden_saved_answers(connection, before, after)
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
                refuse_hidden_saved_answers(connection, before, after)
                counts = tuple(
                    connection.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0]
                    for table in ("posts", "post_template_answers")
                )
                dependent_data = dependent_data_fingerprints(connection)
                timestamp = dt.datetime.now(dt.timezone.utc).isoformat(timespec="microseconds").replace("+00:00", "Z")
                for name in sorted(after):
                    row = before[name]
                    result = connection.execute(
                        "UPDATE templates SET description=?,body=?,updated_at=? WHERE id=?",
                        (after[name]["description"], after[name]["body"], timestamp, row["id"]),
                    )
                    if result.rowcount != 1:
                        raise RuntimeError(f"{name}: update did not match exactly one row")
                if counts != tuple(
                    connection.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0]
                    for table in ("posts", "post_template_answers")
                ):
                    raise RuntimeError("a post or saved answer count changed")
                if dependent_data != dependent_data_fingerprints(connection):
                    raise RuntimeError("post content or saved answers changed")
                for name, row in database_rows(connection).items():
                    if row["body"] != after[name]["body"] or row["description"] != after[name]["description"]:
                        raise RuntimeError(f"{name}: read-back failed")
                    if any(row[field] != before[name][field] for field in ("id", "name", "title_area", "target_length", "tag_count")):
                        raise RuntimeError(f"{name}: an unrelated template field changed")
                connection.commit()
            except Exception:
                connection.rollback()
                raise
            if connection.execute("PRAGMA quick_check").fetchone()[0] != "ok":
                raise RuntimeError("the updated database failed quick_check; restore from backup")
        print(json.dumps({
            "applied": args.apply,
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
