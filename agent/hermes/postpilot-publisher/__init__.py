from pathlib import Path
import ipaddress
import json
import os
import re
from urllib.parse import urlparse
from urllib.request import ProxyHandler, Request, build_opener

from . import schemas, tools


OWN_TOOLS = {
    "postpilot_read_manifest",
    "postpilot_resolve_asset",
    "postpilot_report_progress",
    "postpilot_enter_commit_fence",
    "postpilot_finish",
}
ALLOWED_HOSTS = {"blog.naver.com", "nid.naver.com", "m.blog.naver.com"}
# Naver redirects the generic Blog entry point to its signed-in Blog Home. This
# host is pairing-only: the publisher itself remains restricted to the editor
# hosts above.
PAIRING_ALLOWED_HOSTS = ALLOWED_HOSTS | {"www.naver.com", "section.blog.naver.com"}
SAFE_BROWSER_TOOLS = {
    "browser_navigate",
    "browser_snapshot",
    "browser_click",
    "browser_type",
    "browser_scroll",
    "browser_press",
    "browser_back",
    "browser_dialog",
    "browser_console",
    "browser_cdp",
}
PAIRING_BROWSER_TOOLS = {
    "browser_navigate",
    "browser_snapshot",
    "browser_vision",
    "browser_click",
    "browser_scroll",
    "browser_back",
    "browser_console",
    "browser_cdp",
}
SAFE_CDP_METHODS = {
    "Target.getTargets",
    "DOM.getDocument",
    "DOM.querySelector",
    "DOM.querySelectorAll",
    "DOM.describeNode",
    "DOM.setFileInputFiles",
}
# This is a versioned compatibility signature, not a keyword search. A new Naver
# label must fail closed until a live compatibility probe explicitly adds it.
FINAL_ACTIVATION_LABELS = frozenset({"발행", "네이버에 발행"})
FINAL_ACTIVATION_TERMS = ("발행", "등록", "완료", "publish", "submit", "post")
SAFE_PRECOMMIT_BUTTON_TERMS = (
    "카테고리",
    "사진",
    "이미지",
    "태그",
    "공개 설정",
    "전체공개",
    "이웃공개",
    "비공개",
    "취소",
    "닫기",
)
_browser_policy_violation = ""
_snapshot_refs = {}
_trusted_page = None
_pending_browser_target = None
_document_roots = {}
_pending_category_query = None
_LOCATION_EXPRESSIONS = {"window.location.href", "document.location.href", "location.href"}


def _pairing_mode():
    return os.environ.get("POSTPILOT_MODE", "").strip().lower() == "pairing"


def _allowed_hosts():
    return PAIRING_ALLOWED_HOSTS if _pairing_mode() else ALLOWED_HOSTS


def _active_browser_targets():
    parsed = urlparse(os.environ.get("BROWSER_CDP_URL", ""))
    try:
        address = ipaddress.ip_address(parsed.hostname or "")
        port = parsed.port
    except ValueError as error:
        raise RuntimeError("browser policy could not verify the local CDP endpoint") from error
    if (
        parsed.scheme not in {"ws", "wss"}
        or not address.is_loopback
        or port is None
        or parsed.username is not None
        or parsed.password is not None
    ):
        raise RuntimeError("browser policy could not verify the local CDP endpoint")
    scheme = "https" if parsed.scheme == "wss" else "http"
    request = Request(f"{scheme}://{parsed.hostname}:{port}/json/list", method="GET")
    with build_opener(ProxyHandler({})).open(request, timeout=2) as response:
        payload = response.read(65537)
        if response.status != 200 or len(payload) > 65536:
            raise RuntimeError("browser policy could not inspect the active page")
    targets = json.loads(payload)
    if not isinstance(targets, list):
        raise RuntimeError("browser policy received malformed target metadata")
    pages = []
    for target in targets:
        if not isinstance(target, dict) or target.get("type") != "page":
            continue
        target_id = str(target.get("id") or target.get("targetId") or "").strip()
        current_url = str(target.get("url") or "").strip()
        if not target_id or not current_url:
            raise RuntimeError("browser policy received incomplete page metadata")
        pages.append({"id": target_id, "url": current_url})
    return pages


def _active_browser_urls():
    """Compatibility projection used by older diagnostics and tests."""
    return [target["url"] for target in _active_browser_targets()]


