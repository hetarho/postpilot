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
| CLIP | 28 | 28 | - | 2 |
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
| T186 | A project carries its own instruction | CLIP | - | todo |
| T187 | The writer reads the instruction and lets it outrank guidance | CLIP | T186 | todo |
| T188 | An instruction lets the writer speak from experience | CLIP | T186 | todo |
| T189 | The owner binds a source to an item before generating | CLIP | - | todo |
| T190 | Media-operation durations for successful work | CLIP | - | todo |
| T191 | A delivered-clip identity baseline the speed work must hold | CLIP | - | todo |
| T192 | Decoding uses the cores the encode cannot | CLIP | T191 | todo |
| T193 | The cuts become one timeline in a single pass | CLIP | T191 | todo |
| T194 | The overlay and the delivery encode become one | CLIP | T191 | todo |
| T195 | A plate's sample frames come from one decode | CLIP | T191 | todo |
| T196 | Preparation decodes each original once | CLIP | - | todo |

## next
- implement-task T191, then T190 — T191 gates T192 T193 T194 T195 and T190 makes each one's effect measurable; T193 T194 T196 carry most of the 1225 s a prod generation took.
- implement-task T186, then T187 T188 — the instruction path the owner is waiting on; T189 removes the binding failure that emptied every meat caption.
- T177 is blocked on the owner's viewing answers; T008 stays owner-dependent.
## log
- 260916 T185 done; the writer is told which sections this project's answers admit and how many instances each has, the cut ceiling unchanged (owner's choice — update-ssot candidate on CLIP-103)
- 260916 T185 claimed (grp)
- 260916 T184 done; a validated plan under the 15 s floor fails as CLIP_INSUFFICIENT_FOOTAGE with its own plan_length_floor check, never as an unreadable response
- 260916 flake: internal/clip/store fails one differing test per full-suite run on clip source expiry (reproduced on unmodified HEAD) — review-code candidate
- 260916 T184 claimed (grp)
- 260916 T183 done; one effective minimum per group in both grammars, admission refusing an empty required group by name with its counts, and the owner's controls opened there
- 260916 create-task CLIP done; T190 T191 instrument duration and delivered-clip identity, T192 T193 T194 T195 T196 carry r28's pass budget
- 260916 prod measurement behind r28: generation 1225 s — prepare 285, analyze 69, plan 13, render 857 (encode 461, overlay 304); the box is one physical core
- 260916 create-task CLIP start
- 260916 update-ssot CLIP done; CLIP@28 — no repeated full-resolution pass, an identical clip from any speed change, and one read/decode per original
- 260916 T177 (blocked) unaffected — CLIP-125 holds the delivered clip identical, so its rendered review clips stay valid
- 260916 update-ssot CLIP start
- 260916 T183 claimed (grp)
- 260916 CLIP-64 defect fixed outside the task list; an unassigned cut now omits only text bound to its group's fields, so a caption citing no item fact survives (BE verify green)
- 260916 create-task CLIP done; T183 T184 T185 carry the r26 delta, T186 T187 T188 the project instruction and T189 owner source binding
- 260916 create-task CLIP start
- 260916 update-ssot CLIP done; CLIP@27 — a project instruction outranks authored guidance on content and a source can be bound to an item before generation
- 260916 T177 (blocked) unaffected — it reads CLIP-27 download shape, not the writer's inputs
- 260916 update-ssot CLIP start
- 260916 T182 done; the writer is told each declared maximum and an over-long answer takes the shorten-then-omit ladder
