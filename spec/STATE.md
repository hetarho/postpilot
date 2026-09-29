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
| voice-tidy | open@260929 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 13 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 10 | 10 | - | 0 |
| QUOTA | 23 | 23 | - | 0 |
| POST | 24 | 24 | - | 0 |
| VOICE | 4 | 4 | - | 1 |
| GEN | 18 | 18 | - | 0 |
| MODEL | 19 | 19 | - | 0 |
| TMPL | 16 | 16 | - | 1 |
| GUIDE | 11 | 11 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 6 | 6 | - | 0 |
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
| T462 | Sample every unplated text's ground from the originals through one shared sampler | CLIP CDS ARCH | - | doing@260929.rr |
| T463 | Sample a browser render's grounds on a media worker and serve its assets with them | CLIP ARCH | T462 | todo |
| T464 | A browser render waits for its sampling job and draws from render-bound assets | CLIP ARCH | T463 | todo |

## next
- next: implement-task T462 (then T463, T464: a browser render draws the server-sampled scrim under CLIP-192)
- owner review of /IMPLEMENTATION-DECISIONS.md (the ambiguous calls of T414–T448 and T451–T456), then review-code the clip wave
- ideation voice-tidy continues (open: the axis set under the owner's Korean-research rule, readiness-meter numbers, the prompt photo source, the VOICE-49 analyze experiment, existing voices), then update-ssot VOICE GEN POST GUIDE AUTH QUOTA; ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); the /about header overflow at 320px/200% text still wants a task (MKT THEME); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; a rapid phrase edge between two frames (e.g. 1020 ms at 30 fps) gives both phrases that frame, and the browser draws the earlier one there; a browser render started 23 s after a server render once showed no outcome for 30 minutes (not reproduced); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26)

## log
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
- 260929 ideation voice-tidy open: 말투 learns only from prose the owner wrote (pasted posts, per-분야 photo/situation prompts, a readiness meter, one analysis at 100%); read-only 말투 분석 with research-defined axes; 검증 beside the owner's answer; optional voice and 기본; drops 대조 규칙, finished-post learning, 문장 의견, the 버전 기록 tab, the seed and 규칙으로 저장
- 260929 ideation voice-tidy start (does the voice apply; what 말투/프로필/버전/측정·분석/여섯 성향/대조 규칙/검증 mean; list and detail pages)
- 260929 T460 done: one closed-row guideline list (기본 지침 in use, then the owner's) with a 기본 지침 sheet and a one-form edit
- 260929 T460 claimed (sm)
- 260929 T459 done: guidelines carry an optional title (≤40, migration 0104) offered on every create surface and never in a prompt
- 260929 T459 claimed (sm)
- 260929 T452 done: the storyline call and 바로 만들기's flow call draft the generated intro/outro slots in their one approved call; builds and revisions copy the slot words; region inputs join quotes, payloads and recovery
- 260929 T461 done: ① is 가제 → template fields → memo → photos; the 가제's Enter goes to the next field on screen
- 260929 T461 claimed (sm)
- 260929 create-task POST r24 → T461
- 260929 update-ssot POST r24: ①'s memo moves below the template data fields; no active task/worker affected
- 260929 T458 done: a memory row is its text over one badge line with 수정/삭제 icons; one form saves text, kind and tags
