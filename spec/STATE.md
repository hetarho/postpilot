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
| CLIP | 25 | 24 | CLIP-60✎ CLIP-102✎ CLIP-116+ CLIP-117+ CLIP-118+ | 2 |
| CDS | 19 | 19 | - | 1 |
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
| T177 | Release QA viewing checklist | CDS CLIP | T176 | blocked@260916 |

## next
- create-task CLIP for the r25 character-maxima delta.
- T177 is blocked on the owner's viewing answers; the three review clips are rendered and the checklist is open. T008 stays owner-dependent.

## log
- 260916 production migration/rollback fix shipped; deploy f7362b9a green, prod health 200
- 260916 T177 blocked; three review clips rendered with every CDS-53 assembly case, checklist open, recall questions need real footage
- 260916 create-task CLIP start
- 260916 update-ssot CLIP done; CLIP@25 — a template declares character maxima and the answer form enforces the effective one
- 260916 T177 (doing) unaffected — the maxima change authoring and admission, not the render checklist
- 260916 update-ssot CLIP start
- 260916 T178 done; the disclosure is a 12 px rounded rectangle, pinned by the token test and read by both renderer and preview
- 260916 T178 claimed (bdg)
- 260916 create-task CDS r19 done; T178 carries the badge shape
- 260916 update-ssot CDS r19; T177 (doing) reviews the badge — its checklist sees the new shape
- 260916 T177 claimed (qa)
- 260916 Production recovered to ef94ac0b after DB backup (schema 53, quick_check ok); health/GetMe/CORS verified; local migration 54 + rollout fixes pass BE vet/build/tests and 5 rollout regressions, not deployed (ops).
- 260916 Production incident: investigate API 502 and failed backend rollout; repair startup/rollback and verify recovery (ops).
- 260916 T176 done; one shared offset moves a region block as one piece, real-ink overlap and safe-area pinned, eight goldens and the frontend mirror regenerated
- 260916 T176 claimed (rgn)
- 260915 create-task CDS done; T176 from the r18 delta and T177 carrying CDS-53's viewing checklist from the void T110
- 260915 create-task CDS start
- 260915 update-ssot CDS done; CDS@18 — a region block keeps its 9:16 spacing and moves only its centre
- 260915 update-ssot CDS start
- 260915 T175 done; ratio guidance and the design spec reference no longer name one platform