def _verify_current_browser_target(*, allow_blank=False):
    pages = _active_browser_targets()
    if len(pages) != 1:
        raise RuntimeError("publisher requires exactly one dedicated browser page")
    target = pages[0]
    if target["url"] in {"about:blank", "chrome://newtab/"} and allow_blank:
        return target
    parsed = urlparse(target["url"])
    if parsed.scheme != "https" or parsed.hostname not in _allowed_hosts():
        raise RuntimeError("browser left the approved Naver hosts")
    return target


def _verify_current_browser_hosts():
    """Compatibility projection; publication policy uses the target-bound form."""
    return [_verify_current_browser_target(allow_blank=True)["url"]]


def _normalize_ref(value):
    return str(value or "").strip().lstrip("@").lower()


def _capture_snapshot(result, full=False, page_url="", target_id=""):
    """Cache only refs and labels returned by Hermes' real accessibility snapshot."""
    global _snapshot_refs
    try:
        payload = json.loads(result) if isinstance(result, str) else result
        snapshot = payload.get("snapshot", "") if isinstance(payload, dict) else ""
        succeeded = isinstance(payload, dict) and payload.get("success") is True
    except (TypeError, ValueError):
        snapshot = ""
        succeeded = False
    refs = {}
    for line in snapshot.splitlines():
        node = re.match(
            r'^\s*-\s*(?P<role>[a-z][a-z0-9_-]*)(?:\s+"(?P<label>(?:\\.|[^"\\])*)")?',
            line,
            re.IGNORECASE,
        )
        role = node.group("role").lower() if node is not None else ""
        label = node.group("label") if node is not None else ""
        label = (label or "").replace(r'\"', '"').replace(r"\\", "\\").strip().lower()
        for match in re.finditer(r"\[(?:ref=)?@?(e\d+)\]", line, re.IGNORECASE):
            refs[_normalize_ref(match.group(1))] = {
                "description": line.lower(),
                "role": role,
                "label": label,
                "target_id": target_id,
            }
    _snapshot_refs = refs
    if succeeded and full:
        tools.observe_editor_snapshot(snapshot, page_url, target_id)
    return snapshot if succeeded else None


def _looks_like_final_activation(tool_name, args):
    """Recognize irreversible controls from the trusted snapshot, not model prose."""
    if tool_name == "browser_press":
        return str(args.get("key", "")).lower() in {"enter", "return", "space", " "}
    if tool_name == "browser_dialog":
        return str(args.get("action", "")).lower() == "accept"
    if tool_name != "browser_click":
        return False
    evidence = _snapshot_refs.get(_normalize_ref(args.get("ref")))
    if evidence is None:
        # Hermes supports clicks only through refs from a snapshot. Unknown/stale refs
        # fail closed before the irreversible boundary.
        return True
    description = evidence["description"]
    if any(term in description for term in FINAL_ACTIVATION_TERMS):
        return True
    # Naver's irreversible control is a button. Before the durable fence, only
    # versioned reversible editor buttons are allowed; a renamed/new button is
    # treated as a possible publish control and fails closed.
    is_button = re.search(r"(?:^|[\s-])button(?:\s|\")", description) is not None
    return is_button and not any(term in description for term in SAFE_PRECOMMIT_BUTTON_TERMS)


def _prepare_commit_fence(args):
    final_ref = _normalize_ref(args.get("final_ref"))
    evidence = _snapshot_refs.get(final_ref)
    try:
        target = _verify_current_browser_target()
    except Exception as error:
        return {"action": "block", "message": str(error)}
    candidates = [
        ref
        for ref, candidate in _snapshot_refs.items()
        if candidate.get("role") == "button"
        and candidate.get("label") in FINAL_ACTIVATION_LABELS
        and candidate.get("target_id") == target["id"]
    ]
    if (
        evidence is None
        or evidence.get("target_id") != target["id"]
        or evidence.get("role") != "button"
        or evidence.get("label") not in FINAL_ACTIVATION_LABELS
        or candidates != [final_ref]
    ):
        return {
            "action": "block",
            "message": "Exactly one versioned Naver publish control is required in the current accessibility snapshot.",
        }
    try:
        tools.prepare_commit_control(final_ref, target["id"])
    except Exception as error:
        return {"action": "block", "message": str(error)}
    return None

