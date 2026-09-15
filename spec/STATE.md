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
| CLIP | 24 | 24 | - | 2 |
| CDS | 18 | 18 | - | 1 |
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
| T177 | Release QA viewing checklist | CDS CLIP | T176 | todo |

## next
- implement-task T177 with the owner watching.
- T008 stays owner-dependent.

## log
- 260916 T176 done; one shared offset moves a region block as one piece, real-ink overlap and safe-area pinned, eight goldens and the frontend mirror regenerated
- 260916 T176 claimed (rgn)
- 260915 create-task CDS done; T176 from the r18 delta and T177 carrying CDS-53's viewing checklist from the void T110
- 260915 create-task CDS start
- 260915 update-ssot CDS done; CDS@18 — a region block keeps its 9:16 spacing and moves only its centre
- 260915 update-ssot CDS start
- 260915 T175 done; ratio guidance and the design spec reference no longer name one platform
- 260915 T175 claimed (wrd)
- 260915 T174 done; 9:16 centres on the canvas, SA-N retired, goldens and the frontend mirror regenerated
- 260915 T174 claimed (sym)
- 260915 create-task CDS CLIP done; T174 T175 from the CDS@17 CLIP@24 delta, T110 retired void with CDS-10 CDS-11
- 260915 create-task CDS CLIP start
- 260915 update-ssot CDS CLIP done; CDS@17 CLIP@24 — 9:16 safe area and CENTER/RIGHT anchors symmetric, SA-N retired, clip defined platform-neutral
- 260915 warn T110 (blocked) targets the retired CDS-10 CDS-11 and needs retiring in create-task
- 260915 update-ssot CDS CLIP start
- 260915 T172 done; design-first authoring, lossless skeleton rebuilding, shared examples and final frontend/backend checks pass
- 260915 T171 done; canvas font metadata now passes the style scanner, browser check and final build
- 260915 T171 reopened (dsg); final style scanner flags canvas attribute reads, fix before committing T172
- 260915 T171 done; style controls removed, preset previews and all corrected frontend regression gates pass
- 260915 T172 claimed (dsg); T171 implementation is frozen for verification
