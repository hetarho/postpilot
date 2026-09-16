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
| ARCH | 3 | 2 | ARCH-36+ ARCH-37+ ARCH-38+ ARCH-39+ | 0 |
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
| CLIP | 31 | 30 | CLIP-134+ CLIP-135+ CLIP-136+ CLIP-137+ CLIP-138+ CLIP-139+ CLIP-140+ CLIP-1✎ CLIP-4✎ CLIP-11✎ CLIP-14✎ CLIP-15✎ CLIP-17✎ CLIP-31✎ CLIP-45✎ CLIP-59✎ CLIP-61✎ CLIP-63✎ CLIP-64✎ CLIP-65✎ CLIP-66✎ CLIP-67✎ CLIP-72✎ CLIP-90✎ CLIP-97✎ CLIP-111✎ CLIP-112✎ CLIP-113✎ CLIP-121✎ CLIP-123✎ CLIP-130✎ CLIP-131✎ CLIP-132✎ CLIP-62x CLIP-103x | 2 |
| CDS | 20 | 19 | CDS-1✎ CDS-15✎ CDS-27✎ CDS-37✎ CDS-38✎ CDS-41✎ CDS-42✎ CDS-43✎ CDS-44✎ CDS-45✎ CDS-52✎ CDS-53✎ CDS-56✎ CDS-57✎ CDS-59✎ CDS-60✎ CDS-61✎ CDS-62✎ CDS-63✎ CDS-30x | 1 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T177 | Release QA viewing checklist | CDS CLIP | T176 | blocked@260916 |
| T197 | A cut's rate is read from its own observation | CLIP | T191 | todo |
| T198 | Speech a retained source lets through stays at 1x | CLIP | T197 | todo |
| T200 | The creation screen settles only the ratio | CLIP | - | todo |
| T201 | The writer rewrites a saved plan from a written request | CLIP | - | todo |
| T202 | A revision request runs as its own job and charges one writing call | CLIP | T201 | todo |
| T203 | ② asks for a revision in its own panel | CLIP | T202 | todo |
| T204 | What the owner asked for is kept | CLIP | T202 | todo |

## next
- create-task CLIP then CDS — r31/r20 shrink the template to what every clip must carry, write the flow and then a narration of captions on the absolute output timeline in two calls, ground numbers on collected facts only, retire the information pair and record no notice for the writer's choices; T197 T198 (r29 single writing call) and T200-T204 (r30 whole-plan revision, one call) must be re-cut against r31 before implementing, T199 stands.
- r28's pass budget is done: T190-T196 all landed against T191's identity baseline (docker --target identity-smoke), which none of them moved. T197 T198 and T200-T204 are held for create-task against r31; T177 is blocked on the owner's viewing answers and T008 stays owner-dependent.
- create-task ARCH — r3 sets the deploy smoke gate: the smokes stay image stages outside ARCH-26 (ARCH-36 ARCH-37), they move beside the deploy until closed beta (ARCH-38), and the bundled ffmpeg is checked against the names the render code emits (ARCH-39); both tasks wait on T198.
## log
- 260916 T195 done; every element's CDS-44 frames come from one read of the composed footage (output-side seeks select the same frames), the measurements and the delivered clip unchanged
- 260916 T199 done; the owner instruction now rides both reuse digests, so a changed instruction re-plans on the stored observations instead of re-rendering the plan written without it
- 260916 T195 claimed (perf)
- 260916 update-ssot CDS done; CDS@20 — rhythm and voice follow the instruction, captions hold disjoint absolute windows on the output timeline whatever cut lies beneath, the information pair is retired, numbers match any collected fact, and the accent is chosen in ①
- 260916 T197 T198 (base CLIP@29) and T200-T204 (base CLIP@30) affected — r31 rebuilds the writing contract into two calls and targets the revision request; re-cut under create-task before implementing
- 260916 update-ssot CLIP done; CLIP@31 — the template shrinks to what every clip must carry (design, slots, badge, fields, groups, optional guide), the instruction directs footage order and narration, two writing calls write the flow and then a narration of captions on the absolute output timeline, numbers ground on collected facts only, and an unwritten caption is no notice
- 260916 T196 done; the analysis copies carry the source's verification output from their own decode, so an original is decoded once in preparation (partial-recovery reuse still falls back to the separate pass)
- 260916 T199 claimed (inst)
- 260916 create-task ARCH start
- 260916 update-ssot ARCH done; ARCH@3 — the media smokes stay image stages a media task runs locally before done, they move beside the deploy until closed beta, and the bundled ffmpeg is checked against the filter/codec/muxer names the render code emits; review clip-release-smoke-260914 F2 closed by ARCH-37
- 260916 create-task CLIP done; T199-T204 carry r30 — the instruction into both reuse digests, the creation screen down to title/template/ratio, then the revise contract, its job and credits, ②'s request panel and the kept record of what was asked for
- 260916 update-ssot ARCH start (deploy smoke gate)
- 260916 T196 claimed (perf)
- 260916 T194 done; a single-window clip is overlaid and delivered in one encode with no lossless intermediate between them, the delivered clip still matching T191's baseline
- 260916 create-task CLIP start
- 260916 T194 claimed (perf)
- 260916 T193 done; the cuts merge in rounds of six instead of a pairwise tree (100 cuts 98 passes→2, ordinary plans none at all) and the delivered clip still matches T191's baseline
- 260916 update-ssot CLIP done; CLIP@30 — the creation screen keeps only title/template/ratio, ② gains a written revision request charged as one writing call and stopping at the plan, every instruction and request is kept verbatim, and an instruction change now invalidates the candidate plan
- 260916 T193 claimed (perf)
- 260916 T192 done; decoding takes 2 threads while every encoder and the filters stay at 1, the delivered clip still matching T191's baseline and the analysis copies identical at either decoder count
