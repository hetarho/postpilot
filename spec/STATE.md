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
| T415 | Clip ① and ② behave and speak as CLIP-14, CLIP-21/23, CLIP-39 and CLIP-121 decide | CLIP | T441 | todo |
| T416 | A caption face's coverage is the set of characters it actually draws | CDS | - | blocked@260927 |
| T441 | A video template carries a starting design selection, and a project takes it on selection | CLIP CDS | T443 | todo |
| T442 | The video-template preview plays an illustrative timed clip | CLIP | T441 | todo |
| T443 | Every video-template builder entry opens in place with its own delete | CLIP | T414 | todo |
| T444 | ②'s flow simulation over still cut frames | CLIP | - | todo |

## next
- implement-task T443 (→ T441 → T442, T415) and T444
- update-ssot CDS for T416 (blocked): what a caption does when its face — the default 크게 강조 included — has no ink for a syllable; update-ssot AUTH-36: name VerifyEmail among the throttled writes (T427 throttles it); implement-task T414 T415 after their refresh
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); create-task MKT THEME (/about overflow) and CLIP CDS THEME (Wanted Sans delta); ARCH-5's context list is stale (lacks clip, quality, voucher and others); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; finding (T400): Paperlogy's cmap maps all 11,172 syllables but draws 8,392 empty (e.g. 갂), so the cmap-based faceCoverage passes them and a Paperlogy caption renders them blank; update-ssot POST-39 (DeleteImage drops the row before the object since T358, see T411 result); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results)

## log
- 260928 T448 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T448 claimed (ia)
- 260928 T447 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T447 claimed (ia)
- 260928 T446 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T446 claimed (ia)
- 260928 T445 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T445 claimed (ia)
- 260928 T440 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T440 claimed (ia)
- 260928 T439 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T439 claimed (ia)
- 260928 T438 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T438 claimed (ia)
- 260928 T437 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T437 claimed (ia)
- 260928 T436 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T436 claimed (ia)
- 260928 T435 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T435 claimed (ia)
