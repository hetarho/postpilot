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

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 11 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 8 | 8 | - | 0 |
| QUOTA | 19 | 19 | - | 0 |
| POST | 14 | 14 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 12 | 12 | - | 0 |
| MODEL | 17 | 17 | - | 0 |
| TMPL | 11 | 11 | - | 1 |
| GUIDE | 6 | 6 | - | 0 |
| EXPORT | 5 | 5 | - | 0 |
| PUB | 6 | 6 | - | 0 |
| LANG | 5 | 5 | - | 0 |
| THEME | 18 | 15 | THEME-19✎ | 0 |
| MKT | 6 | 6 | - | 0 |
| VIDEO | 3 | 3 | - | 0 |
| CLIP | 44 | 40 | CLIP-13✎ CLIP-163+ | 2 |
| CDS | 25 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 3 | 3 | - | 2 |
| QUAL | 4 | 4 | - | 0 |

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

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T355 | The writing brief saves its run options together | POST MEM ARCH | - | todo |
| T356 | Tab moves through an anchored panel; a hover-opened panel closes on Escape | THEME ARCH | T355 | todo |
| T357 | The published lock is one guard, and an unclassified write fails a test | ARCH | T355 | todo |
| T358 | A photo's row goes before its object, and "finalized at the current revision" is one rule | ARCH | T357 | todo |
| T359 | The write prompt builder and template Create each take one named input | ARCH | T354 | todo |
| T360 | Ticked quality rules open the per-post half; in revise the title form binds only a title request | GEN TMPL ARCH | T359 | todo |
| T361 | A write comparison freezes the same write material as Start | MEM MODEL GEN GUIDE QUAL ARCH | T354 T359 | todo |
| T362 | One replacement-rule fixture both sides read | ARCH | - | todo |
| T363 | A taken replacement candidate is spent | GEN POST ARCH | T355 T362 | todo |
| T364 | A measurement change cannot ship without its version bump, and emoji tails trim | ARCH | - | todo |
| T365 | Generation preconditions take one named input | ARCH | - | todo |
| T366 | The draft queue's assignments are one record per channel | ARCH | T355 | todo |
| T367 | The autosave owns the published lock, and every control reads the lock from the post it holds | ARCH | T355 T365 T366 | todo |
| T368 | The editor page suites pin behavior, not wiring | ARCH | T355 T356 T363 T365 T367 | todo |
| T369 | An empty search answer keeps the stored list, and the refresh interval has a floor | QUAL ARCH | T364 | todo |
| T370 | Quality reads the post it needs and nothing more, and TopNoun is gone | ARCH | T357 T364 | todo |
| T371 | The small mirrors are pinned | ARCH | T362 T364 T369 | todo |
| T372 | The test harness pins what it claims, and template limits have one constructor | ARCH | T354 T355 T359 T369 T371 | todo |
| T373 | Entity boundaries for 분야 and post status | ARCH | T356 T367 | todo |
| T374 | The template screen's ask-conflict flags follow the mounted composition | ARCH | - | todo |
| T375 | The editor's per-control cases live in the tests of the slices that own them | ARCH | T368 | todo |
| T379 | Media bytes cross stages through authorized private artifacts | ARCH CLIP | T376 T378 | todo |
| T380 | A standalone CPU worker prepares and renders frozen media jobs | ARCH CLIP | T378 T379 | todo |
| T381 | Generation resumes from prepared artifacts without repeating AI work | ARCH GEN CLIP | T376 T377 T379 T380 | todo |
| T382 | A worker render becomes the result through one durable API commit | ARCH CLIP | T381 | todo |
| T383 | Recovery bounds media retries and cleans cancelled or abandoned work | ARCH CLIP GEN | T381 T382 | todo |
| T384 | Clip progress distinguishes waiting, execution and media recovery | ARCH CLIP | T383 | todo |
| T385 | Local development and CPU images run API and worker separately | ARCH | T383 | todo |
| T386 | Deployment configuration keeps the CPU VPS default and supports a separate worker | ARCH CLIP | T383 T385 | todo |
| T387 | CPU separation passes local release checks with the VPS layout as default | ARCH CLIP GEN | T384 T386 | todo |
| T388 | GPU setup tooling and guides cover all three deployment environments | ARCH CLIP | T387 | todo |

## next
- implement-task T379 next for the media-worker wave (T376..T378 are done; T387 verifies CPU/VPS compatibility locally; T388 completes README/DEPLOY guides for all three environments and GPU setup tooling without live migration)
- Later: update-ssot CLIP-163, then create-task ARCH CLIP for real-GPU validation, profile approval/automatic selection and concurrency tuning; ARCH r11 and CLIP r44 remain partially consumed, and physical host setup/migration remain operator actions
- implement-task T355 (continuing T354..T375 in dependency order, p42); create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta

