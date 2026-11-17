# Provider preference

For each user-invoked Gusta run, prefer the invoking conversation's provider for implementation and the other allowed provider for independent review. Keep both Codex and Claude allowed for usage-limit fallback.

Before dispatch, put the invoking provider first in SETTINGS `agents` through `vibe settings --set`; prefer it when planning task `run:` recommendations. Leave provider-pinning flags unset so cross-provider fallback remains available. Existing explicit task `run:` recommendations take precedence in the CLI; report actual routing when that prevents the calling-provider preference instead of claiming automatic caller detection.

## UI reuse (hard rule)

UI that does the same job is ONE component, used everywhere that job appears. Never build a second, smaller version of a surface the product already has.

- Before writing any UI, find the surface that already does this job — picking and uploading photos, showing a result, a progress list, a confirm, a model choice — in `frontend/src/{shared/ui,features,widgets}` and on the screen the owner already knows (the writing editor first). Use it.
- If the existing one is bound to one context (a post, a slug, one page), extract its presentational part into the owning slice (`features/<slice>/ui`, `widgets/`, or a domain-free `shared/ui` primitive under THEME-10/ARCH-20) and bind every caller to it in the same change. Each caller keeps only its own data binding.
- A cut-down substitute is a bug, not a simplification: a count line ("N장 준비됨", "2장 올라감") in place of the photo strip, a status word ("관찰 완료") in place of the result it stands for, a second picker beside the existing one. A surface may leave out actions it cannot support (pass no handler); it never re-implements the ones it keeps.
- New behaviour goes into the shared component as an optional prop, never into a copy.
- Reviews refuse a change that adds a parallel component for a job an existing one already does.
- Reference: the photo observation test's photos step is `PhotoAttachments` (`features/upload-photos`), the writing editor's own photo slot; a test's contestant list reads results with the review's renderers (`WritingTestObservationViewer`, `WritingTestPost`).

<!-- gusta:begin managed by gusta: do not edit inside this block (gusta-update rewrites it with "gusta agent-block"); write project rules outside it -->
## GUSTA

This project is built spec-first with GUSTA. `spec/` is the source of truth and code follows it.

- Implement only on a child branch; `gusta-start`, `gusta-self-improve` and `gusta-self-improve-remote` require a dedicated worktree. Plan with `gusta-sketch`.
- Do not change behavior covered by a `spec/ssot/` decision without updating the spec first.
- Branch, push, deploy and version behavior comes from `spec/SETTINGS.md` (the `gusta-preference` skill); never assume branch names.

<!-- gusta:end -->
