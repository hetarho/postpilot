import ipaddress
import json
import os
import re
from pathlib import Path
from urllib.parse import parse_qs, unquote, urlparse
from urllib.request import ProxyHandler, Request, build_opener


_manifest_read = False
_resolved_assets = set()
_verified_uploaded_assets = set()
_verified_upload_sequence = []
_verified_readback_targets = {}
_reported_stages = []
_commit_fence_attempted = False
_commit_fence_acknowledged = False
_prepared_final_ref = ""
_authorized_final_ref = ""
_final_activation_used = False
_editor_manifest_verified = False
_last_editor_image_count = None
_pending_upload = None
_selected_category_evidence = None
_editor_verified_target_id = ""
_prepared_final_target_id = ""
_authorized_final_target_id = ""


def _job_dir():
    root = Path(os.environ["POSTPILOT_JOB_DIR"]).resolve(strict=True)
    if root.stat().st_mode & 0o077:
        raise RuntimeError("job directory is not owner-only")
    return root


def _manifest():
    path = (_job_dir() / "manifest.json").resolve(strict=True)
    if path.parent != _job_dir():
        raise RuntimeError("manifest escaped job directory")
    return json.loads(path.read_text(encoding="utf-8"))


def _post_progress(stage):
    callback = _callback_url()
    token = os.environ["POSTPILOT_CALLBACK_TOKEN"]
    request = Request(
        callback + "/progress",
        data=json.dumps({"stage": stage}).encode("utf-8"),
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"},
        method="POST",
    )
    # Never forward the loopback bearer to a user-configured HTTP proxy.
    with build_opener(ProxyHandler({})).open(request, timeout=15) as response:
        if response.status != 204:
            raise RuntimeError("server did not acknowledge progress")


def _post_terminal(result):
    callback = _callback_url()
    token = os.environ["POSTPILOT_CALLBACK_TOKEN"]
    request = Request(
        callback + "/finish",
        data=json.dumps(result, ensure_ascii=False).encode("utf-8"),
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"},
        method="POST",
    )
    with build_opener(ProxyHandler({})).open(request, timeout=15) as response:
        if response.status != 204:
            raise RuntimeError("agent did not acknowledge terminal result")


def _callback_url():
    raw = os.environ["POSTPILOT_CALLBACK_URL"]
    parsed = urlparse(raw)
    try:
        address = ipaddress.ip_address(parsed.hostname or "")
        port = parsed.port
    except ValueError as error:
        raise RuntimeError("publisher callback is not loopback") from error
    if (
        parsed.scheme != "http"
        or not address.is_loopback
        or port is None
        or parsed.username is not None
        or parsed.password is not None
        or parsed.query
        or parsed.fragment
        or parsed.path not in {"", "/"}
    ):
        raise RuntimeError("publisher callback is not a plain loopback origin")
    return raw.rstrip("/")


def _active_browser_page_targets():
    """Read the live page targets from the loopback CDP endpoint.

    The published URL is accepted only when the browser itself is currently on
    that exact page. Model text is never sufficient readback evidence.
    """
    parsed = urlparse(os.environ.get("BROWSER_CDP_URL", ""))
    try:
        address = ipaddress.ip_address(parsed.hostname or "")
        port = parsed.port
    except ValueError as error:
        raise RuntimeError("publisher could not verify the active browser") from error
    if (
        parsed.scheme not in {"ws", "wss"}
        or not address.is_loopback
        or port is None
        or parsed.username is not None
        or parsed.password is not None
    ):
        raise RuntimeError("publisher could not verify the active browser")
    scheme = "https" if parsed.scheme == "wss" else "http"
    request = Request(f"{scheme}://{parsed.hostname}:{port}/json/list", method="GET")
    with build_opener(ProxyHandler({})).open(request, timeout=2) as response:
        payload = response.read(65537)
        if response.status != 200 or len(payload) > 65536:
            raise RuntimeError("publisher could not inspect the active browser")
    targets = json.loads(payload)
    if not isinstance(targets, list):
        raise RuntimeError("publisher received malformed browser targets")
    pages = {}
    for target in targets:
        if not isinstance(target, dict) or target.get("type") != "page":
            continue
        target_id = str(target.get("id") or target.get("targetId") or "").strip()
        current_url = str(target.get("url") or "").strip()
        if not target_id or not current_url or target_id in pages:
            raise RuntimeError("publisher received incomplete browser target metadata")
        pages[target_id] = current_url
    if len(pages) != 1:
        raise RuntimeError("publisher requires exactly one dedicated browser page")
    return pages


