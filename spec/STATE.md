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
| CLIP | 23 | 23 | - | 2 |
| CDS | 16 | 16 | - | 2 |
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
| T172 | Author templates design first with fixed intro and outro skeletons | CLIP CDS | T169 T171 | doing@260915.dsg |

## next
- Finish T172 (dsg); T166–T171 and T173 are complete.
- Finish the design-first editor verification and commit T172 separately.
- T110 stays owner-blocked and T008 owner-dependent.

## log
- 260915 T171 done; canvas font metadata now passes the style scanner, browser check and final build
- 260915 T171 reopened (dsg); final style scanner flags canvas attribute reads, fix before committing T172
- 260915 T171 done; style controls removed, preset previews and all corrected frontend regression gates pass
- 260915 T172 claimed (dsg); T171 implementation is frozen for verification
- 260915 T173 done; grounded single-line slot repair, owner notices and backend/frontend gates pass
- 260915 T171 claimed (dsg); T173 implementation is frozen for verification
- 260915 T168 done; unplated information, fixed disclosure height and sampled contrast notices verified
- 260915 T173 claimed (dsg); T168 implementation is frozen for verification
- 260915 T169 done; strict/tolerant grammar, byte-preserving migration and retired style permissions verified
- 260915 T168 claimed (dsg); T169 implementation is frozen for verification
- 260915 T167 done; twelve region/font fixtures and backend gates pass; one clock-reversal failure passed three isolated repeats
- 260915 T169 claimed (dsg); T167 implementation is frozen for verification
- 260915 T170 done; named-face startup guard and full backend gates pass
- 260915 T167 claimed (dsg); T170 implementation is frozen for verification
- 260915 T170 claimed (dsg)
- 260915 T166 done; backend/agent gates, frontend regression with corrected autosave assertion, lint/style/build, native code generation and real resvg smoke pass
- 260915 T166 claimed (dsg)
- 260915 create-task CDS CLIP done; T166 T167 T168 T169 T171 refreshed to CDS@16 CLIP@23 and T172 T173 added from the r16/r23 delta; T170 base only
- 260915 create-task CDS CLIP start
- 260915 update-ssot CDS r16 CLIP r23 done; intro A|B and outro B|E presets chosen per template, outro E label lifted, score tracked wider and its bar white, accent confined to the caption word, badge 36/800 pad 16/28, design-first template authoring with fixed intro/outro skeletons
