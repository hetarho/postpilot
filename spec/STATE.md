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
- implement-task T541–T550 sequentially with automated checks and task commits, then merge/push main; T539 real supplier evidence is deferred and ordinary production readiness stays closed.
- create-task ARCH (ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎); create-task POST r32 (POST-108+ implemented by 12d2f428; verification-only); create-task VOICE r8 (VOICE-32✎ implemented by 68ae9a79; verification-only); update-ssot VOICE-31 remains open.
- ops: lower TEMPLATE_PHOTO_ROW_MAX / VITE_TEMPLATE_PHOTO_ROW_MAX to 3 wherever production sets them explicitly (VPS .env, Cloudflare build vars).

## log
- 261005 T540 done: independent spoken scripts, v7/legacy mapping, private project speech assets, selective staleness/reuse, protected caption links and publication CAS; full local CI passed
- 261005 T540 start: independent spoken-script contract, private speech provenance, revision guards and old-plan compatibility
- 261005 create-task DUB start: move real qualification after implementation, retain unfulfilled live evidence tasks and normal automated gates
- 261005 update-ssot DUB start: defer live supplier qualification until complete implementation; keep production readiness evidence mandatory
- 261005 dubbing integration start: preserve completed T533–T538, defer live qualification per owner, reconcile main FX/refund work and unused speech migration numbers
- 261005 T539 blocked: production-job harness, atomic session ceilings and private evidence audit delivered; full BE CI, qualification race and 61 deploy tests passed; live key/account tariffs/approval/listening prerequisites absent, no supplier call or readiness promotion
- 261005 T539 start: bounded production-port qualification harness and private evidence audit; live prerequisites absent locally, no supplier call or readiness promotion
- 261005 T538 done: explicit model/quote/listen/select/confirm UI, private reuse and same-request recovery; 3148 FE tests, full local CI and 30 responsive/theme browser views passed
- 261005 T538 start: explicit model/quote/audition/confirmation flow, reusable spoken library and authenticated gesture playback
- 261005 T537 done: exact-input voice quotes, once-only durable jobs, atomic cancellation/publication, private reuse probes and known-identity restart recovery; full local CI passed
- 261005 T536 done: private immutable spoken library, authenticated audition access, tombstones and race-safe recoverable media cleanup
- 261004 T535 done: exact-input bounded speech quotes, durable call claims and typed decimal usage; ceiling/unknown/failure/cancellation/refund/master SQLite and race checks plus full local CI passed
- 261004 T534 done: immutable speech-profile revisions, separate admin tab, price-free owner choices and owner-scoped provisional qualification; full local CI passed
- 261004 T533 done: typed speech ports, optional ElevenLabs connection, bounded MP3/timing validation and decimal reported billing evidence; full local CI gates passed
- 261005 post-deploy media verify of 426e1331 failed TestClipWriterInputRelease: since T572 the release harness prices through FX, so its 5000-credit lot no longer covered a 6595 hold and failed attempts now earn QUOTA-60 compensation; the harness lot and balance checks follow FX; production smokes and release smoke 28/28 pass locally
- 261005 deploy of d645c1ca failed at the media-tools build: code.videolan.org served GitHub runners a challenge page for the x264 archive (T559 changed media-tools.sh, so the cache missed); media-tools.sh now falls back to the GitHub mirror of the same revision under its own checksum
- 261005 perf-cost wave complete: T551–T584 done (review perf-cost-261004 fully converted, QUOTA r34, BILL r9, release smoke 28/28); pushing main
- 261005 T584 done: refund store and provider behaviour are required billing ports (no type assertions); a provider-failed reviewed refund answers REFUND_FAILED with operator copy; BE and FE suites green
- 261005 create-task T584 (left open by T583: optional refund ports, untyped failed refund) and start
- 261005 T583 done: retired purchase-refund reasons reserved in proto, RefundPurchase and its ports deleted, billing USD columns dropped (0132), IntentStore folded into Store; BE and FE suites green
