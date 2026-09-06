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
| TEMPLATE | 3 | 3 | - | 1 |
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
| T012 | Video attachments: proto contract, migration and the post context | VIDEO POST GEN MODEL | - | done@260906 |
| T013 | `video_input` capability, the video content part and the publish refusal | VIDEO MODEL PUBLISH | T012 | todo |
| T014 | Observe videos, write from them and validate VIDEO blocks | VIDEO GEN MODEL | T012 T013 T010 | todo |
| T015 | Video upload in the browser, the attachment strip and the contact sheet | VIDEO POST GEN | T012 | todo |
| T016 | The VIDEO block in the reading view, editor and exports, plus the capability badges and refusals | VIDEO EXPORT MODEL POST | T013 T015 | todo |
| T017 | The `원문` source mode, paste import and the copyable format guide | TEMPLATE | T011 | todo |

## next
- implement-task T007 · then T010 → T011 → T017 (TEMPLATE r2+r3; T010/T011 ship together, T017 adds the `원문` mode on top of T011's screen)
- implement-task T013 → T014 (after T010) → T015 → T016 (T012 done; T014 BEFORE T015 — T012's WARN: generation still reads a video as a photo)
- update-ssot PUBLISH after T008 closes: photo-row transport (TEMPLATE-39) and agent video publishing (VIDEO-17); GEN EXPORT for TEMPLATE-39 at the same time

## log
- 260906 WARN T012 put videos inside generation.PostInput.Images; generation still batches that list as photos — T014 must land before T015 makes videos attachable
- 260906 T012 done (videos as the post's second attachment kind: proto, migration 0026, kind-aware handshake, cascade, sweep, VIDEO block; ARCH-26 + ARCH-28 pass)
- 260906 create-task TEMPLATE → T017 (dep T011; FE only, no server change); TEMPLATE tasked=3; T011 base → TEMPLATE@3 with a hand-off note
- 260906 create-task TEMPLATE start (r3 delta: 26✎ 30✎ 34✎ 41+ 42+)
- 260906 T012 claimed (vd)
- 260906 update-ssot TEMPLATE r3 done (TEMPLATE-26✎ 30✎ 34✎ 41+ 42+: `원문` mode back, paste import, copyable format guide; T010/T011 todo touch the same screen — no doing task affected)
- 260906 update-ssot TEMPLATE start (raw body source view + paste import, reversing TEMPLATE-26/34)
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
