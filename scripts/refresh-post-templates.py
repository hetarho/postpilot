#!/usr/bin/env python3
"""Apply a reviewed, compare-and-swap update to existing SQLite post templates.

The before file is a JSON list of template rows read from this database. The after file
maps template names to {description, body, required_labels}. Run without --apply to
inspect eligibility; --apply first takes a consistent SQLite backup, then updates only
changed rows in one transaction. An explicit --merge-answer option can copy one removed
question's saved text into another answer on the same post; the original row is kept.
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
MERGE_CAPTION = "직접 먹어본 느낌: "


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


def refuse_hidden_saved_answers(connection, before, after, allowed_removed=None):
    for name, old in before.items():
        removed = set(ASK.findall(old["body"])) - set(ASK.findall(after[name]["body"]))
        for label in sorted(removed):
            if (name, label) == allowed_removed:
                continue
            filled = connection.execute(
                "SELECT COUNT(*) FROM post_template_answers a "
                "JOIN posts p ON p.slug=a.post_slug "
                "WHERE p.template_id=? AND a.label=? AND trim(a.answer)<>''",
                (old["id"], label),
            ).fetchone()[0]
            if filled:
                raise RuntimeError(f"{name}: {label} has {filled} saved answers; no rows were written")


def prepare_answer_merge(connection, before, after, merge_spec, expected_count, value_limit):
    """Validate one exact label migration and capture both answer rows for CAS."""
    if merge_spec is None:
        return None
    name, old_label, new_label = merge_spec
    if name not in before or old_label == new_label:
        raise ValueError("answer merge must name one known template and two distinct labels")
    old_labels = set(ASK.findall(before[name]["body"]))
    new_labels = set(ASK.findall(after[name]["body"]))
    if old_label not in old_labels or old_label in new_labels:
        raise ValueError("answer merge source must be a label removed by this update")
    if new_label not in old_labels or new_label not in new_labels:
        raise ValueError("answer merge destination must be an existing, retained label")
    rows = [dict(row) for row in connection.execute(
        "SELECT p.slug AS post_slug, source.answer AS source_answer, "
        "source.enabled AS source_enabled, source.updated_at AS source_updated_at, "
        "destination.answer AS destination_answer, destination.enabled AS destination_enabled, "
        "destination.updated_at AS destination_updated_at "
        "FROM posts p JOIN post_template_answers source "
        "ON source.post_slug=p.slug AND source.label=? "
        "LEFT JOIN post_template_answers destination "
        "ON destination.post_slug=p.slug AND destination.label=? "
        "WHERE p.template_id=? AND trim(source.answer)<>'' ORDER BY p.slug",
        (old_label, new_label, before[name]["id"]),
    )]
    if len(rows) != expected_count:
        raise RuntimeError(f"answer merge expected {expected_count} rows, found {len(rows)}; no rows were written")
    for row in rows:
        if row["destination_answer"] is None:
            raise RuntimeError("answer merge destination row is missing; no rows were written")
        if row["source_enabled"] != 1 or row["destination_enabled"] != 1:
            raise RuntimeError("answer merge requires both saved answers to be enabled; no rows were written")
        suffix = ("\n" if row["destination_answer"] else "") + MERGE_CAPTION + row["source_answer"]
        if suffix in row["destination_answer"]:
            raise RuntimeError("answer merge destination already contains the source text; no rows were written")
        row["merged_answer"] = row["destination_answer"] + suffix
        if len(row["merged_answer"]) > value_limit:
            raise RuntimeError(f"answer merge exceeds the {value_limit}-character answer limit; no rows were written")
    snapshot = [{key: row[key] for key in row if key != "merged_answer"} for row in rows]
    cas = digest(json.dumps(snapshot, ensure_ascii=False, sort_keys=True, separators=(",", ":")))
    return {"name": name, "old_label": old_label, "new_label": new_label, "rows": rows, "cas": cas}


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
    parser.add_argument("--merge-answer", nargs=3, metavar=("TEMPLATE", "OLD_LABEL", "NEW_LABEL"))
    parser.add_argument("--expected-answer-merges", type=int, metavar="N")
    parser.add_argument("--answer-merge-cas", metavar="SHA256")
    parser.add_argument("--answer-value-limit", type=int, default=500, metavar="N")
    args = parser.parse_args()
    before, after = load_inputs(args.before, args.after)
    if args.merge_answer is None and (args.expected_answer_merges is not None or args.answer_merge_cas is not None):
        parser.error("answer merge count and CAS require --merge-answer")
    if args.merge_answer is not None and (args.expected_answer_merges is None or args.expected_answer_merges < 1):
        parser.error("--merge-answer requires a positive --expected-answer-merges")
    if not 1 <= args.answer_value_limit <= 4000:
        parser.error("--answer-value-limit must be between 1 and 4000")
    if args.answer_merge_cas is not None and not re.fullmatch(r"[0-9a-f]{64}", args.answer_merge_cas):
        parser.error("--answer-merge-cas must be a lowercase SHA-256 digest")
    if args.apply and args.merge_answer is not None and args.answer_merge_cas is None:
        parser.error("--apply with --merge-answer requires a dry-run --answer-merge-cas")
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
        merge = prepare_answer_merge(
            connection, before, after, args.merge_answer, args.expected_answer_merges,
            args.answer_value_limit,
        )
        if merge is not None and args.answer_merge_cas is not None and merge["cas"] != args.answer_merge_cas:
            raise RuntimeError("saved answers changed since the answer-merge dry run; no rows were written")
        allowed_removed = (merge["name"], merge["old_label"]) if merge is not None else None
        refuse_hidden_saved_answers(connection, before, after, allowed_removed)
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
                locked_merge = prepare_answer_merge(
                    connection, before, after, args.merge_answer, args.expected_answer_merges,
                    args.answer_value_limit,
                )
                if locked_merge is not None and locked_merge["cas"] != args.answer_merge_cas:
                    raise RuntimeError("saved answers changed since the answer-merge dry run; no rows were written")
                refuse_hidden_saved_answers(connection, before, after, allowed_removed)
                counts = tuple(
                    connection.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0]
                    for table in ("posts", "post_template_answers")
                )
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
                answer_updates = {}
                if locked_merge is not None:
                    for row in locked_merge["rows"]:
                        key = (row["post_slug"], locked_merge["new_label"])
                        answer_updates[key] = row["merged_answer"]
                        result = connection.execute(
                            "UPDATE post_template_answers SET answer=?,updated_at=? "
                            "WHERE post_slug=? AND label=? AND answer=? AND enabled=? AND updated_at=?",
                            (row["merged_answer"], timestamp, row["post_slug"], locked_merge["new_label"],
                             row["destination_answer"], row["destination_enabled"], row["destination_updated_at"]),
                        )
                        if result.rowcount != 1:
                            raise RuntimeError("answer merge CAS update did not match exactly one row")
                if counts != tuple(
                    connection.execute(f"SELECT COUNT(*) FROM {table}").fetchone()[0]
                    for table in ("posts", "post_template_answers")
                ):
                    raise RuntimeError("a post or saved answer count changed")
                if dependent_data[0] != dependent_data_fingerprints(connection)[0]:
                    raise RuntimeError("post content changed")
                expected_answers = [
                    (slug, label, answer_updates[(slug, label)], enabled, timestamp)
                    if (slug, label) in answer_updates else (slug, label, answer, enabled, updated_at)
                    for slug, label, answer, enabled, updated_at in previous_answers
                ]
                if expected_answers != answer_rows(connection):
                    raise RuntimeError("a saved answer changed outside the exact merge plan")
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
            "answer_merge": (
                {"template": merge["name"], "from_label": merge["old_label"],
                 "into_label": merge["new_label"], "rows": len(merge["rows"]), "cas": merge["cas"]}
                if merge is not None else None
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
