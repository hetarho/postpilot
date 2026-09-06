# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 1 | 1 | - | 0 |
| AUTH | 1 | 1 | - | 0 |
| QUOTA | 2 | 2 | - | 0 |
| POST | 2 | 2 | - | 0 |
| VOICE | 1 | 1 | - | 1 |
| GEN | 2 | 2 | - | 0 |
| MODEL | 3 | 3 | - | 0 |
| TEMPLATE | 2 | 2 | - | 1 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUBLISH | 2 | 2 | - | 0 |
| LANG | 1 | 1 | - | 0 |
| THEME | 2 | 2 | - | 0 |
| MARKETING | 1 | 1 | - | 0 |
| VIDEO | 1 | 1 | - | 1 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T001 | Price credit holds from each call's own completion budget | QUOTA GEN | - | done@260905 |
| T002 | Give the write budget reasoning headroom and let the adapter disable reasoning | MODEL GEN QUOTA | T001 | done@260905 |
| T003 | Persist the dedicated browser session across pairing retries | PUBLISH | - | done@260905 |
| T004 | Mac pairing, profile setup and the non-publishing compatibility probe | PUBLISH | T003 | done@260905 |
| T005 | The deterministic Naver publisher | PUBLISH EXPORT | T004 | done@260905 |
| T006 | Agent execution, the commit fence, readback and cleanup | PUBLISH | T005 | done@260905 |
| T007 | Agent automated test suite and LaunchAgent packaging | PUBLISH | T006 | doing@260905.cx |
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUBLISH MARKETING | T007 | todo |
| T009 | Pointer affordances for clickable controls | THEME | - | done@260905 |
| T010 | Photo row count in the grammar and ordered photo expansion | TEMPLATE | - | todo |
| T011 | Plain-language block names, fixed text instead of place/link, and the photo row stepper | TEMPLATE | T010 | todo |
| T012 | Video attachments: proto contract, migration and the post context | VIDEO POST GEN MODEL | - | todo |
| T013 | `video_input` capability, the video content part and the publish refusal | VIDEO MODEL PUBLISH | T012 | todo |
| T014 | Observe videos, write from them and validate VIDEO blocks | VIDEO GEN MODEL | T012 T013 T010 | todo |
| T015 | Video upload in the browser, the attachment strip and the contact sheet | VIDEO POST GEN | T012 | todo |
| T016 | The VIDEO block in the reading view, editor and exports, plus the capability badges and refusals | VIDEO EXPORT MODEL POST | T013 T015 | todo |

## next
- implement-task T007 · then T010 → T011 (TEMPLATE r2, ship both together — the shared grammar fixture changes in T010 and the FE catches up in T011)
- implement-task T012 → T013 → {T014 (after T010), T015} → T016 (video attachments; T012/T015 can run beside T010/T011 — only T014 shares prompts.go with T010)
- update-ssot PUBLISH after T008 closes: photo-row transport (TEMPLATE-39) and agent video publishing (VIDEO-17); GEN EXPORT for TEMPLATE-39 at the same time

## log
- 260906 create-task VIDEO POST GEN MODEL EXPORT → T012–T016; VIDEO tasked=1, POST GEN MODEL EXPORT tasked=rev; VIDEO-17 stays open (no task)
- 260906 create-task VIDEO POST GEN MODEL EXPORT start
- 260906 update-ssot done: VIDEO r1 (new domain, owner interview: placed in post, ≤1 min · 200 MB · 3/post) + POST r2 GEN r2 MODEL r3 EXPORT r2 hooks
- 260906 WARN VIDEO-16/17 change StartPublish (PUBLISH-9) without a PUBLISH rev — T007 doing, T008 todo; PUBLISH update-ssot after T008; LANG reason catalog needs MODEL_VIDEO_UNSUPPORTED VIDEO_NOT_PUBLISHABLE
- 260906 update-ssot pivot: scope became a new domain → VIDEO SSOT created instead of spreading it over POST/MODEL/GEN alone
- 260906 update-ssot POST MODEL GEN start (video attachments as observation material)
- 260906 create-task TEMPLATE → T010 T011; TEMPLATE tasked=2 (TEMPLATE-39 stays open, TEMPLATE-22/23 no code change: legacy pass kept)
- 260906 create-task TEMPLATE start
- 260906 update-ssot TEMPLATE r2 done (TEMPLATE-36..40; TEMPLATE-39 open on photo-row transport)
- 260906 WARN TEMPLATE-39 ripples into PUBLISH-21 (T007 doing, T008 todo) and GEN-1 / EXPORT-5; PUBLISH not bumped on purpose — decide after T008
- 260906 update-ssot TEMPLATE start
- 260905 T007 claimed (cx)
- 260905 T006 done (durable one-shot commit fence, exact same-target readback and terminal cleanup; ARCH-27 passes)
- 260905 T006 claimed (cx)
- 260905 T005 done (closed typed pre-fence publisher, exact semantic snapshots and ordinal JPEG verification; ARCH-27 passes)
- 260905 T005 claimed (cx)
- 260905 T004 done (versioned non-publishing Naver compatibility probe, safe pairing and existing-connection re-probe; ARCH-27 passes)
- 260905 T004 claimed (cx)
- 260905 T003 done (durable local drafts preserve isolated browser sessions across code replacement and restart; ARCH-27 passes)
- 260905 T003 claimed (cx)
