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
| THEME | 11 | 11 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 12 | 12 | - | 0 |
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
| T129 | Author video templates through source and composition controls | CLIP CDS ARCH | T128 | todo |
| T130 | Build bounded draft video preview and overlay preparation | CLIP CDS ARCH | T128 T133 | todo |
| T131 | Replace cut forms with a preview-led timeline editor | CLIP CDS ARCH | T129 T130 | todo |
| T132 | Verify clip composition quality and editing parity | CLIP CDS QUOTA THEME ARCH | T136 | todo |
| T134 | Cancel clip attempts and settle the unused reservation | CLIP QUOTA ARCH | T128 T133 | todo |
| T135 | Finalize a matching clip result and delete its originals | CLIP ARCH | T134 | todo |
| T136 | Show focused clip progress and explicit confirmation | CLIP QUOTA THEME POST ARCH | T131 T134 T135 | todo |

## next
- implement-task T129, then preview/editor T130 → T131 plus cancellation/finalization T134 → T135; integrate T136 and verify T132.
- T110 must refresh CLIP@12 / CDS@10 before resuming because mandatory cards, disclosure and fact QA changed; it remains owner-blocked: privately load a 9:16 clip with both cards in the Naver picker without publishing, capture the clip-tab overlays and supply the images for CDS-11 measurement. Remaining CDS interpretations are recorded in the results of T107, T108, T109 and T115 (contrast/card geometry, audio seams, exposure and frequency, footage-bound duration).
- T008 needs the owner present: refresh PUB@5 / ARCH@2, rerun postpilot-agent setup before installation for driver signature smarteditor-one-20260910-a6, and cancel or deliberately reuse the queued 20260905-test job; after completion, update PUB for VIDEO-17 and TMPL-39.

## log
- 260913 T133 done (clip); reusable private originals, durable 24-hour retention and source playback pass all local and 27 production release gates; commit then T129 authoring
- 260913 T133 verification (clip); complete FE/BE and build gates pass, finish signing-race and production lifecycle checks before commit
- 260913 T133 resumed (clip); confirmed policies remain consumed by T133–T136, continue the authorized implementation and commit sequence
- 260913 T133 claimed (clip); T128 committed as 828f8f3 with a clean worktree, implement private 24-hour original retention and reuse
- 260913 T128 done (clip); native composition rendering, bounded overlays and durable element errors pass local and production gates; 100-cut/200-phrase and 20-source/49-observation stress pass, commit then T133 retention
- 260913 T128 verification (clip); local ARCH gates pass, 100 cuts / 90 s / 200 rapid phrases pass at 366,342,144-byte peak with no OOM; production media, final-frame sampling and 20-source release gates remain running
- 260913 T128 resumed (clip); confirmed lifecycle policies remain fully covered by CLIP r12 / QUOTA r10 / THEME r11 and T133–T136; continue the authorized T125–T136 implementation goal
- 260913 T128 claimed (clip); T127 committed as a987444a with a clean worktree, implement template-owned rendering and activate the complete composition path
- 260913 T127 done (clip); single-call native writer, scoped scene/item evidence and conservative omission pass all local gates; commit then T128 renderer
- 260913 T127 resumed (clip); preserve completed T125/T126 and continue the single-call native writer and scoped-evidence checks
- 260913 T127 claimed (clip); T126 committed as 7fd2337 with a clean worktree, implement bounded scene/item-grounded native composition planning
- 260913 T126 done (clip); composition storage, frozen quotes, version-5 plan codec and legacy migration pass all local gates; 1,718 FE tests, true migration restart and deterministic codegen verified, commit then T127
- 260912 T126 resumed (clip); continue the active authorized T125–T136 implementation goal after verifying confirmed lifecycle policy coverage
- 260912 create-task audit done (check); CLIP r12 / QUOTA r10 / THEME r11 remain consumed by T133–T136 with T130–T132 integration, spec lint passes with 11 existing warnings
- 260912 create-task CLIP QUOTA THEME start (check); audit existing T133–T136 and integrated T130–T132 coverage without duplicating consumed changes
- 260912 update-ssot CLIP QUOTA THEME verified (check); current CLIP r12 / QUOTA r10 / THEME r11 already match the confirmed policies, no additional content revision
- 260912 update-ssot CLIP QUOTA THEME start (check); reconcile the confirmed retention, finalization and cancellation policies with existing specifications and tasks
- 260912 T126 resumed (clip); verified T125 commit and clean code baseline, implement owned composition persistence, conversion and frozen contracts
- 260912 T126 claimed (clip); T125 committed as 6e4e8ec, implement composition persistence and legacy migration next
- 260912 T125 done (clip); portable Go/TS composition grammar and resolution, 65 shared cases, 70 FE grammar tests and all local gates pass; commit authorized and T126 next
