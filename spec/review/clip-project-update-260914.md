# REVIEW clip-project-update-260914
> st:converted@260914 | scope:backend/internal/clip (project update admission), frontend/src/entities/clip-project | at:c02fa07 | base:ARCH@2

## summary
- one P1: a composition-template clip project cannot be edited at all, because update admission refuses the empty legacy disclosure that creation deliberately allows and that every composition project stores
- found from production, not from reading: an owner adding the menu item their template requires got `invalid_argument` / CLIP_INVALID_INPUT on every save, with the screen still showing the previous generation's failure

## findings
- F1 [o] P1 `backend/internal/clip/service.go:283` UpdateProject: bug: the disclosure check refuses the empty value that CreateProject at `:235` explicitly admits (`input.Disclosure != "" && !ValidDisclosure(...)`) ← a composition template owns its own disclosure field, so a project created against one stores the legacy enum as `""`; `validClipProject` exempts exactly that case (`types.ts:168`) while the save still sends the field on every call (`clip-project.ts:192`), so no owner edit of a composition project can ever be saved — title, target duration and composition inputs alike. Production proof: project `7cdeb67c` has `created_at == updated_at` after repeated edits, while `5dc66054`/`266c1ea5` carry `ad`/`provided` from the retired picker and save normally →T153

## notes
- not a finding, but the same call chain: the save failure never reaches the owner. The page keeps rendering the previous generation failure, so a refused write looks like a stale error message and is only visible in the network panel. Surfacing write failures belongs with the inspection-visibility work rather than with this fix
