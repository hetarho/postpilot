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
| ARCH | 3 | 3 | - | 0 |
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
| CLIP | 31 | 31 | - | 2 |
| CDS | 20 | 20 | - | 1 |
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
| T201 | The writer rewrites a saved plan from a written request | CLIP | T212 | todo |
| T202 | A revision request runs as its own job and charges the writing calls its target needs | CLIP | T201 T213 | todo |
| T203 | ② asks for a revision in its own panel | CLIP | T202 | todo |
| T204 | What the owner asked for is kept | CLIP | T202 | todo |
| T209 | Caption pace and accent are project settings | CLIP CDS | - | todo |
| T211 | Writing call 1 writes the footage flow | CLIP CDS | T210 | todo |
| T212 | Writing call 2 writes the narration | CLIP CDS | T211 | todo |
| T213 | A generation runs the flow call and then the narration call | CLIP | T212 | todo |
| T214 | The renderer schedules captions across the whole timeline | CLIP CDS | T210 | todo |
| T215 | ② edits captions on their own track | CLIP CDS | T210 | todo |
| T216 | ① arranges the sources and hints the writer | CLIP | - | todo |

## next
- implement-task T211 next (dep T210 done), with T214, T215, T209 and T216 free to run beside it; then T212 → T213, and the revision tasks T201 → T202 → T203/T204 after T212/T213 — every writing-contract change lives in T211/T212, so nothing else should touch ai/ prompts; T209, T214, T215 and T216 all change clip.proto, so run them one after another in one tree
- T200 stands alone; ARCH's T205 T206 (deploy smoke gate) wait on T211, which absorbed T198; T177 is blocked on the owner's viewing answers; T008 stays owner-dependent
## log
- 260916 T210 done; a plan carries narration captions with absolute, disjoint output intervals beside the template regions — the shape is validated at store time and again against the duration an edit produces, so footage edits never retime a caption and a caption the new output cannot hold is named for correction; ② may add, edit and remove one (server-minted `narration-N`, owner-written text skips grounding), `GroundNarration` drops the cross-item and context-item rules, and the three caption removal reasons exist (`caption_floor` has no producer until T212/T214)
- 260916 T208 done; the template editor offers only fields, groups, guides and the badge, the accent and pace selects are gone, a converted legacy template says its scenes moved into the guide and saves that body, and the preview supplies one sample caption line; the converter now lifts an intro/outro authored inside a scene and keeps carrying prose when the body still has an authoring error
- 260916 T200 done; a project is minted from title/template/ratio and everything else is written in ① beside the sources — migration 0056 relaxes the duration CHECK (goose NO TRANSACTION, foreign keys off), and the upload gate now asks for a savable project rather than a complete one
- 260916 T208 claimed (tmpl)
- 260916 T207 done; a template body must satisfy ParseTemplate (no scene, repeat, caption/info text or cut basis), frozen snapshots still read through Parse/ReadStored, a legacy template reads back converted into one guide with composition_converted=true and its stored body untouched, the FE parser/guide/skeleton mirror it through the shared corpus; media (untracked tooling.go, T206) and pages/clip (T200's form rewrite) suites fail in the shared tree independently of this task
- 260916 T206 done; the media package declares the filters, decoders, encoders and muxers it names, the image proves the bundled ffmpeg carries them in 0.01 s before the deploy pushes it, and the graph fixtures keep the list honest without Docker
- 260916 T206 claimed (dply)
- 260916 T205 done; the pushed image is the `runtime` target and the rollout no longer waits — the smokes run on the same commit in their own job and still fail the run, with a deploy check that catches the gate being removed rather than moved
- 260916 T207 claimed (tmpl)
- 260916 T205 claimed (dply)
- 260916 T205 dep T211→- ; the original dep:T198 meant "after the pass-budget work that changes ffmpeg's filter set" (T193 T194), which is done, and r31's re-cut moved it onto a writing-call task T205 does not touch
- 260916 T200 claimed (mint)
- 260916 T205-T214 (CLIP, this session) renumbered to T207-T216 — create-task ARCH minted T205 T206 at the same moment; ARCH T205 dep T198→T211 since T198 was folded into T211
- 260916 create-task CLIP done; T207-T216 carry r31 — the template grammar shrinks to fixed regions with legacy conversion, pace/accent move to the project, the plan gains a narration, two writing calls (flow, narration) replace the single writer, the renderer and ② schedule captions on the whole timeline, ① orders sources; T197 T198 folded into T211 (numbers retired), T201-T204 re-cut for the targeted revision, T200 rebased; CDS r20 is consumed by T207 T209 T210 T211 T212 T214 T215
- 260916 create-task ARCH done; T205 T206 carry r3 — the smokes move beside the deploy and the bundled ffmpeg is checked against the names the render code emits; ARCH-36 ARCH-37 are no-op (they state what already holds and what a task owes before done, no code follows)
- 260916 T195 done; every element's CDS-44 frames come from one read of the composed footage (output-side seeks select the same frames), the measurements and the delivered clip unchanged
- 260916 T199 done; the owner instruction now rides both reuse digests, so a changed instruction re-plans on the stored observations instead of re-rendering the plan written without it
- 260916 T195 claimed (perf)
- 260916 update-ssot CDS done; CDS@20 — rhythm and voice follow the instruction, captions hold disjoint absolute windows on the output timeline whatever cut lies beneath, the information pair is retired, numbers match any collected fact, and the accent is chosen in ①
- 260916 T197 T198 (base CLIP@29) and T200-T204 (base CLIP@30) affected — r31 rebuilds the writing contract into two calls and targets the revision request; re-cut under create-task before implementing
