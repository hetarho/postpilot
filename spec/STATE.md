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

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 14 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ | 0 |
| AUTH | 10 | 10 | - | 0 |
| QUOTA | 24 | 23 | QUOTA-13✎ QUOTA-14✎ | 0 |
| POST | 25 | 24 | POST-4✎ POST-8✎ POST-13✎ POST-16✎ POST-23✎ POST-24✎ POST-25✎ POST-28✎ POST-46✎ POST-47✎ POST-49✎ POST-51✎ POST-54✎ POST-56✎ POST-57✎ POST-62✎ POST-71✎ POST-72✎ POST-74✎ POST-82✎ POST-96✎ POST-101+ POST-102+ POST-21- | 0 |
| VOICE | 5 | 4 | VOICE-1✎ VOICE-2✎ VOICE-3✎ VOICE-4✎ VOICE-5✎ VOICE-6✎ VOICE-8✎ VOICE-9✎ VOICE-10✎ VOICE-12✎ VOICE-13✎ VOICE-15✎ VOICE-16✎ VOICE-20✎ VOICE-21✎ VOICE-22✎ VOICE-23✎ VOICE-24✎ VOICE-25✎ VOICE-26✎ VOICE-27✎ VOICE-30✎ VOICE-31✎ VOICE-32✎ VOICE-43✎ VOICE-44✎ VOICE-45✎ VOICE-46✎ VOICE-47✎ VOICE-50✎ VOICE-51✎ VOICE-52✎ VOICE-53✎ VOICE-54✎ VOICE-55✎ VOICE-56✎ VOICE-57✎ VOICE-58✎ VOICE-59+ VOICE-60+ VOICE-61+ VOICE-62+ VOICE-63+ VOICE-64+ VOICE-7- VOICE-11- VOICE-17- VOICE-18- VOICE-19- VOICE-28- VOICE-29- VOICE-33- VOICE-34- VOICE-35- VOICE-36- VOICE-37- VOICE-38- VOICE-39- VOICE-40- VOICE-41- VOICE-42- VOICE-48- VOICE-49- | 0 |
| GEN | 19 | 18 | GEN-14✎ GEN-17✎ GEN-23✎ GEN-25✎ GEN-27✎ GEN-30✎ GEN-38✎ GEN-40✎ GEN-41✎ GEN-43✎ GEN-46✎ GEN-74+ GEN-75+ GEN-34- GEN-39- | 0 |
| MODEL | 20 | 19 | MODEL-16✎ MODEL-23✎ MODEL-25✎ MODEL-26✎ MODEL-30✎ MODEL-31✎ MODEL-36✎ MODEL-37✎ MODEL-39✎ MODEL-41✎ MODEL-44✎ MODEL-62✎ MODEL-67+ MODEL-43- | 0 |
| TMPL | 17 | 16 | TMPL-1✎ TMPL-12✎ | 1 |
| GUIDE | 12 | 11 | GUIDE-15✎ GUIDE-21✎ GUIDE-41✎ | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 7 | 6 | LANG-1✎ LANG-2✎ LANG-13✎ LANG-14✎ LANG-15✎ LANG-18✎ LANG-20✎ LANG-21✎ LANG-26✎ LANG-28✎ LANG-19- | 0 |
| THEME | 19 | 19 | - | 0 |
| MKT | 8 | 8 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 52 | 52 | - | 2 |
| CDS | 31 | 31 | - | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 5 | 5 | - | 2 |
| QUAL | 6 | 6 | - | 0 |
| GIFT | 2 | 2 | - | 0 |

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

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T463 | Sample a browser render's grounds on a media worker and serve its assets with them | CLIP ARCH | T462 | todo |
| T464 | A browser render waits for its sampling job and draws from render-bound assets | CLIP ARCH | T463 | todo |

