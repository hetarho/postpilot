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
| QUOTA | 33 | 33 | - | 0 |
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
| BILL | 8 | 8 | - | 0 |
| MEM | 5 | 5 | - | 2 |
| QUAL | 7 | 7 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 2 | 2 | - | 0 |

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
| perf-cost-261004 | converted@261004 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T533 | Add a bounded voice-design and speech provider boundary | ARCH MODEL DUB | - | doing@261004.du |
| T534 | Curate explicit speech profiles and their qualified pricing | ARCH MODEL DUB QUOTA THEME | T533 | todo |
| T535 | Meter speech units under explicit bounded credit approvals | ARCH QUOTA DUB MODEL | T534 | todo |
| T536 | Persist private reusable spoken voices and audition assets | ARCH DUB MODEL | T535 | todo |
| T537 | Generate voice candidates and confirm the auditioned identity | ARCH DUB MODEL QUOTA | T536 | todo |
| T538 | Build voice creation, audition, confirmation and account reuse screens | ARCH DUB THEME | T537 | todo |
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | todo |
| T540 | Add independent spoken-script and speech provenance to clip plans | ARCH DUB CLIP | T539 | todo |
| T541 | Make preview and contextual editing the clip workspace entry | ARCH CLIP THEME | T540 | todo |
| T542 | Implement direct trim, reorder, seek and playhead split | ARCH CLIP CDS THEME | T541 | todo |
| T543 | Edit captions directly and refresh script-derived wording safely | ARCH DUB CLIP CDS THEME | T541 | todo |
| T544 | Generate only stale clip speech with immutable revision guards | ARCH DUB CLIP QUOTA | T540 | todo |
| T545 | Assemble narrated first drafts from measured speech timing | ARCH DUB CLIP QUOTA CDS | T544 | todo |
| T546 | Expose voice selection, script edits and explicit timing conflict choices | ARCH DUB CLIP QUOTA THEME | T541 T543 T545 | todo |
| T547 | Play synchronized narration in the editable draft preview | ARCH DUB CLIP CDS | T546 | todo |
| T548 | Mix immutable narration into server MP4 exports | ARCH DUB CLIP CDS | T547 | todo |
| T549 | Mix the same narration into browser MP4 exports | ARCH DUB CLIP CDS | T548 | todo |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | todo |
| T551 | Index the job and admission lookups every poll and provider call makes | ARCH | - | todo |
| T552 | Run a clip hold's live model-access check before its write transaction | ARCH | - | todo |
| T555 | Make the comparison list light and bound a comparison's candidate fan-out | ARCH | - | todo |
| T556 | Read voice samples once and leave an unchanged answer alone | ARCH | - | todo |
| T557 | Keep FX selection and balance reads off the writer and out of the lock | ARCH | - | todo |
| T558 | Sample browser renders without a full decode and load each original once | ARCH | - | todo |
| T559 | Take a server render's ground frames from its own lossless cut, and trim before scaling | ARCH | T558 | todo |
| T560 | Stop re-laying out a clip per caption-frames request and re-decoding JSON per poll | ARCH | - | todo |
| T561 | Take job-kind lists out of generation_jobs' CHECKs and the dispatcher SQL | ARCH | T551 | todo |
| T562 | Keep the billing pass running, charge each order once and never revert a cancel | ARCH | - | todo |
| T563 | Record every confirmed refund and release credits a refused refund froze | ARCH | T562 | todo |
| T564 | Stop the clip detail polling forever and stop each settings save from refetching it | ARCH | - | todo |
| T565 | Send each clip autosave through its own project's sender and queue the clip storyline | ARCH | - | todo |
| T566 | Keep lazily routed pages out of the entry chunk | ARCH | - | todo |
| T567 | Keep every photo turn in the cache, and stop per-autosave fingerprint reads and poll listener leaks | ARCH | - | todo |

