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

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 12 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 9 | 9 | - | 0 |
| QUOTA | 22 | 22 | - | 0 |
| POST | 20 | 20 | - | 0 |
| VOICE | 4 | 4 | - | 1 |
| GEN | 16 | 16 | - | 0 |
| MODEL | 18 | 18 | - | 0 |
| TMPL | 14 | 14 | - | 1 |
| GUIDE | 8 | 8 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 6 | 6 | - | 0 |
| THEME | 19 | 15 | THEME-19✎ | 0 |
| MKT | 7 | 7 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 48 | 40 | CLIP-13✎ CLIP-163+ CLIP-4✎ CLIP-14✎ CLIP-42✎ CLIP-111✎ CLIP-130✎ CLIP-139✎ CLIP-166+ CLIP-167+ CLIP-168+ CLIP-169+ CLIP-170+ CLIP-171+ CLIP-172+ CLIP-53✎ CLIP-173+ CLIP-174+ CLIP-175+ CLIP-176+ | 2 |
| CDS | 28 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ CDS-61✎ | 1 |
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
| T414 | Template authoring, the 형식 안내 and paste follow the outline grammar alone | CLIP | T413 | todo |
| T415 | Clip ① and ② behave and speak as CLIP-14, CLIP-21/23, CLIP-39 and CLIP-121 decide | CLIP | - | todo |
| T416 | A caption face's coverage is the set of characters it actually draws | CDS | - | blocked@260927 |
| T424 | Place/link template slots and every slot token are gone | TMPL GEN EXPORT | - | todo |
| T425 | Template grammar errors, attributes, ranges and builder copy follow TMPL | TMPL | T424 | todo |
| T426 | ②'s M2 minimum, the guideline queue notice and the public copy are right | QUAL GUIDE MKT | - | todo |
| T427 | CI runs gofmt, plan mapping fails closed, catalog tokens resolve, refunds are labelled, VerifyEmail is throttled | ARCH BILL AUTH THEME | - | todo |
| T428 | Template photo places bind no photo and a repeat renders once | TMPL GEN | T424 | todo |
| T429 | The write sets the day's flow first and places every photo along it | GEN | T428 | todo |
| T430 | The template builder describes photo places the writer fills | TMPL | T425 | todo |

## next
- create-task CLIP CDS (CLIP r48 CDS r28): video templates carry the design selection a project takes on selection; the template preview's 15–90 s timing, 3–4 s captions and last frame; builder entries open in place with their own delete; ②'s flow simulation over still cut frames — also refresh T414 (its preview-select removal and builder items meet CLIP-166/169/172) and T415 (F58/F59/F73's copy assumed a template carries no design) before they are implemented
- implement-task T428 after T424 (template photo places unbound), then T429 (the write sets the day's flow first); T430 after T425 (builder copy)
- update-ssot CDS for T416 (blocked): what a caption does when its face — the default 크게 강조 included — has no ink for a syllable
- implement-task the P1-bearing tasks T424 T426, then T425 T427 (review/conformance-all-260927); T414 T415 after their refresh
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached
- Later: update-ssot CLIP narration — captions read as a shot list, one observation caption per cut (no story step before the flow, every claim must cite an observation, visit/taste markers dropped without an instruction, none of the post writer's memo/voice/memory); update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); create-task MKT THEME (/about overflow) and CLIP CDS THEME (Wanted Sans delta); ARCH-5's context list is stale (lacks clip, quality, voucher and others); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; finding (T400): Paperlogy's cmap maps all 11,172 syllables but draws 8,392 empty (e.g. 갂), so the cmap-based faceCoverage passes them and a Paperlogy caption renders them blank; update-ssot POST-39 (DeleteImage drops the row before the object since T358, see T411 result); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results)

## log
- 260927 T423 done
- 260927 T423 claimed (ia): base POST@18→20 (r19/r20 no code impact)
- 260927 T422 done
- 260927 T422 claimed (ia)
- 260927 T421 done
- 260927 T421 claimed (ia)
- 260927 T420 done
- 260927 T420 claimed (ia)
- 260927 T419 done
- 260927 T419 claimed (ia)
- 260927 T418 done
- 260927 T418 claimed (ia)
- 260927 T417 done
- 260927 T413 done
- 260927 T417 claimed (ia): base GEN@14→16 (r15/r16 changed GEN-14/47/67 only)
- 260927 T416 blocked (ia): CDS-84's fallback target is itself Paperlogy — 크게 강조 draws 2,780 of 11,172 syllables (갂 쎯 empty), so an ink-aware check leaves a Paperlogy caption nothing to fall back to; update-ssot CDS decides
- 260927 T416 claimed (ia)
- 260927 create-task GEN TMPL POST QUOTA VIDEO done (sp): GEN r16 TMPL r14 → T428 (photo places bind nothing, repeat renders once, dep T424) → T429 (flow-first write answer, every photo once) · T430 (builder copy, dep T425)
- 260927 POST r19..r20 no-op (no code impact)
- 260927 QUOTA r21..r22 no-op (no code impact)