def _guard(tool_name, args, task_id=None, **kwargs):
    global _pending_browser_target, _pending_category_query
    if _browser_policy_violation and tool_name != "postpilot_finish":
        return {"action": "block", "message": _browser_policy_violation}
    if _pairing_mode():
        return _guard_pairing(tool_name, args)
    if tool_name == "postpilot_enter_commit_fence":
        return _prepare_commit_fence(args)
    if tool_name in OWN_TOOLS:
        return None
    if tool_name not in SAFE_BROWSER_TOOLS:
        return {"action": "block", "message": "The publisher profile permits only browser and Postpilot publisher tools."}
    try:
        target = _verify_current_browser_target(allow_blank=tool_name == "browser_navigate")
    except Exception as error:
        return {"action": "block", "message": str(error)}
    _pending_browser_target = target

    if tools.commit_fence_acknowledged():
        if not tools.final_activation_used():
            if tool_name != "browser_click":
                return {
                    "action": "block",
                    "message": "The fenced Naver publish control must be the next browser action.",
                }
            try:
                tools.consume_final_activation(args.get("ref"), target["id"])
            except Exception as error:
                return {"action": "block", "message": str(error)}
        elif tool_name not in {"browser_navigate", "browser_snapshot", "browser_scroll", "browser_console"}:
            return {
                "action": "block",
                "message": "No browser mutation is permitted after the one fenced publish activation.",
            }
        elif tool_name == "browser_console" and not args.get("expression"):
            return {
                "action": "block",
                "message": "Only the current page URL may be read after the publish activation.",
            }
    if tool_name in {"browser_click", "browser_press", "browser_dialog"} and (
        tools.final_click_is_blocked() or
        (not tools.commit_fence_acknowledged() and _looks_like_final_activation(tool_name, args))
    ):
        return {
            "action": "block",
            "message": (
                "The durable Postpilot commit fence was not acknowledged. Browser activation is blocked; "
                "repair the fence or finish with a safe failure."
            ),
        }
    if tool_name == "browser_navigate":
        destination = args.get("url", "")
        parsed = urlparse(destination)
        if parsed.scheme != "https" or parsed.hostname not in ALLOWED_HOSTS:
            return {"action": "block", "message": "Navigation outside the approved Naver hosts is blocked."}

    if tool_name == "browser_console" and args.get("expression"):
        if str(args.get("expression", "")).strip() not in _LOCATION_EXPRESSIONS:
            return {"action": "block", "message": "Only the current page URL may be read through JavaScript."}

    if tool_name == "browser_cdp":
        method = args.get("method", "")
        if method not in SAFE_CDP_METHODS:
            return {"action": "block", "message": "This raw CDP method is disabled for the publisher profile."}
        if args.get("frame_id") or args.get("session_id"):
            return {"action": "block", "message": "Frame/session-scoped CDP calls are disabled for publishing."}
        if method == "Target.getTargets":
            if args.get("target_id"):
                return {"action": "block", "message": "Target enumeration cannot be page-scoped."}
        elif str(args.get("target_id") or "") != target["id"]:
            return {"action": "block", "message": "The CDP call is not bound to the active dedicated page."}
        if method == "DOM.querySelector":
            params = args.get("params", {})
            category_id = tools.expected_category_id()
            expected_selector = tools.category_selection_selector(category_id)
            if (
                params.get("selector") == expected_selector
                and params.get("nodeId") == _document_roots.get(target["id"])
            ):
                _pending_category_query = {
                    "target_id": target["id"],
                    "category_id": category_id,
                }
            else:
                _pending_category_query = None
        if method == "DOM.setFileInputFiles":
            job_root = Path(os.environ["POSTPILOT_JOB_DIR"]).resolve()
            files = args.get("params", {}).get("files", [])
            if not files:
                return {"action": "block", "message": "A staged manifest JPEG is required."}
            for value in files:
                candidate = Path(value)
                try:
                    resolved = candidate.resolve(strict=True)
                    resolved.relative_to(job_root)
                except (OSError, ValueError):
                    return {"action": "block", "message": "Local paths outside the current staged job are blocked."}
                if not resolved.is_file() or resolved.suffix.lower() not in {".jpg", ".jpeg"}:
                    return {"action": "block", "message": "Only current staged JPEG files may be uploaded."}
            try:
                tools.begin_asset_upload(files)
            except Exception as error:
                return {"action": "block", "message": str(error)}
    return None


