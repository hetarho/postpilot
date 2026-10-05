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
| voice-tidy | converted@260929 |
| daily-credit-plans | converted@260929 |
| template-from-request | converted@261001 |
| familiar-video-editing-and-dubbing | converted@261004 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 15 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ | 0 |
| AUTH | 11 | 11 | - | 0 |
| QUOTA | 34 | 34 | - | 0 |
| POST | 32 | 31 | POST-108+ | 0 |
| VOICE | 8 | 7 | VOICE-32✎ | 0 |
| GEN | 23 | 23 | - | 0 |
| MODEL | 29 | 29 | - | 0 |
| TMPL | 21 | 21 | - | 0 |
| GUIDE | 13 | 13 | - | 0 |
| EXPORT | 10 | 10 | - | 0 |
| LANG | 7 | 7 | - | 0 |
| THEME | 22 | 22 | - | 0 |
| MKT | 9 | 9 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 55 | 55 | - | 2 |
| CDS | 32 | 32 | - | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 5 | 5 | - | 2 |
| QUAL | 7 | 7 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |

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
| perf-cost-261004 | converted@261005 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T549 | Mix the same narration into browser MP4 exports | ARCH DUB CLIP CDS | T548 | todo |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | todo |

## next
- implement-task T549–T550 sequentially with automated checks and task commits, then merge/push main; T539 real supplier evidence is deferred and ordinary production readiness stays closed.
- create-task ARCH (ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎); create-task POST r32 (POST-108+ implemented by 12d2f428; verification-only); create-task VOICE r8 (VOICE-32✎ implemented by 68ae9a79; verification-only); update-ssot VOICE-31 remains open.
- ops: lower TEMPLATE_PHOTO_ROW_MAX / VITE_TEMPLATE_PHOTO_ROW_MAX to 3 wherever production sets them explicitly (VPS .env, Cloudflare build vars).

## log
- 261005 T548 done: owned natural-speed worker narration, immutable output provenance, 3269 FE tests, full local CI and CPU-image audio/ordinary parity passed
- 261005 T548 start: owned immutable speech admission, versioned worker retrieval and independent natural-speed final mixing
- 261005 T547 done: bounded private narration preview, monotonic clock, seek/reuse/cleanup and independent gain; full local CI and eight browser fixture reviews passed
- 261005 T547 start: bounded private PCM and one monotonic output clock for narrated draft playback
- 261005 T546 done: explicit confirmed-voice/script editing, selective approval, protected retiming, recovery and private playback; full local CI and four browser reviews passed
- 261005 T546 start: confirmed voice selection, explicit script/speech editing and reviewed natural-speed retiming
- 261005 T545 done: bounded measured narrated generation, one mixed approval and retained partial speech; 3249 FE tests and full local CI passed
- 261005 T545 start: bounded script/speech/flow initial variant, one mixed approval and reusable measured checkpoints
- 261005 T544 done: bounded selective speech, private MP3 validation, durable no-retry recovery and publication races; full local CI passed; connect speech accounting projection in T546
- 261005 T544 start: bounded stale-segment speech quotes, durable publication guards and no paid retry
- 261005 T543 done: main-preview placement, protected wording-only refresh and overflow/pending notices; 3249 FE tests and full local CI passed
- 261005 T543 start: main-preview caption selection/placement and explicit owner-safe script wording refresh
- 261005 T542 done: direct trim/reorder/seek, overlap-safe split and cancel/keyboard/undo semantics; 3240 FE tests and full local CI passed
- 261005 T542 start: source-bound direct trim/reorder, shared output seek, cancellation and one history entry per gesture
- 261005 T541 done: preview-first contextual editor, three named tracks and explicit mobile details; 3231 FE tests, full local CI and 12 responsive/theme browser reviews passed
- 261005 T541 start: persistent desktop properties, explicit phone details, named tracks and preview-first saved drafts
- 261005 T540 done: independent spoken scripts, v7/legacy mapping, private project speech assets, selective staleness/reuse, protected caption links and publication CAS; full local CI passed
- 261005 T540 start: independent spoken-script contract, private speech provenance, revision guards and old-plan compatibility
- 261005 create-task DUB start: move real qualification after implementation, retain unfulfilled live evidence tasks and normal automated gates
- 261005 update-ssot DUB start: defer live supplier qualification until complete implementation; keep production readiness evidence mandatory