## log
- 260925 T354 done; a generate's payload is encoded by Start and decoded by Generate, cmd/api carries opaque bytes; the native-effort flag now reaches the drain and each A/B candidate takes its own model's headroom; BE gate passes (p42)
- 260925 T354 claimed (p42)
- 260925 T378 done (mw): authenticated separate worker API, compatible claims, fenced reports, bounded transport and private runtime status; HTTP/SQLite/race, isolated ARCH-26/FE/CI/codegen checks pass; T379 next
- 260925 T378 claimed (mw); T377 committed as b56a62bc; sequential media-wave scope only
- 260925 create-task POST MEM done: T355 rewritten as the brief's whole-set 저장 (분야 joins SavePostGenerationOptions, a partial request is refused, 분야 and 기억 사용 leave ①'s panel, the draft queue's 분야 channel goes); T356 T357 T361 T363 T366 T367 T368 T372 T373 T375 updated to match, T366 now after T355, T373 also refuses an unknown quality tick
- 260925 create-task POST MEM start (r14, r3: 분야 and 기억 사용 in the brief, saved together by its 저장; updates todo T355 T356 T366 T367 T368 T373 T375)
- 260925 T377 done (mw): durable park/wake and replay policies, continued queue processing and fenced cancellation acknowledgement; SQLite/queue/race, ARCH-26, FE/CI/codegen and spec checks pass; T378 next
- 260925 T377 claimed (mw); T376 committed as 2554cef1; sequential media-wave scope only
- 260925 T376 done (mw): durable fenced media stages/attempts/artifact reservations and bounded env limits; SQLite concurrency/race/upgrade, ARCH-26, FE/CI/codegen and spec checks pass; production dispatch unchanged; commit per task, T377 next
- 260925 T376 claimed (mw); implementation scope T376..T388 only, sequential; other sessions own the remaining wave
- 260925 create-task media-worker scope correction done: T386 keeps CPU VPS deployment as default, T387 verifies both layouts locally, T388 delivers three-environment README/DEPLOY guides and CPU-verifiable GPU setup tooling; no live deployment, friend-PC installation, migration or hardware benchmark is required by this wave; T376..T388 remain 13 todo with ARCH@11 bases; references, dependencies, STATE/format and diff checks pass
- 260925 ARCH r11 ARCH-37/59 and delivery portion of ARCH-57 tasked in T386..T388; ARCH-52/53/56/57 GPU activation/tuning and CLIP-163 remain pending; POST/MEM and other unrelated tasks/deltas preserved
- 260925 ARCH r11 ready for create-task: ARCH-37/57 narrow hardware gates to future GPU activation; ARCH-59 makes the current CPU VPS default and requires deployment guides for all three environments; provisioning and live migration are later operator actions
- 260925 create-architecture ARCH / create-task media-worker scope correction start: keep the current CPU VPS deployment; deliver guides for CPU co-location, GPU co-location and a remote GPU worker; physical GPU setup and migration are later operator work
- 260925 create-task media-worker done: T376..T388 (13 todo), CPU leases/continuations/artifacts/executor/integration/lifecycle/UI/images/deployment/cutover plus an isolated NVIDIA benchmark; dependency, SSOT-reference and format checks pass
- 260925 GEN r11..r12 fully tasked: GEN-33/35 in T377 T381 T383 T387; r12 deltas already covered by T360 T361 T363; ARCH r10 partial pending ARCH-52/53/56/57 GPU activation/tuning until CLIP-163 is decided, ARCH-58 deferred with no code task; CLIP r44 partial pending CLIP-163 plus prior CLIP-13; existing T354..T375 preserved
- 260925 create-task ARCH GEN CLIP start (media-worker deployment delta only; preserve unrelated tasks and pending; GPU output acceptance remains CLIP-163)
- 260924 ARCH r10, GEN r11, CLIP r44 documented separate CPU/GPU media execution, private stage artifacts and bounded recovery; CLIP-163 output equivalence remains open; prior pending deltas preserved, no tasks changed
- 260925 update-ssot done: POST r14 MEM r3 — 분야 and 기억 사용 move from ①'s panel into the writing brief (POST-51✎ POST-54✎ POST-71✎ POST-82✎ MEM-18✎ MEM-26✎), and the brief's run options (목표 분량, 태그 수, quality ticks, 분야, 기억 사용) save together by its 저장 (POST-81✎ POST-89+); no doing task affected, todo T355 T366 T367 T368 T373 T375 assume the old placement
- 260925 update-ssot POST MEM start: 분야 and 기억 사용 move into the writing brief
