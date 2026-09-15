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
| T168 | Replace the information frame with an unplated label/value pair and retune the disclosure pill | CDS CLIP | T166 T167 | todo |
| T169 | Replace the declared styles with the design selection and migrate every stored template | CLIP CDS | T166 | doing@260915.dsg |
| T171 | Remove the style controls and draw the presets in the previews | CLIP CDS | T166 T167 T168 T169 | todo |
| T172 | Author templates design first with fixed intro and outro skeletons | CLIP CDS | T169 T171 | todo |
| T173 | Keep generated slot text to one line through the plan ladder | CDS CLIP | T167 T169 | todo |

## next
- Finish T169 (dsg); T166 tokens, T167 region rendering and T170 font validation are complete.
- Continue sequentially with T168 and T173 after T167/T169; T171 closes the FE style controls and T172 adds the design-first editor last.
- T110 stays owner-blocked and T008 owner-dependent.

## log
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
- 260915 warn: T166 T167 T168 T169 T171 (todo, base CDS@15 CLIP@22) sit inside the r16/r23 delta and must be refreshed by create-task before implementation; T170 is untouched
- 260915 update-ssot CDS CLIP start
- 260915 create-task CDS CLIP done; T166 T167 T168 T169 T170 T171 from the r15/r22 delta
- 260915 T165 done
- 260915 T164 done
- 260915 T163 done
- 260915 T165 claimed (frm); T164 parser and notice contract tests pass, coordinated release gates remain
- 260915 update-ssot CDS r15 CLIP r22 done; three fixed regions replace five caption styles, cards and pills; only the disclosure keeps a pill
- 260915 warn: T163 finished during this update and its CDS-69 frame family is retired by r15 — CDS-30's unplated label/value pair replaces it
- 260915 T164 claimed (frm); T163–T165 selected for a coordinated release