## next
- implement-task T551–T567 (review perf-cost-261004), in order, one commit per task; update-ssot VOICE/QUOTA for perf-cost F10 (analysis corpus vs the 30 000-token hold) and BILL for the `review` order exit (review notes).
- implement-task T533; complete T533–T539 and the real voice-creation qualification before T540–T550 narrated editing/export work.
- create-task ARCH (ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎); create-task POST r32 (POST-108+ implemented by 12d2f428; verification-only); create-task VOICE r8 (VOICE-32✎ implemented by 68ae9a79; verification-only); update-ssot VOICE-31 remains open.
- ops: lower TEMPLATE_PHOTO_ROW_MAX / VITE_TEMPLATE_PHOTO_ROW_MAX to 3 wherever production sets them explicitly (VPS .env, Cloudflare build vars).

## log
- 261004 T554 done: a template-request correction carries only the request, the last answer and what it broke; BE suite green
- 261004 T553 done: storyline and template-request caps sized by LLMCompletionBudget.Short with native-effort headroom, frozen at start, hold = call; BE suite green
- 261004 create-task review/perf-cost-261004: T551–T567 (17 tasks) from 29 adopted findings; F10 back to [?] (VOICE-23/QUOTA-14 planning change); review converted
- 261004 create-task review/perf-cost-261004 start: 30 adopted findings (P1/P2 + cost/bug P3) → tasks from T551; implemented in order, one commit per task
- 261004 review-code perf-cost-261004 ready: 38 findings (P1 3 · P2 16 · P3 19); 30 adopted, maintainability P3 F22 F24–F28 F36 F37 held [?]
- 261004 T533 start: sequential implementation with one validated commit per task; spoken provider boundary first
- 261004 update-ssot POST r32: POST-108+ ①'s actions ask once (사진 없이 만들까요?) before a run with no photo or video attached (already live in 12d2f428)
- 261004 spec validation passed: 18 tasks, full changed-decision coverage, current bases, acyclic dependencies and voice-first qualification; lint 0 errors (41 format/history warnings), git diff --check clean; no implementation
- 261004 create-task DUB MODEL QUOTA CLIP CDS THEME: T533–T550 (18 tasks), voice creation/qualification first; independent captions, responsive timeline and both narrated exports next
- 261004 create-task DUB MODEL QUOTA CLIP CDS THEME start: voice qualification precedes narrated editing; explicit provider, accounting, compatibility and export acceptance
- 261004 create-ssot/update-ssot complete: DUB r2 MODEL r29 QUOTA r33 CLIP r55 CDS r32 THEME r22; all adopted ideation domains converted
- 261004 create-ssot/update-ssot DUB MODEL QUOTA CLIP CDS THEME start: convert the adopted voice-first editor scope before create-task; unrelated ARCH/VOICE/review pending excluded
- 261004 create-ssot DUB r1: description-generated private voices, audition/confirmation, account reuse and selective narration; related MODEL/QUOTA extensions precede implementation tasks
- 261004 create-ssot DUB start: confirmed custom spoken voices, account reuse and selective narration generation from adopted ideation
- 261004 ideation familiar-video-editing-and-dubbing ready: complete v1 product policy adopted; description-generated reusable voices first, responsive editor and speech-led clips next
- 261004 ideation familiar-video-editing-and-dubbing start: owner adopted the complete editor, timing, voice-library and initial-scope recommendation
- 261004 ideation familiar-video-editing-and-dubbing open: description-generated voice recommendation accepted; full editor/timing and reusable-voice packages proposed with a concrete creation flow
- 261004 review-code perf-cost-261004 start: performance, provider cost and severe maintainability in code changed since conformance-all-260927 (87206229..3e6a14ea)
- 261004 ideation familiar-video-editing-and-dubbing start: continue from description-generated voice recommendation and resolve remaining editor/lifecycle choices
- 261004 ideation familiar-video-editing-and-dubbing open: voice-creation milestone precedes dubbing; selective regeneration, independent captions and audio controls adopted; creation method/device/timing pending
