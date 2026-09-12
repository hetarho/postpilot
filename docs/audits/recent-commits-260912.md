# Recent commit documentation audit — 2026-09-12

Scope: every commit dated from 2026-09-10 00:00 KST through `b59a709`: 67 commits. The audit checks changed paths, task/SSOT traceability, task-result caveats and the behavioral changes of commits without a task record. This is documentation coverage, not a claim that every historical implementation or visual requirement is correct.

Reproduce the inventory with `git log --since="2026-09-10 00:00:00 +0900" b59a709 --format="%h %s"` and inspect each patch with `git show <sha>`. No provider call or owner media upload is used.

## Corrections applied

| Evidence | Missing or stale description | Resolution |
|---|---|---|
| `6930283`, `ai/timeline.go`, `design/verify.go` | A footage-bound result may be shorter than target above 15 s; the style-run rule cannot alternate when the template disallows an alternate. | CLIP r7 / CDS r5 describe already-shipped behavior. |
| `79b4aed`, `cmd/api/clip_credits.go`, `llm/execution.go` | The live endpoint rechecks had no task or durable implementation note. | CLIP-30 explicitly names quote, admission and completion rechecks; existing T111 qualification policy is retained. |
| `b59a709`, T118 | List statuses now refresh while a job is in flight. Renderer memory bounds and audio correction were already documented in RENDER.md and T118. | CLIP-41 names terminal-state refresh; no new audio or visual policy. |
| `2838120`, `3d2cc3c`, `49e15a2`, `50114dc`, T042/T043/T045/T046 | PUB still said editor mutations, the commit port and publisher wiring were unbuilt. | PUB r5 removes stale implementation status; live-owner QA remains in T008. |
| `ff6b3b3`, current config/adapter/catalog tests | MODEL still said reasoning headroom, disable and truncation counters were unbuilt; these implementations predate this range. | MODEL r10 removes inherited stale status without changing MODEL-46–48. |
| `394bf61`, `publish-job/model/types.test.ts` | The cancel-stage hotfix had no task note. | Recorded here: it restores PUB-13 by using semantic stage order; no policy delta. |

## Existing design-contract gaps retained for a separate design decision

These are not silently made compliant by the SVG refactor. The current renderer output is the regression baseline, not visual approval.

| Contract | Implementation evidence | Gap retained |
|---|---|---|
| CDS-17 / CLIP-13 | T107 result; `media/copy.go` and bundled font assets | The server ships Pretendard and Paperlogy, refuses unsupported glyphs and has no active Noto fallback. |
| CDS-2 / CDS-9 / CDS-28 | T107 result; `measureCard` and V1 | Card background may extend outside the safe area while padded text must remain inside; SSOT has no explicit card-background exemption. |
| CDS-44 / CDS-52 V3 | T107 result; `design/contrast.go` | Unplated contrast includes the outline over sampled footage; plated text uses certified token pairings rather than a per-frame sample. |
| CDS-28 / CDS-29 / CDS-47 | `media/cards.go` / card SVG goldens | Horizontal hook and end cards currently stack lines; the specified single-row/two-column arrangements are not implemented. |
| CDS-45 / CDS-52 | T107/T109 results; current card overlay chain and T117 overlap policy | Component ordering and card exclusion targets do not imply an additional delivery failure; actual ordering remains the existing baseline. |
| CDS-11 / CDS-53 | T110 | Real Naver overlay measurement and human release QA remain owner-blocked; no synthetic render proves them. |

## Complete inventory

