# Provider preference

For each user-invoked Gusta run, prefer the invoking conversation's provider for implementation and the other allowed provider for independent review. Keep both Codex and Claude allowed for usage-limit fallback.

Before dispatch, put the invoking provider first in SETTINGS `agents` through `vibe settings --set`; prefer it when planning task `run:` recommendations. Leave provider-pinning flags unset so cross-provider fallback remains available. Existing explicit task `run:` recommendations take precedence in the CLI; report actual routing when that prevents the calling-provider preference instead of claiming automatic caller detection.

<!-- gusta:begin managed by gusta: do not edit inside this block (gusta-update rewrites it with "gusta agent-block"); write project rules outside it -->
## GUSTA

This project is built spec-first with GUSTA. `spec/` is the source of truth and code follows it.

- Plan a change with the `gusta-sketch` skill; run planned work or a one-line change request with the `gusta-start` skill.
- Do not change behavior covered by a `spec/ssot/` decision without updating the spec first.
- Branch, push, deploy and version behavior comes from `spec/SETTINGS.md` (the `gusta-preference` skill); never assume branch names.

<!-- gusta:end -->