def _active_browser_page_urls():
    """Compatibility projection used by diagnostics and older tests."""
    return set(_active_browser_page_targets().values())


def expected_category_id():
    category_id = str(_manifest().get("category_id", "")).strip()
    if not re.fullmatch(r"[A-Za-z0-9_-]{1,128}", category_id):
        raise RuntimeError("manifest category id cannot be bound to browser evidence")
    return category_id


def category_selection_selector(category_id):
    if not re.fullmatch(r"[A-Za-z0-9_-]{1,128}", str(category_id or "")):
        raise RuntimeError("category id is not safe for exact DOM selection")
    value = str(category_id)
    return ",".join((
        f'option[value="{value}"]:checked',
        f'[data-category-id="{value}"][aria-selected="true"]',
        f'[data-category-no="{value}"][aria-selected="true"]',
        f'[data-category-id="{value}"][aria-checked="true"]',
        f'[data-category-no="{value}"][aria-checked="true"]',
    ))


def read_manifest(args, **kwargs):
    try:
        global _manifest_read
        if args.get("handle", "") != os.environ["POSTPILOT_JOB_HANDLE"]:
            raise RuntimeError("job handle does not match the active publication")
        manifest = _manifest()
        _manifest_read = True
        return json.dumps(
            {
                "manifest": manifest,
                "browser_evidence": {
                    "selected_category_selector": category_selection_selector(
                        str(manifest.get("category_id", "")).strip()
                    )
                },
            },
            ensure_ascii=False,
        )
    except Exception as error:
        return json.dumps({"error": str(error)})


def resolve_asset(args, **kwargs):
    try:
        filename = args.get("filename", "")
        manifest = _manifest()
        allowed = {asset["filename"] for asset in manifest.get("assets", [])}
        if filename not in allowed or Path(filename).name != filename:
            raise RuntimeError("asset is not in the current manifest")
        path = (_job_dir() / filename).resolve(strict=True)
        if path.parent != _job_dir() or not path.is_file():
            raise RuntimeError("asset path escaped the current job")
        _resolved_assets.add(filename)
        return json.dumps({"filename": filename, "path": str(path)})
    except Exception as error:
        return json.dumps({"error": str(error)})


def _normalize(value):
    return " ".join(str(value or "").split())


def _ordered_manifest_assets(manifest):
    assets = manifest.get("assets", [])
    if not isinstance(assets, list):
        raise RuntimeError("manifest assets are malformed")
    ordered = []
    filenames = set()
    for expected_ordinal, asset in enumerate(assets):
        if not isinstance(asset, dict) or asset.get("ordinal", 0) != expected_ordinal:
            raise RuntimeError("manifest asset ordinals are not contiguous")
        filename = asset.get("filename", "")
        if not filename or Path(filename).name != filename or filename in filenames:
            raise RuntimeError("manifest asset filenames are invalid")
        filenames.add(filename)
        ordered.append(asset)

    image_blocks = [
        block for block in manifest.get("content", {}).get("blocks", [])
        if isinstance(block, dict) and str(block.get("type", "")).upper() == "IMAGE"
    ]
    if len(image_blocks) != len(ordered):
        raise RuntimeError("manifest image blocks and staged assets do not match")
    for block, asset in zip(image_blocks, ordered):
        if block.get("file", "") != asset.get("source_filename", ""):
            raise RuntimeError("manifest image source order does not match staged assets")
    return ordered


_SNAPSHOT_NODE = re.compile(
    r'^\s*-\s*(?P<role>[a-z][a-z0-9_-]*)(?:\s+"(?P<label>(?:\\.|[^"\\])*)")?(?P<state>.*)$',
    re.IGNORECASE,
)
_CONTENT_ROLES = {"heading", "paragraph", "img", "textbox", "input", "listitem", "text", "statictext"}