def _guard_pairing(tool_name, args):
    """Permit only reversible Naver navigation while Hermes locates Write."""
    global _pending_browser_target
    if tool_name in OWN_TOOLS:
        return {"action": "block", "message": "Publishing tools are disabled during Naver pairing."}
    if tool_name not in PAIRING_BROWSER_TOOLS:
        return {"action": "block", "message": "Naver pairing permits only reversible browser navigation."}
    try:
        target = _verify_current_browser_target(allow_blank=tool_name == "browser_navigate")
    except Exception as error:
        return {"action": "block", "message": str(error)}
    _pending_browser_target = target
    if tool_name == "browser_navigate":
        parsed = urlparse(args.get("url", ""))
        if parsed.scheme != "https" or parsed.hostname not in PAIRING_ALLOWED_HOSTS:
            return {"action": "block", "message": "Navigation outside approved Naver pairing hosts is blocked."}
    if tool_name == "browser_console" and str(args.get("expression", "")).strip() not in _LOCATION_EXPRESSIONS:
        return {"action": "block", "message": "Only the current page URL may be read through JavaScript."}
    if tool_name == "browser_cdp" and (
        args.get("method") != "Target.getTargets" or args.get("target_id")
    ):
        return {"action": "block", "message": "Only page-target enumeration is allowed during pairing."}
    if tool_name == "browser_click":
        evidence = _snapshot_refs.get(_normalize_ref(args.get("ref")))
        if evidence is None or evidence.get("target_id") != target["id"]:
            return {"action": "block", "message": "Pairing clicks require a fresh current-page snapshot ref."}
        label = (evidence.get("label") or "").lower()
        description = (evidence.get("description") or "").lower()
        if any(term in label or term in description for term in FINAL_ACTIVATION_TERMS):
            return {"action": "block", "message": "Publish activation is disabled during Naver pairing."}
    return None


def _verify_browser_result(tool_name, args, result, **kwargs):
    if _pairing_mode():
        if tool_name not in PAIRING_BROWSER_TOOLS:
            return None
        return _verify_pairing_browser_result(tool_name, args, result)
    if tool_name not in SAFE_BROWSER_TOOLS:
        return None
    global _browser_policy_violation, _trusted_page, _pending_browser_target
    global _pending_category_query
    try:
        before = _pending_browser_target
        after = _verify_current_browser_target(allow_blank=tool_name == "browser_navigate")
        _pending_browser_target = None
        if before is None or before["id"] != after["id"]:
            raise RuntimeError("browser target changed during the action")
        if tool_name not in {"browser_snapshot", "browser_console"}:
            _trusted_page = None
        is_read_only = tool_name in {"browser_snapshot", "browser_console"} or (
            tool_name == "browser_cdp"
            and args.get("method") in {
                "Target.getTargets", "DOM.getDocument", "DOM.querySelector",
                "DOM.querySelectorAll", "DOM.describeNode",
            }
        )
        if not is_read_only:
            tools.invalidate_editor_verification()
        if tool_name == "browser_console" and args.get("expression"):
            payload = json.loads(result) if isinstance(result, str) else result
            current = payload.get("result", "") if isinstance(payload, dict) and payload.get("success") is True else ""
            parsed = urlparse(current) if isinstance(current, str) else None
            if parsed is None or parsed.scheme != "https" or parsed.hostname not in ALLOWED_HOSTS:
                raise RuntimeError("browser policy could not bind the current page URL")
            if current != after["url"]:
                raise RuntimeError("browser URL evidence came from a different page")
            _trusted_page = {"url": current, "target_id": after["id"]}
        if tool_name == "browser_cdp":
            try:
                payload = json.loads(result) if isinstance(result, str) else result
            except (TypeError, ValueError):
                payload = None
            method = args.get("method")
            if method == "DOM.getDocument":
                root = payload.get("result", {}).get("root", {}) if isinstance(payload, dict) else {}
                node_id = root.get("nodeId") if isinstance(root, dict) else None
                if payload.get("success") is True and isinstance(node_id, int) and node_id > 0:
                    _document_roots[after["id"]] = node_id
                else:
                    _document_roots.pop(after["id"], None)
            elif method == "DOM.querySelector" and _pending_category_query is not None:
                evidence = _pending_category_query
                _pending_category_query = None
                node_id = payload.get("result", {}).get("nodeId") if isinstance(payload, dict) else None
                if (
                    payload.get("success") is True
                    and isinstance(node_id, int)
                    and node_id > 0
                    and evidence["target_id"] == after["id"]
                ):
                    tools.observe_category_selection(evidence["category_id"], after["id"])
            elif method == "DOM.setFileInputFiles":
                tools.complete_asset_upload(
                    isinstance(payload, dict)
                    and payload.get("success") is True
                    and not payload.get("error")
                )
        if tool_name in {"browser_navigate", "browser_snapshot"}:
            full = tool_name == "browser_snapshot" and args.get("full") is True
            trusted = _trusted_page if full else None
            if full and (trusted is None or trusted["target_id"] != after["id"]):
                raise RuntimeError("full snapshot was not preceded by target-bound URL evidence")
            page_url = trusted["url"] if trusted is not None else ""
            snapshot = _capture_snapshot(
                result, full=full, page_url=page_url, target_id=after["id"]
            )
            if snapshot is not None and full:
                tools.observe_readback(page_url, snapshot, after["id"])
            if full:
                _trusted_page = None
        elif tool_name in SAFE_BROWSER_TOOLS:
            # Any DOM-changing interaction invalidates Hermes' ephemeral ref map. The
            # next click must be grounded in a fresh accessibility snapshot.
            _snapshot_refs.clear()
        return None
    except Exception:
        _pending_browser_target = None
        _pending_category_query = None
        _snapshot_refs.clear()
        _browser_policy_violation = (
            "Browser navigation escaped the approved Naver hosts. Stop immediately and call "
            "postpilot_finish with a safe failure; no further browser action or commit is permitted."
        )
        return json.dumps({"error": _browser_policy_violation})


