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

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 2 | 2 | - | 0 |
| AUTH | 5 | 5 | - | 0 |
| QUOTA | 10 | 10 | - | 0 |
| POST | 6 | 6 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 6 | 6 | - | 0 |
| MODEL | 10 | 10 | - | 0 |
| TMPL | 6 | 6 | - | 1 |
| GUIDE | 2 | 2 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 5 | 5 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 12 | 12 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 13 | 13 | - | 0 |
| CDS | 10 | 10 | - | 2 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T110 | Release QA on the Naver app and the overlay measurement | CDS CLIP | T108 | blocked@260912 |

## next
- T110 must refresh CLIP@13 / CDS@10 before resuming because mandatory cards, disclosure and fact QA changed; it remains owner-blocked: privately load a 9:16 clip with both cards in the Naver picker without publishing, capture the clip-tab overlays and supply the images for CDS-11 measurement. Remaining CDS interpretations are recorded in the results of T107, T108, T109 and T115 (contrast/card geometry, audio seams, exposure and frequency, footage-bound duration).
- T008 needs the owner present: refresh PUB@5 / ARCH@2, rerun postpilot-agent setup before installation for driver signature smarteditor-one-20260910-a6, and cancel or deliberately reuse the queued 20260905-test job; after completion, update PUB for VIDEO-17 and TMPL-39.

## log
- 260913 T137 done (grd); original playback and one cancellation confirmation pass 1,809 frontend tests, local gates and 40 browser surfaces including readable 200-percent text
- 260913 T137 verification (grd); visual review found unreadable confirmation labels at 200% text despite geometry checks, restore doing and adapt the narrow footer
- 260913 T137 claimed (grd); implement original preview and cancellation confirmation
- 260913 create-task CLIP THEME done (guard); T137 consumes CLIP r13 and THEME r12
- 260913 update-ssot CLIP r13 THEME r12 done (guard); retained source playback and one cancellation confirmation, no billing formula change
- 260913 create-task CLIP THEME start (guard)
- 260913 update-ssot CLIP THEME start (guard); allow source playback during production and require one explicit cancellation confirmation
- 260913 T132 done (clip); composition/lifecycle quality, retained preview recovery and keyboard layout pass all local, production-media and resource gates; T125–T136 implemented and ready for final commit
- 260913 T132 verification (clip); offline composition and lifecycle gates pass; fix reopened-source preview hydration and pin only the frame above keyboard/action controls, finish local gates
- 260913 T132 claimed (clip); T136 committed as 79141fa with a clean worktree, run offline semantic/frame/lifecycle integration and archive review evidence
- 260913 T136 done (clip); explicit confirmation, focused cancellation and retained-original retries pass local gates and 48 browser cases; commit then T132 integration
- 260913 T136 claimed (clip); T135 committed as fd9364c with a clean worktree, connect save-flushed confirmation and focused cancellation progress
- 260913 T135 done (clip); explicit matching-result confirmation, irreversible source cleanup and result download pass local and production checks; commit then T136 UI
- 260913 T135 claimed (clip); T134 committed as 1362710 with a clean worktree, implement explicit finalization and durable original deletion
- 260913 T134 done (clip); durable cancellation, atomic result completion and reservation settlement pass local, production-media and 27 release gates; commit then T135 finalization
- 260913 T134 claimed (clip); T131 committed as 321a1be with a clean worktree, implement durable cancellation and reservation-based settlement
- 260913 T131 done (clip); timeline editing, exact phrase windows, autosave/undo/conflicts and 24 mobile browser combinations pass local and production-media gates; commit then T134 cancellation
- 260913 T131 claimed (clip); T130 committed as f681f3a with a clean worktree, implement preview-led timeline editing and autosave
- 260913 T130 done (clip); bounded current-draft playback and server glyph preparation pass local, production-media and browser gates; commit then T131 timeline
- 260913 T130 verification (clip); bounded source-free preview passes real-font and ko/en browser checks, complete local gates before commit