def _snapshot_nodes(snapshot):
    nodes = []
    for line in str(snapshot).splitlines():
        match = _SNAPSHOT_NODE.match(line)
        if match is None:
            continue
        label = match.group("label") or ""
        label = label.replace(r'\"', '"').replace(r"\\", "\\")
        state = str(match.group("state") or "").lower()
        nodes.append({
            "role": match.group("role").lower(),
            "label": _normalize(label),
            "selected": bool(re.search(
                r'\[(?:checked|selected)(?:=(?:true|"true"))?\]|\b(?:aria-checked|aria-selected)\s*=\s*"?true"?',
                state,
            )),
        })
    return nodes


def _expected_body_nodes(content, assets):
    expected = [("text", _normalize(content.get("title", "")))]
    image_ordinal = 0
    blocks = content.get("blocks", [])
    if not isinstance(blocks, list):
        raise RuntimeError("manifest blocks are malformed")
    for block in blocks:
        if not isinstance(block, dict):
            raise RuntimeError("manifest block is malformed")
        block_type = str(block.get("type", "")).upper()
        if block_type in {"TEXT", "HEADING"}:
            expected.append(("text", _normalize(block.get("content", ""))))
        elif block_type == "QUOTE":
            expected.append(("text", _normalize(f"“{block.get('content', '')}”")))
        elif block_type == "LIST":
            items = block.get("items", [])
            if not isinstance(items, list):
                raise RuntimeError("manifest list block is malformed")
            expected.extend(("text", _normalize(f"- {item}")) for item in items)
        elif block_type == "IMAGE":
            if image_ordinal >= len(assets):
                raise RuntimeError("manifest image blocks and staged assets do not match")
            expected.append(("img", ""))
            image_ordinal += 1
            caption = _normalize(block.get("caption", ""))
            if caption:
                expected.append(("text", caption))
        else:
            raise RuntimeError("manifest contains an unsupported block")
    if image_ordinal != len(assets) or not expected[0][1] or any(kind == "text" and not label for kind, label in expected):
        raise RuntimeError("manifest publishable content is incomplete")
    return expected


def _node_matches_expected(node, expected):
    role, label = node["role"], node["label"]
    kind, expected_label = expected
    if kind == "img":
        return role == "img"
    return role != "img" and label == expected_label


def _has_exact_setting(nodes, value):
    expected = _normalize(value)
    if not expected:
        return False
    return any(node["label"] == expected for node in nodes)


def _canonical_tag(value):
    return _normalize(value).lstrip("#").strip()


def _snapshot_tags(nodes):
    # Frozen tags are exposed as hashtag-named controls in the editor and links
    # after publication. Compare the complete ordered collection so stale,
    # duplicated, or model-inserted tags cannot pass a presence-only check.
    tags = []
    for node in nodes:
        label = node["label"]
        if node["role"] not in {"textbox", "input", "button", "link"} or not label.startswith("#"):
            continue
        tag = _canonical_tag(label)
        if tag:
            tags.append(tag)
    return tags


def _has_selected_setting(nodes, value, roles):
    expected = _normalize(value)
    return bool(expected) and any(
        node["label"] == expected and node["role"] in roles and node["selected"]
        for node in nodes
    )


def _is_setting_node(node, category_name, expected_tags, expected_visibility):
    if (
        node["label"] == category_name
        and node["role"] in {"option", "radio", "menuitemradio", "link", "button"}
    ):
        return True
    if (
        expected_visibility
        and node["label"] == expected_visibility
        and node["role"] in {"option", "radio", "menuitemradio"}
    ):
        return True
    return (
        node["role"] in {"textbox", "input", "button", "link"}
        and node["label"].startswith("#")
        and _canonical_tag(node["label"]) in expected_tags
    )


