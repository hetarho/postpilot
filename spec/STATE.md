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
| T223 | A template carries named composition stages | CLIP | - | doing@260917.stg |
| T224 | ① chooses the design, the caption styles and an optional template | CLIP | T217 T218 | todo |
| T226 | The approval surface quotes sequence-rendered captions separately | CDS CLIP | T220 | todo |

## next
- implement-task T224 (① chooses the design, the styles and the template — T222 now serves the previews its impl notes want) or T226 (the sequence quote); T223 is taken by another session
## log
- 260917 T225 done; ② places each caption over a still frame of the cut it starts in — drag stopping at the safe area it draws while moving, arrow-key nudging in canvas pixels, the size refused outside CDS-3's floor and the role's own size at the control, the style taken from the project's allowed set, a contrast shortfall shown in place and blocking nothing, and a plain ground with the reason where the footage is not here — every change riding the existing draft queue and undo
- 260917 T223 claimed (stg)
- 260917 T225 claimed (plc)
- 260917 T222 done; GetClipCaptionPreview hands ② each caption as the renderer's own drawing with its root taken off — one `<g>` brought to the origin by a transform the reported box cancels exactly, ids prefixed per caption, a sequence style labelled as one representative frame — and a smoke contract rasterises the placed fragment and the renderer's caption to byte-identical PNGs; Jua and NanumMyeongjo now reach the browser too
- 260917 T222 claimed (plc)
- 260917 T221 done; a caption carries the owner's own position, size and style — clamped into the safe area by moving, never resizing, with the size floor and an unallowed style refused where they are written — and the manifest says who placed it so V1 still holds it inside the safe area, V3 demotes a shortfall under it to a notice, V13 leaves it out of the anchor walk and no repair or automatic placement runs over it again
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
