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
| perf-cost-261004 | converted@261005 |

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
| T569 | Hold a voice analysis at the size of the prompt it will send | QUOTA VOICE ARCH | - | doing@261005.pc |
| T572 | Delete the pre-FX pricing regime from the ledger | QUOTA ARCH | T569 | doing@261005.pc |
| T573 | Delete the non-Portable renderer and the legacy plan paths | ARCH | - | doing@261005.pc |
| T574 | One cut timeline for the render graph, the sampler and the browser drawing | CDS CLIP ARCH | T573 | doing@261005.pc |
| T576 | State the export window, upgrade proration and term end once | QUOTA BILL ARCH | T572 | doing@261005.pc |
| T579 | Make the clip release smoke green again | ARCH | T574 | doing@261005.pc |

## next
- implement-task T569–T579 (perf-cost follow-ups, QUOTA r34, BILL r9, release smoke), one commit per task, then push and watch CI.
- implement-task T533; complete T533–T539 and the real voice-creation qualification before T540–T550 narrated editing/export work.
- create-task ARCH (ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎); create-task POST r32 (POST-108+ implemented by 12d2f428; verification-only); create-task VOICE r8 (VOICE-32✎ implemented by 68ae9a79; verification-only); update-ssot VOICE-31 remains open.
- ops: lower TEMPLATE_PHOTO_ROW_MAX / VITE_TEMPLATE_PHOTO_ROW_MAX to 3 wherever production sets them explicitly (VPS .env, Cloudflare build vars).

## log
- 261005 T580 done: a refused renewal lapses a subscription still on the term it paid for, even if another write touched the row (found in T570); BE suite green
- 261005 T578 done: what a refunded payment funded is one billing value the usage/clip predicates and the 0130 guard triggers all read; upgrades fund their window's lazy lots; BE suite green
- 261005 T570 done: each billing pass refunds a review order in full (idempotent cancel, read-back), fails it, records and mails the refund, unlocks the account; a refunded renewal lapses the account (BILL-8); BE suite green
- 261005 T571 done: a lint refuses whole-database or bare foreign-key checks in migrations after 0129; BE suite green
- 261005 T575 done: one precondition chain for every generation start (each keeps its check order), one observe+write call plan, one refusal mapping, one comparison create-enqueue-link helper; BE suite green
- 261005 T577 done: export panel and template editor state machines moved into widget model hooks and a new features/edit-template slice; FE suite 382/3186 green
- 261005 T569–T579 start (tag pc): five isolated worktrees by blast radius
- 261005 create-task QUOTA r34 BILL r9 review/perf-cost-261004: T569–T579 (11 tasks: F10 hold, BILL-22 refunds, the 8 held maintainability findings, release smoke); review converted again
- 261005 update-ssot QUOTA r34 BILL r9: QUOTA-14✎ a voice analysis holds its real prompt size (perf-cost F10); BILL-22+ a captured payment that cannot be applied is refunded in full automatically (perf-cost F12)
- 261005 update-ssot QUOTA BILL start: owner chose the recommended F10 and review-exit options
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