def _exact_body_sequence(nodes, expected, category_name, expected_tags, expected_visibility):
    """Require the title/body to be the whole semantic region before settings.

    SmartEditor has surrounding chrome, so unrelated nodes outside the post are
    ignored. Once the exact title starts, however, every paragraph/heading/image
    until the first frozen setting must match the manifest with no insertion.
    """
    for start, node in enumerate(nodes):
        if not _node_matches_expected(node, expected[0]):
            continue
        end = len(nodes)
        for index in range(start + 1, len(nodes)):
            if _is_setting_node(
                nodes[index], category_name, expected_tags, expected_visibility
            ):
                end = index
                break
        if end == len(nodes):
            continue
        actual = [node for node in nodes[start:end] if node["role"] in _CONTENT_ROLES]
        if len(actual) == len(expected) and all(
            _node_matches_expected(actual_node, expected_node)
            for actual_node, expected_node in zip(actual, expected)
        ):
            return True
    return False


def _snapshot_matches_manifest(manifest, snapshot, require_settings):
    """Match the exact canonical semantic region in trusted accessibility data."""
    nodes = _snapshot_nodes(snapshot)
    content = manifest.get("content", {})
    if not isinstance(content, dict):
        return False

    try:
        assets = _ordered_manifest_assets(manifest)
        expected_body = _expected_body_nodes(content, assets)
    except RuntimeError:
        return False

    category_name = _normalize(manifest.get("category_name", ""))
    tags = manifest.get("tags", [])
    if not isinstance(tags, list):
        return False
    expected_tags = [_canonical_tag(tag) for tag in tags]
    if (
        any(not tag for tag in expected_tags)
        or len(set(expected_tags)) != len(expected_tags)
        or _snapshot_tags(nodes) != expected_tags
    ):
        return False
    expected_visibility = None
    if require_settings:
        if not _has_selected_setting(
            nodes, category_name, {"option", "radio", "menuitemradio"}
        ):
            return False
        visibility = manifest.get("visibility")
        expected_visibility = {
            "PUBLISH_VISIBILITY_PUBLIC": "전체 공개",
            "PUBLISH_VISIBILITY_PRIVATE": "비공개",
            1: "전체 공개",
            2: "비공개",
        }.get(visibility)
        if not expected_visibility or not _has_selected_setting(
            nodes, expected_visibility, {"option", "radio", "menuitemradio"}
        ):
            return False
    elif not _has_exact_setting(nodes, category_name):
        return False
    return _exact_body_sequence(
        nodes,
        expected_body,
        category_name,
        set(expected_tags),
        expected_visibility,
    )


def _url_matches_expected_account(value, expected, *, published=False):
    parsed = urlparse(str(value or ""))
    if parsed.scheme != "https" or parsed.hostname not in {"blog.naver.com", "m.blog.naver.com"}:
        return False
    expected = str(expected or "").strip()
    if not expected:
        return False
    parts = [unquote(part) for part in parsed.path.split("/") if part]
    if published:
        return (
            parsed.hostname == "blog.naver.com"
            and len(parts) == 2
            and parts[0] == expected
            and parts[1].isdigit()
        )
    if parts and parts[0] == expected:
        return True
    query = parse_qs(parsed.query, keep_blank_values=True)
    return any(
        key.lower() == "blogid" and values == [expected]
        for key, values in query.items()
    )


def invalidate_editor_verification():
    global _editor_manifest_verified, _selected_category_evidence
    global _editor_verified_target_id
    if not _commit_fence_acknowledged:
        _editor_manifest_verified = False
        _editor_verified_target_id = ""
        _selected_category_evidence = None


def observe_category_selection(category_id, target_id):
    global _selected_category_evidence
    if category_id != expected_category_id() or not target_id:
        raise RuntimeError("selected category evidence does not match the frozen manifest")
    _selected_category_evidence = {
        "category_id": category_id,
        "target_id": target_id,
    }


