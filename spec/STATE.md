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
| post-quality-and-related-links | open@260918 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 7 | 7 | - | 0 |
| AUTH | 7 | 7 | - | 0 |
| QUOTA | 19 | 19 | - | 0 |
| POST | 9 | 9 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 7 | 7 | - | 0 |
| MODEL | 13 | 13 | - | 0 |
| TMPL | 8 | 8 | - | 1 |
| GUIDE | 3 | 3 | - | 0 |
| EXPORT | 5 | 5 | - | 0 |
| PUB | 6 | 6 | - | 0 |
| LANG | 4 | 4 | - | 0 |
| THEME | 17 | 15 | THEME-19✎ | 0 |
| MKT | 6 | 6 | - | 0 |
| VIDEO | 3 | 3 | - | 0 |
| CLIP | 43 | 40 | CLIP-13✎ | 1 |
| CDS | 25 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 1 | 1 | - | 2 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |
| arch-260919 | converted@260919 |
| publishing-260922 | converted@260922 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | Superseded by PUB r6; original live-smoke claim retained, do not resume | PUB MKT | T007 T046 | doing@260910.e2e |
| T177 | Release QA viewing checklist | CDS CLIP | T176 | blocked@260916 |
| T279 | Pin enum mirrors in the surviving backend and frontend | ARCH | T313 | todo |
| T282 | Stop and uninstall existing Mac publishing companions | PUB ARCH | T311 | todo |
| T283 | Remove publishing backend, deletion guards and database tables | PUB POST AUTH QUOTA LANG ARCH | T312 | todo |
| T284 | Delete companion source, publishing contracts and build integration | PUB AUTH LANG VIDEO ARCH | T283 | todo |
| T311 | Remove publishing screens, polling and product promises | PUB EXPORT POST MKT QUOTA LANG VIDEO ARCH | T310 | todo |
| T312 | Export retirement outcomes and purge publishing-owned data | PUB ARCH | T282 | todo |
| T313 | Verify complete retirement and preserved manual publishing workflow | PUB EXPORT POST MKT AUTH ARCH | T284 | todo |

## next
- implement-task T311; retirement order T311 → T282 → T312 → T283 → T284 → T313; T008 must not resume, T279 follows removal
- create-task MKT THEME for the existing /about header overflow at 320px/200% text
- create-task CLIP CDS THEME for the r41/r24 Wanted Sans delta, which T305..T309 did not consume
## log
- 260922 T310 done; migration 0072 permanently refuses automatic publishing, revokes capabilities, conservatively settles jobs and exports a private retirement report; backend and spec gates pass (ret)
- 260922 T310 claimed (ret)
- 260922 retirement planning validation passed: 10 SSOT revisions/change entries, 8 task references/bases, acyclic dependencies and STATE consistency; T008 and unrelated pending preserved; review adoption synchronized; implementation not started
- 260922 create-task retirement complete: T310/T311/T312/T313 created, T282/T283/T284 repurposed for removal, T279 narrowed to surviving enums; all 10 retirement SSOT deltas consumed, unrelated pending retained
- 260922 TMPL r8 consumed without a standalone implementation task: only removes the retired-agent dependency from the still-open photo-row question; PUB-49 is covered by T313
- 260922 create-task PUB ARCH AUTH QUOTA POST EXPORT MKT VIDEO TMPL LANG start; plan phased removal and replace obsolete todo agent refactors
- 260922 update-ssot PUB ARCH AUTH QUOTA POST EXPORT MKT VIDEO TMPL LANG complete; phased retirement adopted, content/manual export preserved; create-task pending
- 260922 warning: T008 doing claim is superseded by PUB r6; do not resume live publishing or use T008 as a retirement dependency; its task file remains immutable
- 260922 create-architecture ARCH retirement alignment start; remove the companion architecture through the PUB cutover stages
- 260922 update-ssot PUB and dependent domains start; approved phased retirement of automatic publishing, followed by create-task planning
- 260922 review-code publishing-260922 complete; recommend retiring automatic Naver publish and retaining export, based on official automation restrictions and missing live acceptance; no adoption or implementation
- 260922 review-code publishing-260922 start; assess publishing viability and Naver automation/account restrictions
- 260922 T309 done; a browser render draws each sequence caption from the server's own frame, refusing by name when it cannot; real Chrome decoded a sheet, cropped cells and encoded them on all three canvases (vid)
- 260922 T308 done; one RPC serves a sequence caption's frames as bounded sprite sheets, one resvg run per sheet; backend gates, proto regeneration and the media-smoke image stage pass (vid)
- 260922 T309 claimed while T308's media smoke builds; T307 is done and T308's code is verified but for it (vid)
- 260922 T306 done; a rapid phrase keeps its style's drawing, the template refuses a sequence style, V9 checks the drawing; backend gates and the media-smoke image stage pass (vid)
- 260922 T308 claimed (vid)
- 260922 T307 done; a browser render now moves every caption its style declares, the static ones included; 2,295 frontend tests and every gate pass (vid)
- 260922 T307 claimed beside T306, whose media smoke is building (vid)
- 260922 T306 claimed (vid)
