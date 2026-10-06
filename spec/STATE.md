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
| ARCH | 16 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ ARCH-11✎ ARCH-45✎ ARCH-60+ ARCH-61+ ARCH-62+ ARCH-63+ ARCH-64+ ARCH-65+ ARCH-66+ ARCH-67+ ARCH-68+ | 0 |
| AUTH | 11 | 11 | - | 0 |
| QUOTA | 35 | 35 | - | 0 |
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
| CLIP | 56 | 56 | - | 2 |
| CDS | 33 | 33 | - | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 5 | 5 | - | 2 |
| QUAL | 7 | 7 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |
| INFRA | 2 | 0 | all | 1 |

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
| T590 | Bound Max server-render admission and waiting | ARCH CLIP INFRA | T589 | blocked@261006 |
| T591 | Freeze one browser composition and time contract | ARCH CLIP CDS | T588 | todo |
| T592 | Decode selected video ranges in a bounded browser pipeline | ARCH CLIP CDS | T591 | todo |
| T593 | Bound selected audio and immutable narration processing | ARCH CLIP CDS DUB | T591 | todo |
| T594 | Draw bundled typography and static components locally | ARCH CLIP CDS | T591 | todo |
| T595 | Animate caption transforms and masks from output time | ARCH CLIP CDS | T594 | todo |
| T596 | Port caption blur, light, colour and glitch effects | ARCH CLIP CDS | T595 | todo |
| T597 | Render ember caption geometry and particles locally | ARCH CLIP CDS | T596 | todo |
| T598 | Measure caption backgrounds from local original frames | ARCH CLIP CDS | T592 T594 | todo |
| T599 | Stream browser output and promote the verified private result | ARCH CLIP CDS | T592 T593 T595 T596 T597 T598 | todo |
| T600 | Use the browser composition engine throughout editing previews | ARCH CLIP CDS | T592 T593 T594 T595 T596 T597 T598 | todo |
| T601 | Prepare bounded AI analysis copies in the browser | ARCH CLIP CDS | T592 T602 | todo |
| T602 | Authorize and verify browser-prepared analysis artifacts | ARCH CLIP QUOTA | T591 | todo |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | todo |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | todo |

## next
- T590 implementation and all available local CI checks pass; blocked on the unresponsive Docker daemon. Restore Docker, run the Docker generator checks and matching CPU smokes, then close T590. T591 is the next todo; T539/T550 keep voice/listening and browser qualification gates.
- ARCH r16 and INFRA r2 browser changes are mapped to T588-T604; earlier ARCH revisions and unrelated INFRA database/backups remain unconsumed, so ARCH tasked=9 and INFRA tasked=0 stay. ARCH-10/INFRA-6 database alignment, POST r32 and VOICE r8 verification-only deltas and VOICE-31 remain outside this scope.
- ops: lower TEMPLATE_PHOTO_ROW_MAX / VITE_TEMPLATE_PHOTO_ROW_MAX to 3 in explicit production overrides. Ideation searchable-details continues; ordinary voice/narration readiness stays closed until qualification passes. Doc-review ARCH QUOTA VOICE handles existing document-quality hints.

## log
- 261006 parallel browser-media start: isolated task workers through T604 with dependency-aware dispatch and independent reviews; preserve T590 implementation and its Docker gate
- 261006 T590 blocked: durable 1/2/1 native limits, atomic reservations and queue expiry verified; full BE tests, race tests, 3323 FE tests and 62 deploy tests passed; Docker generators/image smokes await daemon recovery
- 261006 T590 verification scope: installed locked dependencies restored; ignored backend/tmp diagnostic programs excluded from an identical-source CI copy; no production deployment or database changes
- 261006 T590 start: durable finite Max render capacity, cancellable bounded waiting and same-host resource reservations
- 261006 T589 done: current Max-only native access, preserved legacy jobs/history and browser-first ko/en surfaces; 3321 FE tests and full local CI passed
- 261006 user stop boundary: finish T589 and commit current browser-media work; T590-T604 remain todo
- 261006 T589 start: Max-only commercial server admission and browser-first controls, preserving previously accepted work and usage history
- 261006 T588 done: measured browser baseline, common output verification and version/license evidence; 3307 FE tests and local CI passed, SDK timeline and real-device gates remain unqualified
- 261006 T605 done: friendly voice roles, sourced expiring cost references and precise public-rate draft defaults; 3306 FE tests and all local checks passed
- 261006 T605 start: labelled creation/reading models, sourced public cost references and exact prefilled pricing drafts; MODEL r31 consumed
- 261006 update-ssot MODEL start: explain voice design versus script reading, show public cost references and prefill common account-price drafts
- 261006 T588 start: reproducible browser media phase/resource baselines and qualified dependency/license inventory
- 261006 create-task browser media done: T588-T604; CLIP r56 CDS r33 QUOTA r35 consumed, scoped ARCH/INFRA mappings preserve earlier pending work
- 261006 T586/T587 freshness: ARCH r16 browser execution and QUOTA r35 commercial-export changes do not affect speech catalog administration; task bases synchronized
- 261006 T587 done: list administration and common pricing UI; 3292 FE tests, complete local CI and deterministic generation passed
- 261006 T586 done: unique server-owned registrations, common tariff snapshots and retained legacy bindings; all BE tests passed after sequential clip-store recheck
- 261006 create-task browser media start: consume CLIP r56 CDS r33 QUOTA r35; map scoped ARCH r16 and INFRA r2 changes without consuming older infrastructure work
- 261006 update-ssot ARCH r16 CLIP r56 CDS r33 QUOTA r35 INFRA r2: browser media contracts, qualified motion/proxies and bounded Max-only exports
- 261006 planning impact: T539/T550 require browser-contract revalidation; T586/T587 retain unchanged speech decisions but must check advanced ARCH/QUOTA bases; active task files are not edited
- 261006 update-ssot CLIP CDS QUOTA and create-architecture start: browser-first media execution, motion parity and analysis-quality qualification
