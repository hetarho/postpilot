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
| CLIP | 22 | 22 | - | 2 |
| CDS | 15 | 15 | - | 2 |
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
| T166 | Collapse the style system to one caption treatment and add the region tokens | CDS | - | todo |
| T167 | Render the intro and outro blocks instead of the hook and ending cards | CDS | T166 | todo |
| T168 | Replace the information frame with an unplated label/value pair and retune the disclosure pill | CDS | T166 | todo |
| T169 | Stop accepting a declared style and migrate every stored template | CDS CLIP | T166 | todo |
| T170 | Fail when a text renders in a face it did not ask for | CDS | T166 | todo |
| T171 | Remove the style controls and draw the regions in the preview | CLIP CDS | T166 T167 T168 | todo |

## next
- implement-task T166 — every other task in this batch waits on its constants.
- T167 T168 T169 T170 run in parallel after it; T171 closes the surfaces last.
- T163's drawn frame family is removed by T168, retired by r15 before it ever rendered.
- T110 stays owner-blocked and T008 owner-dependent.

## log
- 260915 create-task CDS CLIP done; T166 T167 T168 T169 T170 T171 from the r15/r22 delta
- 260915 T165 done
- 260915 T164 done
- 260915 T163 done
- 260915 T165 claimed (frm); T164 parser and notice contract tests pass, coordinated release gates remain
- 260915 update-ssot CDS r15 CLIP r22 done; three fixed regions replace five caption styles, cards and pills; only the disclosure keeps a pill
- 260915 warn: T163 finished during this update and its CDS-69 frame family is retired by r15 — CDS-30's unplated label/value pair replaces it
- 260915 T164 claimed (frm); T163–T165 selected for a coordinated release
- 260915 T163 claimed (frm)
- 260915 T162 done; T159–T162 form the verified caption-placement release
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