| Commit | Change | Documentation coverage |
|---|---|---|
| `b59a709` | fix(clip): bound rendering resources and recover audio drift (T118) | T118; ARCH, CDS, CLIP |
| `5b7d668` | fix(clip): deliver videos despite caption overlap (T117) | T117; ARCH, CDS, CLIP |
| `c19a2e2` | feat(clip): walk the caption repair ladder before a check can fail the clip (T116) | T116; ARCH, CDS, CLIP |
| `0e87ed6` | feat(clip): make cut length a target the compiler aims at, never a refusal (T115) | T115; ARCH, CDS, CLIP |
| `79b4aed` | fix(clip): quotes, admission and the pre-completion recheck read the live endpoint document | No task; covered by CLIP-30 clarification above. |
| `284afd6` | feat(clip): pick the analysis model by its live eligibility (T112) | T112; ARCH, CLIP, LANG |
| `90d9faf` | feat(clip): qualify any registered observe model against its live OpenRouter route (T111) | T111, T112; ARCH, CLIP, LANG, MODEL, QUOTA, VIDEO |
| `6930283` | fix(clip): make a live clip compile again — chips from the design table, a run the template forces, footage-bound length | No task; CLIP-7 and CDS-40 omissions corrected above; chip vocabulary remains CDS-30. |
| `0f51475` | docs(spec): take cut length off the gate path, name the repair ladder (CDS r3) | T115, T116; ARCH, CDS, CLIP |
| `be27092` | docs(spec): delegate the cut range and the fade ratio to the preset (CDS r2) | CDS r2 proposal superseded by r3 in 0f51475; current rules retained. |
| `52167e1` | docs(spec): block T110 on the owner's Naver measurement | T110; ARCH, CDS, CLIP |
| `653a0e9` | feat(clip): let a long cut carry a description and then its number (T109) | T107, T109; ARCH, CDS, CLIP |
| `2f65965` | feat(clip): join cuts by scene and deliver every clip at -16 LUFS (T108) | T105, T106, T107, T108, T109, T114; ARCH, CDS, CLIP, LANG, POST, THEME, TMPL |
| `1bcf935` | feat(clip): let the clip surfaces speak the design system (T106) | T106; ARCH, CDS, CLIP, LANG, THEME |
| `e0c05bd` | feat(template): author the target length and tag count on the template screen (T114) | T114; ARCH, POST, TMPL |
| `a9306dd` | feat(clip): move every placement decision out of the model and into the CDS tables (T105) | T105; ARCH, CDS, CLIP |
| `b651d5b` | feat(template): seed a post's length and tag count from its template (T113) | T113, T114; ARCH, POST, TMPL |
| `ecce23f` | feat(clip): give every clip its disclosure badge, its facts and a category preset (T104) | T104; ARCH, CDS, CLIP |
| `c92e6fa` | feat(clip): draw the four CDS styles, their two motions and a verified manifest (T103) | T008, T103, T104, T105, T106, T107, T108, T109, T110, T111, T112; ARCH, CDS, CLIP, GEN, LANG, MODEL, PUB, QUOTA, THEME, TMPL, VIDEO, VOICE |
| `4d6417c` | feat(clip): pin the CDS constants and take the plan onto anchors and four styles (T102) | T102; ARCH, CDS, CLIP |
| `21d6b5b` | docs(spec): CLIP r4 and the T097–T101 closure | T097, T101; CLIP, THEME |
| `788c424` | feat(clip): match the video-template screens to the post-template ones (T101) | T101; CLIP |
| `d4c00cf` | feat(clip): give the clip directory the post list's shape (T100) | T100; CLIP, POST |
| `97c65db` | feat(clip): carry each project's latest job on the list answer (T099) | T099; CLIP |
| `1c6862d` | feat(clip): autosave the clip settings and drop the save button (T098) | T098; CLIP, POST, THEME |
| `ac96ffd` | feat(clip): give the clip workspace the post editor's shape (T097) | T097, T098, T099, T100, T101; CLIP, POST, THEME |
| `53621c0` | docs(clip): close T096 with original-video and rollout evidence | T096; ARCH, CLIP, QUOTA |
| `e850fa8` | fix(clip): compile grounded timelines from model timing hints (T096) | T096; ARCH, CLIP, QUOTA |
| `a224a5b` | fix(skill): stop recomend-models substituting its own free-price example | Recommendation skill documentation only; no runtime contract change. |
| `957a4e1` | docs(skill): teach recomend-models to grade what it recommends | Recommendation skill documentation only; no runtime contract change. |
| `6db5115` | test(clip): cover multi-source generation and classify rejections (T096) | T096; ARCH, CLIP, QUOTA |
| `4b16ed2` | feat(models): grade registrations on the 모델 관리 row (T094) | T094; MODEL |
| `226bca6` | feat(models): lead every picker with its grade, cheapest first (T095) | T095; LANG, MODEL |
| `1bc8871` | feat(models): let the paste document carry each id's grade (T093) | T093; MODEL |
| `bf0ed63` | feat(models): grade each registration 가성비→최고 (T092) | T092, T093, T094, T095; LANG, MODEL |
| `353736b` | docs(spec): record verified T091 rollout | T091; ARCH, CLIP, QUOTA |
| `2d484e8` | fix(clip): simplify structured output contracts (T091) | T091; ARCH, CLIP, QUOTA |
| `70ff41d` | docs(spec): close T090 now its push-scope hold is resolved | T090; ARCH, CLIP |
| `3164be1` | feat(guidelines): fold the 후보 queue away and review it in bulk (T089) | T088, T089; ARCH, GUIDE, THEME |
| `7327d93` | feat(guidelines): put authoring behind one docked 새 지침 sheet (T088) | T088; ARCH, GUIDE, THEME |
| `61fe2d2` | fix(clip): retain privacy-safe failure diagnostics (T090) | T090; ARCH, CLIP |
| `ca539e5` | feat(navigation): draw the group menu as sticky second-level chrome (T087) | T087; ARCH, CLIP, THEME |
| `6e81e3b` | docs(spec): two-level navigation and the 지침 screen rework | T088, T089; ARCH, GUIDE, THEME |
| `820eabc` | test(clip): verify bounded release and credit recovery (T086) | T086; ARCH, CLIP, QUOTA, VIDEO |
| `719ef54` | feat(clip): approve credit ceilings and retain active previews (T085) | T085; ARCH, CLIP, QUOTA |
| `6de15a2` | feat(clip): prepare bounded media before credit admission (T084) | T084; ARCH, CLIP, QUOTA |
| `8357849` | feat(clip): enforce bounded multimodal request pricing (T083) | T083, T084, T085, T086; ARCH, CLIP, QUOTA, VIDEO |
| `1bce4d8` | feat(clip): require approved credit ceilings (T082) | T082; ARCH, CLIP, QUOTA |
| `35cd848` | fix(credits): waive unbilled failed clip attempts (T081) | T081, T082, T083, T084, T085, T086; ARCH, CLIP, QUOTA, VIDEO |
| `f9a5aae` | feat(navigation): group writing and video destinations (T080) | T080; ARCH, CLIP, THEME |
| `a4a477c` | feat(clip): add a credit-free correction workspace (T079) | T079; ARCH, CLIP, LANG, THEME |
| `7720d67` | feat(clip): save corrections and rerender without credits (T078) | T078; ARCH, CLIP, QUOTA |
| `8eaaf74` | feat(clip): show guarded generation progress and results (T077) | T077; ARCH, CLIP, LANG, THEME |
| `394bf61` | fix(publishing): the cancel button read stage order off the enum numbers | No task; restores existing PUB-13; recorded above. |
| `15b3925` | feat(clip): run durable generation within reserved credits (T076) | T076, T077, T078; ARCH, CLIP, LANG, MODEL, QUOTA, THEME, VIDEO |
| `50114dc` | feat(agent): the daemon gets a real publisher, one browser per job | T046; PUB |
| `49e15a2` | feat(publish): the commit fence, and the settings stage PUB-13 has wanted since r4 | T045; PUB |
| `7cb65e7` | feat(clip): add timecoded analysis and validated AI edit plans (T075) | T075; ARCH, CLIP, MODEL, VIDEO |
| `aa0c6a4` | feat(clip): render deterministic captioned MP4 outputs (T074) | T074; ARCH, CLIP |
| `3d2cc3c` | feat(publish): photos land where the manifest puts them, and text keeps its last word | T043; PUB |
| `b0ac75e` | feat(clip): add bounded media probing and analysis proxies (T073) | T073; ARCH, CLIP |
| `7e62c19` | feat(clip): add setup and direct source upload UI (T072) | T072; ARCH, CLIP, THEME |
| `3e56236` | feat(clip): add transient source batches and retryable cleanup (T071) | T071; ARCH, CLIP |
| `2838120` | feat(publish): heading, quote and list, converted the way the live editor actually converts | T042; PUB |
| `1adad01` | feat(clip): add video-template management UI (T070) | T070; ARCH, CLIP, THEME |
| `ff6b3b3` | fix(spec): repair domain identifiers and complete verified T069 | Domain-ID migration and T069 closure; inherited PUB/MODEL stale statuses corrected above. |
| `6b1b771` | feat(clip): add owned project and video-template foundation (T069) | T069, T070, T071, T072, T073, T074, T075, T076, T077, T078, T079, T080; ARCH, CLIP, LANG, MODEL, QUOTA, THEME, VIDEO |

The refactor T119 adds asset loading and rendering seams under existing ARCH rules. It does not select a new appearance, suppress disclosure, add a product style/picker, activate a new font, or change timing/credit policy.
