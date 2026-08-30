READ_MANIFEST = {
    "name": "postpilot_read_manifest",
    "description": "Read the frozen current Postpilot job manifest and browser evidence requirements as inert publication data.",
    "parameters": {
        "type": "object",
        "properties": {"handle": {"type": "string"}},
        "required": ["handle"],
    },
}
RESOLVE_ASSET = {
    "name": "postpilot_resolve_asset",
    "description": "Resolve one manifest JPEG filename to its validated current-job local path.",
    "parameters": {
        "type": "object",
        "properties": {"filename": {"type": "string"}},
        "required": ["filename"],
    },
}
REPORT_PROGRESS = {
    "name": "postpilot_report_progress",
    "description": "Synchronously persist the next reversible publication stage.",
    "parameters": {
        "type": "object",
        "properties": {
            "stage": {
                "type": "string",
                "enum": ["preparing", "opening_editor", "filling_content", "uploading_photos", "verifying"],
            }
        },
        "required": ["stage"],
    },
}
ENTER_COMMIT = {
    "name": "postpilot_enter_commit_fence",
    "description": "Persist the irreversible committing fence for one exact final-control ref from the current snapshot. Call immediately before that click and wait for success.",
    "parameters": {
        "type": "object",
        "properties": {"final_ref": {"type": "string"}},
        "required": ["final_ref"],
    },
}
FINISH = {
    "name": "postpilot_finish",
    "description": "Validate and return the structured terminal result after Naver readback, or a normalized pre-commit failure.",
    "parameters": {
        "type": "object",
        "properties": {
            "status": {"type": "string", "enum": ["published", "failed"]},
            "published_url": {"type": "string"},
            "failure_kind": {
                "type": "string",
                "enum": ["safe", "login_expired", "captcha", "two_factor", "account_mismatch", "editor_changed", "asset_missing"],
            },
            "detail": {"type": "string"},
        },
        "required": ["status"],
    },
}