def begin_asset_upload(paths):
    """Arm evidence collection for one resolved JPEG and one editor snapshot."""
    global _pending_upload
    if _pending_upload is not None:
        raise RuntimeError("the previous JPEG is not yet visible in an editor snapshot")
    if len(paths) != 1:
        raise RuntimeError("upload exactly one staged JPEG at a time")
    path = Path(paths[0]).resolve(strict=True)
    root = _job_dir()
    if path.parent != root or path.name not in _resolved_assets:
        raise RuntimeError("upload the currently resolved staged JPEG only")
    if path.name in _verified_uploaded_assets:
        raise RuntimeError("the staged JPEG was already verified in the editor")
    ordered = _ordered_manifest_assets(_manifest())
    next_ordinal = len(_verified_upload_sequence)
    if next_ordinal >= len(ordered) or path.name != ordered[next_ordinal]["filename"]:
        raise RuntimeError("upload staged JPEGs in exact manifest ordinal order")
    if _last_editor_image_count is None:
        raise RuntimeError("take a fresh editor snapshot before uploading a JPEG")
    _pending_upload = {
        "filename": path.name,
        "baseline": _last_editor_image_count,
        "cdp_succeeded": False,
    }


def complete_asset_upload(succeeded):
    """Record only a successful DOM.setFileInputFiles result as upload evidence."""
    global _pending_upload
    if _pending_upload is None:
        return
    if not succeeded:
        _pending_upload = None
        return
    _pending_upload["cdp_succeeded"] = True


def observe_editor_snapshot(snapshot, page_url="", target_id=""):
    """Require one exact image gain, then verify the complete frozen editor state."""
    global _last_editor_image_count, _pending_upload, _editor_manifest_verified
    global _editor_verified_target_id
    image_count = sum(
        1 for line in str(snapshot).splitlines()
        if re.match(r"^\s*-?\s*img(?:\s|$)", line, re.IGNORECASE)
    )
    if (
        _pending_upload is not None
        and _pending_upload["cdp_succeeded"]
        and image_count == _pending_upload["baseline"] + 1
    ):
        filename = _pending_upload["filename"]
        _verified_uploaded_assets.add(filename)
        _verified_upload_sequence.append(filename)
        _pending_upload = None
    _last_editor_image_count = image_count
    if (
        _reported_stages
        and _reported_stages[-1] == "uploading_photos"
        and _pending_upload is None
    ):
        manifest = _manifest()
        required = [asset["filename"] for asset in _ordered_manifest_assets(manifest)]
        _editor_manifest_verified = (
            _verified_upload_sequence == required
            and _selected_category_evidence == {
                "category_id": expected_category_id(),
                "target_id": target_id,
            }
            and _url_matches_expected_account(
                page_url,
                manifest.get("expected_platform_account_id", ""),
            )
            and _snapshot_matches_manifest(manifest, snapshot, require_settings=True)
        )
        _editor_verified_target_id = target_id if _editor_manifest_verified else ""


def observe_readback(page_url, snapshot, target_id=""):
    """Bind completion to the exact current page that supplied this snapshot."""
    if (
        not _commit_fence_acknowledged
        or not _reported_stages
        or _reported_stages[-1] != "verifying"
    ):
        return
    manifest = _manifest()
    expected = manifest.get("expected_platform_account_id", "")
    if not _snapshot_matches_manifest(manifest, snapshot, require_settings=False):
        return
    if _url_matches_expected_account(page_url, expected, published=True):
        _verified_readback_targets[page_url] = target_id


def report_progress(args, **kwargs):
    try:
        stage = args.get("stage", "")
        if stage not in {"preparing", "opening_editor", "filling_content", "uploading_photos", "verifying"}:
            raise RuntimeError("unsupported progress stage")
        if stage == "verifying" and (
            not _commit_fence_acknowledged or not _final_activation_used
        ):
            raise RuntimeError("verification cannot start before the one authorized final activation")
        _post_progress(stage)
        _reported_stages.append(stage)
        return json.dumps({"acknowledged": stage})
    except Exception as error:
        return json.dumps({"error": str(error)})


