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
| clip-template-as-preset | converted@260917 |

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
| CLIP | 32 | 32 | - | 3 |
| CDS | 21 | 21 | - | 1 |
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
| T221 | The owner places, sizes and restyles a caption | CLIP CDS | T219 | doing@260917.plc |
| T222 | The server hands the preview the caption SVG it will render | CDS CLIP | T219 T221 | todo |
| T223 | A template carries named composition stages | CLIP | - | todo |
| T224 | ① chooses the design, the caption styles and an optional template | CLIP | T217 T218 | todo |
| T225 | ② places captions over their own cut frame | CLIP CDS | T221 T222 | todo |
| T226 | The approval surface quotes sequence-rendered captions separately | CDS CLIP | T220 | todo |

## next
- implement-task T221 → T222 → T225 (the caption placement deploy unit: store, serve, edit) — T223 T224 T226 stay free for another session
## log
- 260917 T220 done; the thirteen sequence styles draw one frame at a time into a bounded PNG sequence under the attempt workspace, enter the overlay chain through image2 at the output frame rate with no loop and no fade, are counted by CLIP-33 and deleted the moment their overlay pass is encoded, while a static style's single rasterise is untouched
- 260917 T219 done; internal/clip/design holds the sixteen approved caption styles with their faces, roles, colour treatments and motion, Jua and NanumMyeongjo ship pinned beside Pretendard and Paperlogy, resvg is handed every bundled face with system fonts off, and a caption whose face lacks a syllable falls back to the default style with a CLIP-108 notice naming it
- 260917 CDS constraints still list face_family with two faces while CDS-17 names four — worth an update-ssot
- 260917 T221 claimed (plc)
- 260917 T219 T220 adopted for closeout by plc — cap had committed both complete (7c33b5ca) with every acceptance verified, leaving only the st/archive/STATE closeout
- 260917 T220 claimed (cap)
- 260917 T219 claimed (cap)
- 260917 T218 done; a clip generates with no template at all — minting, both quote gates, the writer's own input check and every prompt stop assuming one, a project with none freezes the grammar's minimum document (one empty hook, one empty ending, nothing else) so every downstream check still reads a real document, the guide section is omitted whole rather than sent empty with the two prefixes pinned by golden fixtures, and a revision runs on a project whose template was deleted from the composition it retained
- 260917 the narration call still measures its generated region rows against the FROZEN document's preset (ai/composition_copy.go regionSelection), so a project that changes a preset can be handed slot limits the render will not use — nothing fails today, worth a review-code finding
- 260917 T217 done; the design selection is the project's (migration 0062 intro_preset/outro_preset/allowed_caption_styles, backfilled from the document the render already reads so nothing re-renders differently), seeded from the template at creation, presence-aware on update and staling only the result; EditPlan.WithCaptions became WithDesign so the compiler names every seam it has to reach, and the layout, V20 and the admission read the presets and the allowed styles from there rather than from the frozen template composition
- 260917 create-task CLIP CDS done; T217–T226 split r32/r21 into two roots — the project owning the design selection (T217) and the caption style registry with its faces and glyph fallback (T219) — then the template becoming optional (T218), sequence rendering through a temporary PNG sequence (T220), owner placement with the verifier changes (T221), the served preview fragment that keeps preview and render identical (T222), template stages (T223) and the two surfaces (T224 T225) with the sequence quote (T226); the CLIP-145 ceiling stays open and T226 only surfaces the numbers a ceiling would need
- 260917 create-task CLIP CDS start
- 260917 update-ssot CLIP r32 CDS r21 done; the video template is a preset rather than a precondition (CLIP-5 at most one), the design selection and the allowed caption styles belong to ① (CLIP-139 CLIP-142), a template may carry named composition stages that guide the flow without admitting or forbidding footage (CLIP-141), and ② places each caption over its own cut frame for free movement inside the safe area with contrast demoted to a notice (CLIP-143 CDS-82 CDS-52); the caption style set, its static/sequence cost split and the preview-render agreement rule are new in CDS (CDS-80 CDS-81 CDS-83)
- 260917 T177 (blocked, CDS CLIP) lies in the changed area: its release QA viewing checklist predates the caption style set
- 260917 update-ssot CLIP CDS start (ideation clip-template-as-preset)
- 260917 T204 done; what the owner asked the AI for is kept with the project (migration 0060 clip_project_requests, cascade to the project) — the instruction a generation froze and the words and target of each revision, verbatim with the time, written at the one seam where the job exists but cannot yet dispatch, so an accepted job always has its entry and a save never writes one; read back newest first in a disclosure beside the observations, and gone when the project is
- 260917 useClipProject stops polling at a terminal revise_clip job, so a revision's settlement lands only on the next read — pre-existing from T202/T203, worth a review-code finding
- 260917 T204 claimed (rui)
- 260917 T203 done; ② asks the writer for a revision from its own panel — a bounded request with its count, a target defaulting to 자막, the ceiling and its priced writing calls re-quoted whenever either changes, and the run reported in place with 취소 while the timeline goes read-only and no focused job view opens; the approval surface came down to entities/clip-project so the revision reuses it instead of writing a second one, and a flush that moves the plan re-quotes rather than sending against a ceiling nobody approved
- 260917 T203 claimed (rui)
- 260917 T202 done; a revision is the third clip job kind — quoted for the writing calls its target needs and no observation, bound to the saved plan by its own digest, reserved at the start of its run, saved with SaveRevisedPlan so the plan revision advances while the rendered one stays behind; migration 0059 lets its job consume a quote, and every clip-kind check now asks job.ClipKind
- 260917 T201 done; the two writing contracts gained a revision mode — the saved plan as the owner's edits left it plus their request, appended to the same prompts and answered on the same schemas; a flow target rewrites the footage then the narration, a narration target is told the flow is final, and the three writing calls now share one `write` helper
- 260917 T216 done; the owner arranges the footage in ① (migration 0058 position, ReorderClipSources for the whole batch or nothing, ORDER BY position then ordinal) and the flow call reads that order; the strip moves a tile by grip-drag or by two buttons with a live announcement, the item control says it is an optional hint, and the instruction help says it directs order, rhythm and what the captions say
- 260916 T209 done; the caption pace and the accent are the project's (migration 0057, seeded from the template at creation, chosen in ① and gone from ②) — the render reads them through EditPlan.WithCaptions and applies them once at layout, a change bumps only the plan revision so the result goes stale without repaying a writing call, and the accent row draws its dots from the design system's own hex because CDS's palette has no FE theme tokens
- 260916 T215 done; ② edits the narration on its own lane — one bar per caption whatever cut lies beneath, absolute start/end fields, add at the playhead into free room (≥900 ms), remove, undo/redo, an overlap or an out-of-output caption blocks 다시 렌더 without retiming anything, the three caption notices read in ko/en, and the item-binding controls left ② for ①
- 260916 T214 done; the renderer schedules the narration on the output timeline (order by start, overlap omitted, CDS-41 floor through the shorter text then the free room, owner windows untouched), places a spanning caption against every cut it covers, and verifies V18 timeline-wide, V16 against the duration and V11 on every collected fact; the identity baseline was NOT re-pinned — measured, the delivered clip is identical at the commit that recorded it, at HEAD and here, so that constant belongs to another host
