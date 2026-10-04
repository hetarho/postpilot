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
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
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

## next
- update-ssot VOICE/QUOTA for perf-cost F10 (analysis corpus vs the 30 000-token hold) and BILL for an exit from a `review` order (review perf-cost-261004 notes); the held maintainability findings F22 F24–F28 F36 F37 wait for a later review-code.
- release smoke: 9/28 modes fail at HEAD with `the delivered caption was retimed 2500 6000`, identically on 228a4015 (seen in T558) — investigate; rebuild the dev media image for T559's `select` filter before rendering locally.
- resume implement-task T539 after the supplier environment, account tariff/capacity evidence and approved whole-session USD ceiling are supplied; complete real Korean listening/continuity qualification before T540–T550. Procedure: docs/qa/spoken-voice-v1.md.
- create-task ARCH (ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎); create-task POST r32 (POST-108+ implemented by 12d2f428; verification-only); create-task VOICE r8 (VOICE-32✎ implemented by 68ae9a79; verification-only); update-ssot VOICE-31 remains open.
- ops: lower TEMPLATE_PHOTO_ROW_MAX / VITE_TEMPLATE_PHOTO_ROW_MAX to 3 wherever production sets them explicitly (VPS .env, Cloudflare build vars).

## log
- 261005 T539 blocked: production-job harness, atomic session ceilings and private evidence audit delivered; full BE CI, qualification race and 61 deploy tests passed; live key/account tariffs/approval/listening prerequisites absent, no supplier call or readiness promotion
- 261005 T539 start: bounded production-port qualification harness and private evidence audit; live prerequisites absent locally, no supplier call or readiness promotion
- 261005 T538 done: explicit model/quote/listen/select/confirm UI, private reuse and same-request recovery; 3148 FE tests, full local CI and 30 responsive/theme browser views passed
- 261005 T538 start: explicit model/quote/audition/confirmation flow, reusable spoken library and authenticated gesture playback
- 261005 T537 done: exact-input voice quotes, once-only durable jobs, atomic cancellation/publication, private reuse probes and known-identity restart recovery; full local CI passed
- 261005 T536 done: private immutable spoken library, authenticated audition access, tombstones and race-safe recoverable media cleanup
- 261004 T535 done: exact-input bounded speech quotes, durable call claims and typed decimal usage; ceiling/unknown/failure/cancellation/refund/master SQLite and race checks plus full local CI passed
- 261004 T534 done: immutable speech-profile revisions, separate admin tab, price-free owner choices and owner-scoped provisional qualification; full local CI passed
- 261004 T533 done: typed speech ports, optional ElevenLabs connection, bounded MP3/timing validation and decimal reported billing evidence; full local CI gates passed
- 261005 T568 done: a fixed-KRW cancel compares the row it read, so it succeeds after time has passed and still refuses a row changed in between; BE suite green
- 261005 create-task T568 (owner: fix at once): fixed-KRW CancelSubscription compares a read UpdatedAt it already overwrote; T568 start
- 261005 T566 done: lazily routed pages leave the entry chunk (sideEffects + build-only route-schema plugin, accepted by the owner); first visit 1.90 MB → 1.46 MB; checked in Playwright
- 261005 T560 done: a browser export lays out once per render revision and batches resvg per role, GetClipProject decodes analysis once, ListClipProjects reads a summary projection; smokes green
- 261005 T559 done: server render reads ground frames from its own lossless bare cut, browser sampling selects before scaling in one output; select joins the ffmpeg allowlist; render_footage 15.1–16.1 s → 8.9–10.0 s on the identity fixture; smokes and identity digest unchanged
- 261005 T558 done: sampling jobs fetch only the originals their reads need with no full decode, a server render downloads and verifies each original once, cuts visited grouped by source; production smokes and identity digest unchanged; release smoke 9/28 red identically at base
- 261004 T567 done: photo turns patch the cache on every call, the fingerprint reads once per autosave pause, one abortable delay frees its listeners; FE suite green
- 261004 T565 done: clip settings and region queues send per project, the clip storyline saves through its own keyed queue flushed before builds and requests and keeps text on failure; FE suite green
- 261004 T564 done: clip detail waits for settlement only on generate_clip/revise_clip, a settings save keeps the cached plan unless its revision moved and refreshes only list and templates; FE suite green
- 261004 out of scope, found while implementing T562: bug: fixed-KRW CancelSubscription (billing/change.go:190) sets UpdatedAt=now before comparing it with the stored row, so a customer cancel always fails ErrStaleQuote in production (BILL-7, since T479) — needs its own task
- 261004 T556 done: voice directory and profile read samples once, the check list skips projections, an unchanged answer is left alone; BE suite green