def _verify_pairing_browser_result(tool_name, args, result):
    global _browser_policy_violation, _pending_browser_target
    try:
        before = _pending_browser_target
        after = _verify_current_browser_target(allow_blank=tool_name == "browser_navigate")
        _pending_browser_target = None
        if before is None or before["id"] != after["id"]:
            raise RuntimeError("browser target changed during pairing")
        if tool_name == "browser_snapshot":
            _capture_snapshot(result, full=False, target_id=after["id"])
        elif tool_name not in {"browser_console", "browser_cdp", "browser_vision"}:
            _snapshot_refs.clear()
        return None
    except Exception:
        _pending_browser_target = None
        _snapshot_refs.clear()
        _browser_policy_violation = (
            "Naver pairing left the approved hosts or changed browser target. Stop without publishing."
        )
        return json.dumps({"error": _browser_policy_violation})


def register(ctx):
    ctx.register_tool(name="postpilot_read_manifest", toolset="postpilot-publisher", schema=schemas.READ_MANIFEST, handler=tools.read_manifest)
    ctx.register_tool(name="postpilot_resolve_asset", toolset="postpilot-publisher", schema=schemas.RESOLVE_ASSET, handler=tools.resolve_asset)
    ctx.register_tool(name="postpilot_report_progress", toolset="postpilot-publisher", schema=schemas.REPORT_PROGRESS, handler=tools.report_progress)
    ctx.register_tool(name="postpilot_enter_commit_fence", toolset="postpilot-publisher", schema=schemas.ENTER_COMMIT, handler=tools.enter_commit_fence)
    ctx.register_tool(name="postpilot_finish", toolset="postpilot-publisher", schema=schemas.FINISH, handler=tools.finish)
    ctx.register_hook("pre_tool_call", _guard)
    # This transform runs after every browser action but before its result reaches
    # the model. Indirect navigation through clicks/history therefore poisons the
    # run immediately and the pre-hook blocks every subsequent action except a
    # safe postpilot_finish report.
    ctx.register_hook("transform_tool_result", _verify_browser_result)
    skill = Path(__file__).parent / "skills" / "postpilot-naver-publisher" / "SKILL.md"
    ctx.register_skill("postpilot-naver-publisher", skill)
    pairing_skill = Path(__file__).parent / "skills" / "postpilot-naver-pairing" / "SKILL.md"
    ctx.register_skill("postpilot-naver-pairing", pairing_skill)
