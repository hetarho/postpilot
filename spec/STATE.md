# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en

## ideation
| id | st |
|---|---|
| clip-source-observation-visibility | converted@260912 |
| clip-template-as-preset | converted@260917 |
| post-quality-and-related-links | converted@260923 |
| searchable-details | open@260926 |
| storyline-first | converted@260927 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 13 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 10 | 10 | - | 0 |
| QUOTA | 23 | 23 | - | 0 |
| POST | 22 | 22 | - | 0 |
| VOICE | 4 | 4 | - | 1 |
| GEN | 18 | 18 | - | 0 |
| MODEL | 19 | 19 | - | 0 |
| TMPL | 16 | 16 | - | 1 |
| GUIDE | 10 | 10 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 6 | 6 | - | 0 |
| THEME | 19 | 19 | - | 0 |
| MKT | 8 | 8 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 51 | 51 | - | 2 |
| CDS | 31 | 31 | - | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 4 | 4 | - | 2 |
| QUAL | 6 | 6 | - | 0 |
| GIFT | 2 | 2 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |
| arch-260919 | converted@260919 |
| publishing-260922 | converted@260922 |
| published-quality-260924 | converted@260925 |
| clip-narrate-failure-260926 | converted@260926 |
| conformance-all-260927 | converted@260927 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T451 | Synchronize project slots with the current clip plan | CLIP CDS ARCH | T450 | todo |
| T452 | Draft region slots in the approved storyline calls | CLIP CDS ARCH | T451 | todo |
| T453 | Edit intro and outro slots in the storyline space | CLIP CDS ARCH | T452 | todo |
| T454 | Validate manual caption styles independently of the AI set | CLIP CDS ARCH | - | todo |
| T455 | Select and style individual captions in the draft preview | CLIP CDS ARCH | T454 | todo |
| T456 | Verify region and caption edits across preview and both renders | CLIP CDS ARCH | T453 T455 | todo |

## next
- next session: implement-task T451 (synchronize project regions with the current plan); T451–T456 remain unstarted at the owner's request
- owner review of /IMPLEMENTATION-DECISIONS.md (the ambiguous calls of T414–T448), then review-code the clip wave
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); the /about header overflow at 320px/200% text still wants a task (MKT THEME); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26)

## log
- 260928 T450 done: project region persistence/API and compatibility verified with all local gates; commit/push requested, stop here and leave T451–T456 unstarted
- 260928 T450 verification: ff900701 fixes recovery writes rejected after wall-clock rollback; the deterministic regression and repeated recovery tests pass
- 260928 finding (T450 verification): attempt-checkpoint read/write recency also uses wall-clock ordering; retained as a follow-up outside the region-state task
- 260928 implementation scope narrowed: finish and commit T450 only; leave T451–T456 unstarted for the next session
- 260928 T450 claimed (cr); sequential T450–T456 implementation and per-task commits
- 260928 create-task CLIP r51 CDS r31 → T450–T456; lint/check passed (42 warnings, 13 review hints); 44 changed decisions covered and dependency graph verified
- 260928 CLIP task cursor reconciled through r50: completed T441–T448 and recorded r50 no-op; r44 no-op (open CLIP-163 creates no code work; GPU acceptance remains undecided)
- 260928 create-task CLIP CDS start (project region slots and caption appearance)
- 260928 update-ssot CLIP r51 CDS r31: project regions and storyline slots; manual caption styling across the approved set; no active task/worker affected
- 260928 update-ssot CLIP CDS start (project-owned intro/outro slots and per-caption preview styling)
- 260928 fix: a browser render's caption frame runs are also cut to half the preview deadline left, costed from two frames drawn alone (heavy styles timed out as CLIP_PREVIEW_TIMEOUT at 5 s)
- 260928 fix: a browser render's caption frame sheets are cut to what one JSON response carries (a neon or ember caption's run was refused as CLIP_PREVIEW_TOO_LARGE)
- 260928 fix done: regenerated backend/Dockerfile's media asset digest for the T416 caption preset; pnpm test:dev passes all 5 tests
- 260928 fix start: refresh the stale media asset digest after the T416 caption preset change
- 260928 fix: a finalized clip with a storyline opens again (GetClipProject failed the whole read when the storyline's source listing hit the revoked originals)
- 260928 fix: ①'s style and preset samples retry a CLIP_PREVIEW_BUSY refusal (fired together, the owner's preview lock refused the second, and the tiles stayed black)
- 260928 T416 done: a caption character its face does not draw is set in Wanted Sans Variable inside its own style
- 260928 finding (T416): backend/examples/overlays/shorts-editorial no longer loads (card-v1 views, no keynote/film/region/info bindings); only its caption template was brought to the runs contract
- 260928 T416 claimed (gs)
- 260928 create-task CDS r30 → T416 revised (unblocked); AUTH r10 no-op (T427 throttles VerifyEmail); THEME r16..r19 no-op (Wanted Sans Variable already first in --font-sans, the renderer's pinned file; r17..r19 consumed by T356 T412 T427)
