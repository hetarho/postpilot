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

## next
- implement-task T533; complete T533–T539 and the real voice-creation qualification before T540–T550 narrated editing/export work.
- create-task ARCH (ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎); create-task POST r32 (POST-108+ implemented by 12d2f428; verification-only); create-task VOICE r8 (VOICE-32✎ implemented by 68ae9a79; verification-only); update-ssot VOICE-31 remains open.
- ops: lower TEMPLATE_PHOTO_ROW_MAX / VITE_TEMPLATE_PHOTO_ROW_MAX to 3 wherever production sets them explicitly (VPS .env, Cloudflare build vars).

## log
- 261005 perf-cost wave complete: T551–T584 done (review perf-cost-261004 fully converted, QUOTA r34, BILL r9, release smoke 28/28); pushing main
- 261005 T584 done: refund store and provider behaviour are required billing ports (no type assertions); a provider-failed reviewed refund answers REFUND_FAILED with operator copy; BE and FE suites green
- 261005 create-task T584 (left open by T583: optional refund ports, untyped failed refund) and start
- 261005 T583 done: retired purchase-refund reasons reserved in proto, RefundPurchase and its ports deleted, billing USD columns dropped (0132), IntentStore folded into Store; BE and FE suites green
- 261005 create-task T583 (left open by T582: PURCHASE_TOO_SMALL, RefundPurchase, USD columns, IntentStore) and start
- 261005 T582 done: billing has one regime (fixed KRW): the flag, non-fixed branches, USD rates port and legacy monthly-lot credit methods are deleted; seed opens benefits like production; BE suite green
- 261005 T581 done: model-combo and clip estimates price through the FX snapshot alone; no rate means no figure; USD helpers deleted; BE suite green
- 261005 T579 done: the release smoke's caption expectation follows CLIP-196's region fitting; 28/28 modes pass; host test pins it
- 261005 T574 done: one cut timeline drives the ffmpeg graph, merge tree, audio and sampler; the browser draws xfade's weights from a Go-written fixture; smokes, identity digest and FE suite green
- 261005 T573 done: the non-Portable renderer, legacy plan resolution, cut-caption scheduler, answerAccent and LatestForVoiceKind are deleted; a plan without a composition is refused before media work; smokes and identity digest unchanged
- 261005 create-task T581 T582 (follow-ups found in T572: USD-terms estimator, the unused non-fixed billing regime) and start
- 261005 T576 done: one export-window rule (usage.ExportWindowAt) for ledger, root and seed; proration via plan.QuoteUpgrade; TermEnd via plan.CoverageEnd; refundWindow constant; BE suite green
- 261005 T572 done: the ledger prices through FX alone (required rate selector), legacy branches and copies deleted, migration 0131 closes pre-FX admissions and legacy monthly lots once; BE suite green
- 261005 T569 done: a voice analysis freezes its 학습 글 snapshot at start and holds max(30 000, prompt runes); the run reads only the frozen ids; BE suite green
- 261005 T580 done: a refused renewal lapses a subscription still on the term it paid for, even if another write touched the row (found in T570); BE suite green
- 261005 T578 done: what a refunded payment funded is one billing value the usage/clip predicates and the 0130 guard triggers all read; upgrades fund their window's lazy lots; BE suite green
- 261005 T570 done: each billing pass refunds a review order in full (idempotent cancel, read-back), fails it, records and mails the refund, unlocks the account; a refunded renewal lapses the account (BILL-8); BE suite green
- 261005 T571 done: a lint refuses whole-database or bare foreign-key checks in migrations after 0129; BE suite green
- 261005 T575 done: one precondition chain for every generation start (each keeps its check order), one observe+write call plan, one refusal mapping, one comparison create-enqueue-link helper; BE suite green
- 261005 T577 done: export panel and template editor state machines moved into widget model hooks and a new features/edit-template slice; FE suite 382/3186 green
