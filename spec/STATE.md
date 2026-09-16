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
| T203 | ② asks for a revision in its own panel | CLIP | T202 | todo |
| T204 | What the owner asked for is kept | CLIP | T202 | todo |

## next
- implement-task T203 next (dep T202 done), then T204; nothing else is free — T209, T214, T215 and T216 all change clip.proto, so run them one after another in one tree; then the revision tasks T201 → T202 → T203/T204 (deps T212/T213 done)
- T200 and ARCH's T205 T206 are done; T177 is blocked on the owner's viewing answers; T008 stays owner-dependent
## log
- 260917 T202 done; a revision is the third clip job kind — quoted for the writing calls its target needs and no observation, bound to the saved plan by its own digest, reserved at the start of its run, saved with SaveRevisedPlan so the plan revision advances while the rendered one stays behind; migration 0059 lets its job consume a quote, and every clip-kind check now asks job.ClipKind
- 260917 T201 done; the two writing contracts gained a revision mode — the saved plan as the owner's edits left it plus their request, appended to the same prompts and answered on the same schemas; a flow target rewrites the footage then the narration, a narration target is told the flow is final, and the three writing calls now share one `write` helper
- 260917 T216 done; the owner arranges the footage in ① (migration 0058 position, ReorderClipSources for the whole batch or nothing, ORDER BY position then ordinal) and the flow call reads that order; the strip moves a tile by grip-drag or by two buttons with a live announcement, the item control says it is an optional hint, and the instruction help says it directs order, rhythm and what the captions say
- 260916 T209 done; the caption pace and the accent are the project's (migration 0057, seeded from the template at creation, chosen in ① and gone from ②) — the render reads them through EditPlan.WithCaptions and applies them once at layout, a change bumps only the plan revision so the result goes stale without repaying a writing call, and the accent row draws its dots from the design system's own hex because CDS's palette has no FE theme tokens
- 260916 T215 done; ② edits the narration on its own lane — one bar per caption whatever cut lies beneath, absolute start/end fields, add at the playhead into free room (≥900 ms), remove, undo/redo, an overlap or an out-of-output caption blocks 다시 렌더 without retiming anything, the three caption notices read in ko/en, and the item-binding controls left ② for ①
- 260916 T214 done; the renderer schedules the narration on the output timeline (order by start, overlap omitted, CDS-41 floor through the shorter text then the free room, owner windows untouched), places a spanning caption against every cut it covers, and verifies V18 timeline-wide, V16 against the duration and V11 on every collected fact; the identity baseline was NOT re-pinned — measured, the delivered clip is identical at the commit that recorded it, at HEAD and here, so that constant belongs to another host
- 260916 a caption over readable_text footage has no admissible anchor (readable allows top/bottom, the caption rule offers upper_mid/lower_mid) and is always dropped with copy_limit — pre-existing, found under T214, worth a review-code finding
- 260916 T213 done; a generation quotes, reserves, runs and resumes TWO writing calls — pricing v3 carries Narration beside Plan with SkipFlow/SkipNarration, the run stages are flow → narrate → layout → render, the written flow is kept with FlowReady so a narration failure resumes on it for one call, the quote lists both writing lines by label and ② shows them; the job reservation's writing line now admits two calls, and preparation measures the larger of the two requests
- 260916 T212 done; the narration call writes captions on absolute output intervals over the resolved flow plus the template's generated slot rows (schemas/narration.schema.json, narrationPrompt) — admitted in start order against the output, disjoint, within CDS-25, grounded by GroundNarration on every collected fact, CDS-41's reading time through the shorter sentence then the free room then caption_floor, with server-minted narration-N ids and no notice for anything the writer simply did not say
- 260916 T211 done; the flow call writes the footage flow alone (schemas/flow.schema.json, flowPrompt) from the instruction, the template guide, the facts, the source order, the item hints and the observations, and the server resolves it into cuts plus the template's fixed regions; the rate contract now names the observed facts it is read from, states the 40% share and the speech rule as WRITING bounds the render never re-enforces, and Plan is refused a composition — 28 tests of the retired single-writer contract were removed (owner-approved) to return as narration tests in T212
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
