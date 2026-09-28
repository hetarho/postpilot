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
| ARCH | 12 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 9 | 9 | - | 0 |
| QUOTA | 23 | 23 | - | 0 |
| POST | 21 | 21 | - | 0 |
| VOICE | 4 | 4 | - | 1 |
| GEN | 17 | 17 | - | 0 |
| MODEL | 18 | 18 | - | 0 |
| TMPL | 15 | 15 | - | 1 |
| GUIDE | 9 | 9 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 6 | 6 | - | 0 |
| THEME | 19 | 15 | THEME-19✎ | 0 |
| MKT | 7 | 7 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 49 | 43 | CLIP-163+ | 2 |
| CDS | 29 | 29 | - | 1 |
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
| T416 | A caption face's coverage is the set of characters it actually draws | CDS | - | blocked@260927 |

## next
- owner review of /IMPLEMENTATION-DECISIONS.md (the ambiguous calls of T414–T448), then review-code the clip wave
- update-ssot CDS for T416 (blocked): what a caption does when its face — the default 크게 강조 included — has no ink for a syllable; update-ssot AUTH-36: name VerifyEmail among the throttled writes (T427 throttles it); update-ssot MODEL-25 (its never-applied-on-mount/login/account-creation rule sits after the ← reason, rule or reason?) and TMPL-6 (no field rule for title_area)
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); create-task MKT THEME (/about overflow) and CLIP CDS THEME (Wanted Sans delta); ARCH-5's context list is stale (lacks clip, quality, voucher and others); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; finding (T400): Paperlogy's cmap maps all 11,172 syllables but draws 8,392 empty (e.g. 갂), so the cmap-based faceCoverage passes them and a Paperlogy caption renders them blank; update-ssot POST-39 (DeleteImage drops the row before the object since T358, see T411 result); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review the other 17 SSOTs (POST GEN GUIDE next; THEME-24 THEME-38 MODEL-37 stayed blocks, split candidates); THEME's row says tasked 15 though T356 and T412 finished on THEME@18

## log
- 260928 doc-review THEME MODEL TMPL tidied (rev unchanged): long decisions → decision blocks; FORMAT gains the block notation and the no-rev wording rule
- 260928 doc-review ssot/ start: scope and FORMAT decision-block adoption asked
- 260928 T444 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T444 claimed (ia)
- 260928 T415 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T415 found (out of scope): a storyline paragraph edit waiting for its 600 ms autosave is not flushed before 이 스토리로 만들기 or a storyline request
- 260928 T415 claimed (ia)
- 260928 T442 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T442 claimed (ia)
- 260928 T441 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T441 claimed (ia)
- 260928 T443 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T443 claimed (ia)
- 260928 T448 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T448 claimed (ia)
- 260928 T447 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T447 claimed (ia)
- 260928 T446 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T446 claimed (ia)
- 260928 T445 done: decisions in /IMPLEMENTATION-DECISIONS.md
