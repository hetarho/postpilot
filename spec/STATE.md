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
| T368 | The editor page suites pin behavior, not wiring | ARCH | T355 T356 T363 T365 T367 | todo |
| T370 | Quality reads the post it needs and nothing more, and TopNoun is gone | ARCH | T357 T364 | todo |
| T372 | The test harness pins what it claims, and template limits have one constructor | ARCH | T354 T355 T359 T369 T371 | todo |
| T373 | Entity boundaries for 분야 and post status | ARCH | T356 T367 | todo |
| T375 | The editor's per-control cases live in the tests of the slices that own them | ARCH | T368 | todo |

## next
- The media-worker wave T376..T388 is complete; CPU separation, both layouts and three-environment guides/isolated NVIDIA diagnostics are committed per task; no live migration
- Later: update-ssot CLIP-163, then create-task ARCH CLIP for real-GPU validation, profile approval/automatic selection and concurrency tuning; ARCH r11 and CLIP r44 remain partially consumed, and physical host setup/migration remain operator actions
- implement-task T368 (continuing T354..T375 in dependency order, p42); create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta

## log
- 260925 T367 done; the autosave decides the published lock and owns ①'s text (a locked refusal takes the text back to the screen); DraftEditor has no lock masking and every editor component reads isPublished(post) itself; FE gates pass (p42)
- 260925 T367 claimed (p42)
- 260925 T366 done; the draft queue holds its voice, 템플릿 and target-language assignments as one record each, walked by one channel list; SendDraft takes one request and the handle one assign(channel, value); FE gates pass (p42)
- 260925 T366 claimed (p42)
- 260925 T365 done; the generation gates take one input whose members are all required; /ai-models/compare now refuses a published post with the lock's sentence; FE gates pass (p42)
- 260925 T365 claimed (p42)
- 260925 T363 done; a take spends its candidate in the same content save (taken_candidates indices resolved at send time), so its mark never returns even where the phrase holds its source; BE and FE gates pass (p42)
- 260925 T363 claimed (p42)
- 260925 T358 done; photo and video deletes remove the guarded row before the object, a failed object delete left to the sweep; one FinalizedAtCurrentRevision rule, applied by the service to the row the snapshot read; BE gate passes (p42)
- 260925 T358 claimed (p42)
- 260925 T357 done; one writablePost guard for the published lock; SQL and Service default-deny tests make an unclassified write statement or exported method fail; photo and video deletes search posts by key; BE gate passes (p42)
- 260925 T357 claimed (p42)
- 260925 T356 done; InlinePopover walks its panel with Tab and leaves past either end (THEME-42); a hover-opened panel or tip closes alone on Escape; one focusable selector, one anchored-panel hook, one quality values/share helper; FE gates pass (p42)
- 260925 T356 claimed (p42)
- 260925 T355 done; the brief's five run options are one form saved by its 저장 as one whole-set SavePostGenerationOptions (field included); ① and /posts/new carry neither 분야 nor 기억 사용 and the draft queue's 분야 channel is gone; BE and FE gates pass (p42)
- 260925 T355 claimed (p42)
- 260925 T388 done (mw): three-environment guides, pinned NVIDIA candidate, explicit overrides/rollback and isolated diagnostics; CPU image/fixture parity, benchmark, 37 deploy tests and CI gates pass; real GPU activation remains unverified and pending
- 260925 T388 claimed (mw); T387 committed as ac3953ae; three-environment guides, NVIDIA candidate packaging and isolated diagnostics only; production GPU activation remains gated
- 260925 T387 done (mw): CPU worker-only production dispatch, separate-process colocated/remote release and capacity preflight; parity, restart/reclaim/cancel/cleanup, API image, 28 release cases and all CI gates pass; T388 next
- 260925 T387 claimed (mw); T386 committed as 4de3ce80; separate-process CPU release fixtures, capacity preflight and production worker-only dispatch
