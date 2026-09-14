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

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 2 | 2 | - | 0 |
| AUTH | 5 | 5 | - | 0 |
| QUOTA | 12 | 12 | - | 0 |
| POST | 6 | 6 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 6 | 6 | - | 0 |
| MODEL | 10 | 10 | - | 0 |
| TMPL | 6 | 6 | - | 1 |
| GUIDE | 2 | 2 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 5 | 5 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 12 | 12 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 21 | 21 | - | 2 |
| CDS | 14 | 14 | - | 2 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | ready@260914 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T110 | Release QA on the Naver app and the overlay measurement | CDS CLIP | T108 | blocked@260912 |
| T162 | Rank automatic anchors by the observed caption-safe regions | CDS CLIP | T161 | doing@260915.cap |
| T163 | Replace the information pill with a product-drawn frame family | CDS | - | todo |
| T164 | Repair or narrow a generated plan and record its notices instead of refusing it | CLIP | - | todo |
| T165 | Show a delivered clip's notices with the result, in step ② and at finalization | CLIP | T164 | todo |

## next
- implement-task T164 — the ladder and the stored notices; T165 surfaces them and waits on it.
- T162 implements observed-space anchor ranking (cap); T159 T160 T161 are complete.
- T163 remains a separate information-frame design change; T110 stays owner-blocked and T008 owner-dependent.

## log
- 260915 T160 done
- 260915 T162 claimed (cap); base refreshed to CLIP@21 because the notice-policy delta does not change anchor ranking
- 260915 T161 done
- 260915 T159 done
- 260915 T160 T161 refreshed to CLIP@21 (cap); deliver-with-notice decisions do not change observation language or factual caption-safe regions
- 260915 create-task CLIP done; T164 T165 from the r21 delta (tolerance ladder, then its surface)
- 260915 T161 claimed (cap)
- 260915 T160 claimed (cap)
- 260915 T159 claimed (cap)
- 260915 update-ssot CLIP r21 done; deliver-with-notice tolerance, retries narrowed to unreadable responses, notices stored with the project; CDS unchanged — notices are product surface, not rendered output
- 260915 warn: T160 T161 are doing on CLIP but sit outside the r21 decisions (observation language, caption-safe regions); no refresh needed
- 260915 create-task CLIP CDS done; T159 T160 T161 T162 T163 from the r20/r14 delta
- 260915 update-ssot CLIP r20 CDS r14 done; observation language, caption-safe regions, information frame family, pace-independent caption sequencing; free vertical placement dropped by the owner
- 260915 T157 done (vis); refused saves retain actionable page-level reasons across steps; retry backoff preserved
- 260915 T157 claimed (vis)
- 260915 T156 done (vis); all 97 public checks explained beside failures in ko/en
- 260915 T156 claimed (vis)
- 260915 T155 done (grp); named controls, minimum display without opening writes, maximum and editor round trips verified
- 260915 T155 claimed (grp)
- 260915 T154 done (grp); compatible group declarations and pre-work admission verified
