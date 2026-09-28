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

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 13 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 10 | 10 | - | 0 |
| QUOTA | 23 | 23 | - | 0 |
| POST | 22 | 22 | - | 0 |
| VOICE | 4 | 4 | - | 1 |
| GEN | 18 | 18 | - | 0 |
| MODEL | 19 | 19 | - | 0 |
| TMPL | 16 | 16 | - | 1 |
| GUIDE | 10 | 10 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 6 | 6 | - | 0 |
| THEME | 19 | 19 | - | 0 |
| MKT | 8 | 8 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 50 | 43 | CLIP-163+ | 2 |
| CDS | 30 | 30 | - | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 4 | 4 | - | 2 |
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

## next
- owner review of /IMPLEMENTATION-DECISIONS.md (the ambiguous calls of T414–T448), then review-code the clip wave
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); the /about header overflow at 320px/200% text still wants a task (MKT THEME); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26)

## log
- 260928 T416 done: a caption character its face does not draw is set in Wanted Sans Variable inside its own style
- 260928 finding (T416): backend/examples/overlays/shorts-editorial no longer loads (card-v1 views, no keynote/film/region/info bindings); only its caption template was brought to the runs contract
- 260928 T416 claimed (gs)
- 260928 create-task CDS r30 → T416 revised (unblocked); AUTH r10 no-op (T427 throttles VerifyEmail); THEME r16..r19 no-op (Wanted Sans Variable already first in --font-sans, the renderer's pinned file; r17..r19 consumed by T356 T412 T427)
- 260928 create-task CDS AUTH THEME start
- 260928 update-ssot CDS-84✎ CDS-17✎ CDS-52✎ AUTH-36✎ (a caption character its face does not draw is set in Wanted Sans Variable; VerifyEmail named among the throttled writes)
- 260928 update-ssot CDS AUTH start (T416's decision; the stale doc rows)
- 260928 T449 done: 기억을 통한 감상 추가 (memory_impressions) with the 취향: label and the /guidelines note
- 260928 T449 claimed (mi)
- 260928 create-task T449 (GEN r18, GUIDE r10); POST r22 MKT r8 MODEL r19 TMPL r16 CLIP r50 ARCH r13 no-op (no code impact)
- 260928 create-task POST GEN GUIDE CLIP ARCH MKT MODEL TMPL start (260928 deltas)
- 260928 update-ssot POST-64✎ POST-45✎ POST-51✎ GEN-16✎ GEN-73+ GUIDE-41✎ CLIP-40✎ ARCH-30✎ MKT-5✎ MODEL-25✎ TMPL-6✎
- 260928 update-ssot POST GEN GUIDE CLIP ARCH MKT MODEL TMPL start (the doc-review holds, answered)
- 260928 doc-review QUAL MEM GIFT AUTH BILL tidied (rev unchanged); all 20 SSOTs reviewed, lint candidates 209 → 14
- 260928 doc-review QUAL MEM GIFT AUTH BILL start
- 260928 doc-review MKT LANG VIDEO EXPORT QUOTA tidied (rev unchanged)
- 260928 doc-review MKT LANG VIDEO EXPORT QUOTA start
- 260928 doc-review ARCH VOICE CDS tidied (rev unchanged); MODEL-37 now points at VOICE-13 for DeleteVoice
- 260928 doc-review ARCH VOICE CDS start
- 260928 doc-review CLIP tidied (rev unchanged)
