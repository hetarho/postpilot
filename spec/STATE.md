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
| CLIP | 30 | 29 | CLIP-130+ CLIP-131+ CLIP-132+ CLIP-133+ CLIP-1✎ CLIP-17✎ CLIP-20✎ CLIP-35✎ CLIP-36✎ CLIP-40✎ CLIP-93✎ | 2 |
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
| T194 | The overlay and the delivery encode become one | CLIP | T191 | todo |
| T195 | A plate's sample frames come from one decode | CLIP | T191 | todo |
| T196 | Preparation decodes each original once | CLIP | - | todo |
| T197 | A cut's rate is read from its own observation | CLIP | T191 | todo |
| T198 | Speech a retained source lets through stays at 1x | CLIP | T197 | todo |

## next
- create-task CLIP — r30 moves every setting but title/template/ratio into ①, gives ② a written revision request that rewrites the current plan for one writing call, and keeps every instruction and request verbatim; CLIP-93's instruction clause is the one that fixes today's defect, where a changed instruction reuses the old plan because neither the plan-reuse nor the quote digest carries it.
- implement-task T194, then T196, then T197 T198 — T194/T196 carry the rest of the 1225 s a prod generation took (T193 already collapsed the merge passes), measured by T190's per-operation durations against T191's identity baseline (docker --target identity-smoke) that none of them may move; T197/T198 are r29's rate work and move delivered frames on purpose.
- T177 is blocked on the owner's viewing answers; T008 stays owner-dependent.
## log
- 260916 T193 done; the cuts merge in rounds of six instead of a pairwise tree (100 cuts 98 passes→2, ordinary plans none at all) and the delivered clip still matches T191's baseline
- 260916 update-ssot CLIP done; CLIP@30 — the creation screen keeps only title/template/ratio, ② gains a written revision request charged as one writing call and stopping at the plan, every instruction and request is kept verbatim, and an instruction change now invalidates the candidate plan
- 260916 T193 claimed (perf)
- 260916 T192 done; decoding takes 2 threads while every encoder and the filters stay at 1, the delivered clip still matching T191's baseline and the analysis copies identical at either decoder count
- 260916 update-ssot CLIP start
- 260916 T192 claimed (perf)
- 260916 T191 done; one fixed plan renders byte-identically twice, and its digest, player-visible properties and one frame per second are pinned as the baseline T192-T195 must hold (docker target identity-smoke, 112 s)
- 260916 T191 claimed (perf)
- 260916 T190 done; every media command reports its operation, outcome and elapsed time on success too, labelled with the stage it ran in, through the sink the render substages already used
- 260916 create-task CLIP done; T197 T198 carry r29 — the observed rate rules and the 40 % share into both writing contracts, then the per-source sound setting the speech rule needs
- 260916 create-task CLIP start
- 260916 T190 claimed (perf)
- 260916 update-ssot CLIP done; CLIP@29 — a cut's rate read from its own observation, a 40 % transformed share bounding the writing, and 1x wherever a retained source lets speech through
- 260916 T191-T196 (todo) unaffected but ordered first — r29 moves delivered frames deliberately, so the CLIP-125 identity baseline must be taken against today's clip before a rate change rewrites it
- 260916 T189 done; the owner binds a whole source to an item where sources are selected, every cut inherits it, and a binding with no matching observation is ignored rather than refused
- 260916 update-ssot CLIP start
- 260916 T189 claimed (grp)
- 260916 T188 done; with an instruction present the experiential-marker check stands down on presence alone, every figure still needing a referenced fact
- 260916 T188 claimed (grp)
- 260916 T187 done; the writer reads project_instruction and the contract names it the content authority above authored guidance, structure still the template's; no instruction leaves the request byte-identical
