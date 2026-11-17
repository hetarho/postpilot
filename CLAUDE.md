# postpilot

@AGENTS.md

AGENTS.md (imported above) holds every agent's project instructions: provider preference, the UI reuse rule and GUSTA. The rule below is repeated here because it is the one most often broken.

## UI reuse (hard rule)

UI that does the same job is ONE component, used everywhere that job appears. Never build a second, smaller version of a surface the product already has.

- Before writing any UI, find the surface that already does this job — picking and uploading photos, showing a result, a progress list, a confirm, a model choice — in `frontend/src/{shared/ui,features,widgets}` and on the screen the owner already knows (the writing editor first). Use it.
- If the existing one is bound to one context (a post, a slug, one page), extract its presentational part into the owning slice (`features/<slice>/ui`, `widgets/`, or a domain-free `shared/ui` primitive under THEME-10/ARCH-20) and bind every caller to it in the same change. Each caller keeps only its own data binding.
- A cut-down substitute is a bug, not a simplification: a count line ("N장 준비됨", "2장 올라감") in place of the photo strip, a status word ("관찰 완료") in place of the result it stands for, a second picker beside the existing one. A surface may leave out actions it cannot support (pass no handler); it never re-implements the ones it keeps.
- New behaviour goes into the shared component as an optional prop, never into a copy.
- Reviews refuse a change that adds a parallel component for a job an existing one already does.
- Reference: the photo observation test's photos step is `PhotoAttachments` (`features/upload-photos`), the writing editor's own photo slot; a test's contestant list reads results with the review's renderers (`WritingTestObservationViewer`, `WritingTestPost`).
