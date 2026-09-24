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
| T363 | A taken replacement candidate is spent | GEN POST ARCH | T355 T362 | todo |
| T365 | Generation preconditions take one named input | ARCH | - | todo |
| T366 | The draft queue's assignments are one record per channel | ARCH | T355 | todo |
| T367 | The autosave owns the published lock, and every control reads the lock from the post it holds | ARCH | T355 T365 T366 | todo |
| T368 | The editor page suites pin behavior, not wiring | ARCH | T355 T356 T363 T365 T367 | todo |
| T370 | Quality reads the post it needs and nothing more, and TopNoun is gone | ARCH | T357 T364 | todo |
| T372 | The test harness pins what it claims, and template limits have one constructor | ARCH | T354 T355 T359 T369 T371 | todo |
| T373 | Entity boundaries for 분야 and post status | ARCH | T356 T367 | todo |
| T375 | The editor's per-control cases live in the tests of the slices that own them | ARCH | T368 | todo |
| T381 | Generation resumes from prepared artifacts without repeating AI work | ARCH GEN CLIP | T376 T377 T379 T380 | todo |
| T382 | A worker render becomes the result through one durable API commit | ARCH CLIP | T381 | todo |
| T383 | Recovery bounds media retries and cleans cancelled or abandoned work | ARCH CLIP GEN | T381 T382 | todo |
| T384 | Clip progress distinguishes waiting, execution and media recovery | ARCH CLIP | T383 | todo |
| T385 | Local development and CPU images run API and worker separately | ARCH | T383 | todo |
| T386 | Deployment configuration keeps the CPU VPS default and supports a separate worker | ARCH CLIP | T383 T385 | todo |
| T387 | CPU separation passes local release checks with the VPS layout as default | ARCH CLIP GEN | T384 T386 | todo |
| T388 | GPU setup tooling and guides cover all three deployment environments | ARCH CLIP | T387 | todo |

## next
- implement-task T381 next for the media-worker wave (T376..T380 are done; T387 verifies CPU/VPS compatibility locally; T388 completes README/DEPLOY guides for all three environments and GPU setup tooling without live migration)
- Later: update-ssot CLIP-163, then create-task ARCH CLIP for real-GPU validation, profile approval/automatic selection and concurrency tuning; ARCH r11 and CLIP r44 remain partially consumed, and physical host setup/migration remain operator actions
- implement-task T355 (continuing T354..T375 in dependency order, p42); create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta

## log
- 260925 T371 done; five two-place values are each pinned to one source or one test: the URL length, the slot-token grammar (template owns it), the search page size (sent as display), the composition count and the replacement instruction's caps; BE and FE gates pass (p42)
- 260925 T371 claimed (p42)
- 260925 T380 done (mw): standalone CPU execution, bounded transfers/leases/drain and measured runtime profiles; byte/frame parity including sequence captions, Linux reaping, health and isolated CI pass; T381 next
- 260925 T380 claimed (mw); T379 committed as fa6d9517; standalone CPU executor only
- 260925 T374 done; an ask conflict a 블록 composition raised is lowered when it unmounts, so 원문 saves a source that parses; FE gates pass (p42)
- 260925 T374 claimed (p42)
- 260925 T369 done; an empty search answer keeps a field's stored phrase list and retries after the delay (QUAL-46), and a refresh interval below the quality context's 1h floor fails the boot naming its key; BE gate passes (p42)
- 260925 T369 claimed (p42)
- 260925 T364 done; MeasureSelf's answer is pinned per MeasureVersion by a golden digest over a 20-post corpus, and emoji tails (variation selectors, ZWJ, format runes, keycaps) trim at 어절 edges under MeasureVersion 2; BE gate passes (p42)
- 260925 T364 claimed (p42)
- 260925 T362 done; one replacement-placement fixture and one tag-identity fixture are run by the Go and TS suites alike, and the browser's whitespace is now Go's unicode.IsSpace set; BE and FE gates pass (p42)
- 260925 T362 claimed (p42)
- 260925 T361 done; a write comparison freezes the material Start freezes (memories included), the preset line rides only with frozen phrases, and the snapshot has its own tagged wire struct with every stored byte unchanged; BE gate passes (p42)
- 260925 T361 claimed (p42)
- 260925 T379 done (mw): private artifact access, immutable conditional uploads, verified receipts and durable deletion intent; real MinIO plus SQLite/HTTP/race and isolated full CI/codegen checks pass; T380 next
- 260925 T379 claimed (mw); T378 committed as 0d43a284; private artifact handoff only
- 260925 T360 done; ticked quality rules open the per-post half with the stable prefix unmoved, and the revise title form binds only a title request; BE gate passes (one parallel-session test aside) (p42)
- 260925 T360 claimed (p42)
- 260925 T359 done; the write prompt builder takes one WritePromptInput and template Create one Authored; prompts byte-identical, BE gate passes (p42)
- 260925 T359 claimed (p42)