## next
- next: implement-task T463 (then T464: a browser render draws the server-sampled scrim under CLIP-192)
- owner review of /IMPLEMENTATION-DECISIONS.md (the ambiguous calls of T414–T448 and T451–T456), then review-code the clip wave
- create-task VOICE GEN POST GUIDE QUOTA MODEL LANG TMPL ARCH (VOICE r5: the voice as the owner's fingerprint); after the new voice ships, the one-time prod hand edit that keeps only `맛집 리뷰 블로거 학습` (its pasted post as the one 학습 글); ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); the /about header overflow at 320px/200% text still wants a task (MKT THEME); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; a rapid phrase edge between two frames (e.g. 1020 ms at 30 fps) gives both phrases that frame, and the browser draws the earlier one there; a browser render started 23 s after a server render once showed no outcome for 30 minutes (not reproduced); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26)

## log
- 260929 T462 done: one originals-based ground sampler (the composition's frame arithmetic, each cut's own chain, xfade's dissolve and fade through black) serves the server render and `SampleGrounds`; a server render of the T456 clip is byte-identical to the one before; `clip.SampledGround` round-trips
- 260929 update-ssot VOICE r5 GEN r19 POST r25 GUIDE r12 QUOTA r24 MODEL r20 LANG r7 TMPL r17 ARCH r14: the voice is the owner's fingerprint — 학습 글 only, eight counted items plus a short AI part, a readiness meter, 검증 and 말투 반영 비교, optional voice and 기본, Korean only, learning, rules, versions and the analyze comparison removed; AUTH needs no change (VOICE-4); THEME-23 lost a stale example (wording); no active task affected — T462–T464 cite ARCH-45…51, not the changed ARCH-34
- 260929 update-ssot VOICE GEN POST GUIDE AUTH QUOTA MODEL LANG start (from ideation voice-tidy: the voice as the owner's fingerprint)
- 260929 ideation voice-tidy ready: 말투 is the owner's fingerprint — eight counted habits (endings, sentence-final marks, emoji, sentence/paragraph shape, opening/closing lines, adverbs, first person, headings/lists) plus a short AI description, the six axes dropped; 60 sentences from pasted posts or one shared prompt set on the owner's own photos; 검증, ② and the model lab's 말투 반영 비교 (replacing the analyze comparison) compare fingerprints with no extra call; Korean voices only, tied to no 분야 or template, several per owner by mood; no paid A/B now
- 260929 T456 follow-up: a rapid phrase in a sequence style is one raster of its style in the server render (CDS-4), so the draft preview now serves it as one (`sequenceDrawn`) and the frame endpoint refuses it; the first fix, which served it animated frames, is withdrawn; Chrome and server renders now match at 1.5 s (0.00 % of pixels off)
- 260929 T462 claimed (rr)
- 260929 create-task CLIP r52 → T462 T463 T464 (one originals-based ground sampler for both kinds; a media-worker sampling stage bound to the browser render; the page waits for it and sends render_id)
- 260929 ideation voice-tidy resume (open: axis set, readiness meter, prompt photos, VOICE-49, existing voices)
- 260929 create-task CLIP start (r52: CLIP-192+)
- 260929 update-ssot CLIP r52: CLIP-192+ the server samples the retained originals so a browser render draws the CDS-44 scrim, accent colour and contrast notices; no active task/worker affected
- 260929 update-ssot CLIP CDS start (a browser render draws the CDS-44 scrim)
- 260929 T456 done: one real run (바로 만들기, gemini-3.8-flash, 5 calls, $0.0107) rendered on the CPU server and in Chrome WebCodecs agree on duration, codecs, frames, slots, the owner caption and the muted span; the run found and fixed caption frames serving only a rapid caption's first phrase (every browser render of a rapid sequence-style caption failed); the browser render's missing scrim is left to update-ssot
- 260929 T456 resumed (rr): the owner approved one real provider generation run (≈ $0.012) for the browser and CPU server render checks
- 260929 T456 blocked: preview/export parity on every ratio and pace, the no-template workflow to both render kinds, older-project fixtures, flush-before-render regressions and a real CPU render smoke of the edited clip all pass; the real browser render needs a plan, which needs a paid generation call or a dev provider double
- 260929 T456 claimed (p15)
- 260929 T455 done: ②'s caption sheet picks any approved style from renderer-drawn tiles, keeps the owner size (a size the drawn style cannot take holds the save on that field), and follows undo/redo
- 260929 T455 claimed (p15)
- 260929 T454 done: an owner may give a caption any approved style outside the AI set; one rule draws every caption, so a set change restyles only captions naming no style and moves the revision only then; a size the drawn style cannot take is refused as caption_size with its range
- 260929 T454 claimed (p15)
- 260929 T453 done: ② edits the intro/outro slots around the storyline body before any template, body or plan, and ① offers 사용 안 함; a drawn slot's words are the plan's rows, and the settings, slot and correction saves share one write lane
- 260929 T453 claimed (p15)