def enter_commit_fence(args, **kwargs):
    global _commit_fence_attempted, _commit_fence_acknowledged
    global _prepared_final_ref, _authorized_final_ref, _final_activation_used
    global _prepared_final_target_id, _authorized_final_target_id
    _commit_fence_attempted = True
    _commit_fence_acknowledged = False
    _authorized_final_ref = ""
    _authorized_final_target_id = ""
    _final_activation_used = False
    try:
        final_ref = str(args.get("final_ref", "")).strip().lstrip("@").lower()
        if not final_ref or final_ref != _prepared_final_ref:
            raise RuntimeError("the final Naver control was not grounded in the current snapshot")
        if not _manifest_read or not _reported_stages or _reported_stages[-1] != "uploading_photos":
            raise RuntimeError("publisher preparation did not reach the upload verification stage")
        required = [asset["filename"] for asset in _ordered_manifest_assets(_manifest())]
        if (
            _verified_upload_sequence != required
            or _verified_uploaded_assets != set(required)
            or _pending_upload is not None
            or not _editor_manifest_verified
            or not _editor_verified_target_id
            or _prepared_final_target_id != _editor_verified_target_id
        ):
            raise RuntimeError("the exact frozen manifest is not verified in the editor")
        _post_progress("committing")
        _reported_stages.append("committing")
        _commit_fence_acknowledged = True
        _authorized_final_ref = final_ref
        _authorized_final_target_id = _prepared_final_target_id
        _prepared_final_ref = ""
        _prepared_final_target_id = ""
        return json.dumps({"acknowledged": "committing", "final_click_permitted": True})
    except Exception as error:
        _prepared_final_ref = ""
        _prepared_final_target_id = ""
        return json.dumps({"error": str(error), "final_click_permitted": False})


def prepare_commit_control(final_ref, target_id):
    global _prepared_final_ref, _prepared_final_target_id
    if _commit_fence_attempted or _commit_fence_acknowledged:
        raise RuntimeError("the commit fence cannot be prepared twice")
    normalized = str(final_ref or "").strip().lstrip("@").lower()
    if not normalized or not target_id:
        raise RuntimeError("the final Naver control reference and target are required")
    _prepared_final_ref = normalized
    _prepared_final_target_id = target_id


def consume_final_activation(final_ref, target_id):
    global _final_activation_used
    normalized = str(final_ref or "").strip().lstrip("@").lower()
    if (
        not _commit_fence_acknowledged
        or _final_activation_used
        or normalized != _authorized_final_ref
        or target_id != _authorized_final_target_id
    ):
        raise RuntimeError("only the one fenced Naver publish control may be activated")
    # Consume before the browser call. A failed/ambiguous click is never retried.
    _final_activation_used = True


def final_activation_used():
    return _final_activation_used


def final_click_is_blocked():
    """Block activation throughout the verified-photo → durable-fence boundary."""
    awaiting_fence = bool(_reported_stages) and _reported_stages[-1] == "uploading_photos"
    return (awaiting_fence or _commit_fence_attempted) and not _commit_fence_acknowledged


def commit_fence_acknowledged():
    return _commit_fence_acknowledged


def finish(args, **kwargs):
    try:
        status = args.get("status")
        if status == "published":
            if (
                not _commit_fence_acknowledged
                or not _final_activation_used
                or not _reported_stages
                or _reported_stages[-1] != "verifying"
            ):
                raise RuntimeError("published result was reported before verified readback")
            raw_url = args.get("published_url", "")
            expected = _manifest().get("expected_platform_account_id", "")
            if not _url_matches_expected_account(raw_url, expected, published=True):
                raise RuntimeError("published URL does not belong to the paired blog")
            pages = _active_browser_page_targets()
            target_id, current_url = next(iter(pages.items()))
            if (
                raw_url != current_url
                or _verified_readback_targets.get(raw_url) != target_id
                or target_id != _authorized_final_target_id
            ):
                raise RuntimeError("published URL is not open in the active paired browser")
            result = {"status": "published", "published_url": raw_url}
        else:
            kind = args.get("failure_kind", "safe")
            result = {"status": "failed", "failure_kind": kind, "detail": args.get("detail", "")[:500]}
        _post_terminal(result)
        return json.dumps(result, ensure_ascii=False)
    except Exception as error:
        return json.dumps({"status": "failed", "failure_kind": "safe", "detail": str(error)})
