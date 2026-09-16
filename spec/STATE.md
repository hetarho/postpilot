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
| CLIP | 29 | 29 | - | 2 |
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
| T192 | Decoding uses the cores the encode cannot | CLIP | T191 | todo |
| T193 | The cuts become one timeline in a single pass | CLIP | T191 | todo |
| T194 | The overlay and the delivery encode become one | CLIP | T191 | todo |
| T195 | A plate's sample frames come from one decode | CLIP | T191 | todo |
| T196 | Preparation decodes each original once | CLIP | - | todo |
| T197 | A cut's rate is read from its own observation | CLIP | T191 | todo |
| T198 | Speech a retained source lets through stays at 1x | CLIP | T197 | todo |

## next
- implement-task T193, then T194 T196 — they carry most of the 1225 s a prod generation took; T190's per-operation durations measure each one and T191's identity baseline (docker --target identity-smoke) is what none of them may move.
- implement-task T197, then T198 — r29's rate work, held behind T191 because it moves delivered frames on purpose.
- T177 is blocked on the owner's viewing answers; T008 stays owner-dependent.
## log
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
- 260916 T187 claimed (grp)
- 260916 T186 done; a clip project stores one bounded instruction beside its answers and freezes it into every attempt (docker came back, so proto/sqlc were regenerated properly)
- 260916 T186 claimed (grp)
- 260916 T185 done; the writer is told which sections this project's answers admit and how many instances each has, the cut ceiling unchanged (owner's choice — update-ssot candidate on CLIP-103)
- 260916 T185 claimed (grp)
- 260916 T184 done; a validated plan under the 15 s floor fails as CLIP_INSUFFICIENT_FOOTAGE with its own plan_length_floor check, never as an unreadable response
