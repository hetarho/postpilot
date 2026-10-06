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
| MODEL | 31 | 31 | - | 0 |
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
| INFRA | 1 | 0 | all | 1 |

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
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |

## next
- T586/T587/T605 speech catalog registration, common pricing and public cost references are complete. T539/T550 retain real voice/listening/export/device qualification and require MODEL r31 revalidation; ordinary production voice/narration readiness stays closed.
- update-ssot ARCH so ARCH-10 makes the production database PostgreSQL as INFRA-6 states (ARCH-10 still says SQLite), then create-task INFRA r1 (all) and ARCH (ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎); create-task POST r32 (POST-108+ implemented by 12d2f428; verification-only); create-task VOICE r8 (VOICE-32✎ implemented by 68ae9a79; verification-only); update-ssot VOICE-31 remains open.
- ops: lower TEMPLATE_PHOTO_ROW_MAX / VITE_TEMPLATE_PHOTO_ROW_MAX to 3 wherever production sets them explicitly (VPS .env, Cloudflare build vars).
- ideation searchable-details continues: per-post search inflow from the owner's Naver statistics screenshots.

## log
- 261006 T605 done: friendly voice roles, sourced expiring cost references and precise public-rate draft defaults; 3306 FE tests and all local checks passed
- 261006 T605 start: labelled creation/reading models, sourced public cost references and exact prefilled pricing drafts; MODEL r31 consumed
- 261006 update-ssot MODEL start: explain voice design versus script reading, show public cost references and prefill common account-price drafts
- 261006 T587 done: list administration and common pricing UI; 3292 FE tests, complete local CI and deterministic generation passed
- 261006 T586 done: unique server-owned registrations, common tariff snapshots and retained legacy bindings; all BE tests passed after sequential clip-store recheck
- 261006 T587 start: list registration, direct grade changes, optional synthesis settings and common tariff editor
- 261006 T586 start: server-owned speech bindings, unique catalog registrations and atomic common tariff snapshots
- 261006 create-task MODEL r30 done: T586 server registration/pricing then T587 list administration
- 261006 create-task MODEL start: implement r30 speech catalog registration and common verified pricing
- 261006 update-ssot MODEL r30: single ElevenLabs catalog registration, optional synthesis controls and common pricing; T539/T550 remain blocked on real qualification
- 261006 update-ssot MODEL start: simplify the single-supplier speech catalog to list registration and product-managed defaults/pricing
- 261005 T585 done: actionable speech setup reasons, truthful saved-list failure/recovery, 3289 FE tests and all local CI checks passed
- 261005 T585 start: distinguish missing speech setup from supplier failure and unknown saved-list state; development credential is absent
- 261005 speech catalog diagnosis start: inspect provider connection, catalog failures and saved-list state before correcting administration feedback
- 261005 T550 blocked: lifecycle/readiness implementation, full local CI, five CPU narration/boundary cases, ordinary parity and Chromium editing/export checks passed; real supplier/human qualification is deferred
- 261005 T550 start: durable private speech cleanup, finalization provenance and complete offline workflow/boundary qualification
- 261005 T549 done: independent bounded browser narration, exact durable speech fingerprints, 3282 FE tests, full local CI, CPU smokes and browser/server decoded parity passed
- 261005 T549 start: bounded natural-speed browser audio and frozen speech fingerprints for output verification
- 261005 T548 done: owned natural-speed worker narration, immutable output provenance, 3269 FE tests, full local CI and CPU-image audio/ordinary parity passed
- 261005 T548 start: owned immutable speech admission, versioned worker retrieval and independent natural-speed final mixing
