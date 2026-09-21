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
| ARCH | 6 | 6 | - | 0 |
| AUTH | 6 | 6 | - | 0 |
| QUOTA | 18 | 18 | - | 0 |
| POST | 8 | 8 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 7 | 7 | - | 0 |
| MODEL | 13 | 13 | - | 0 |
| TMPL | 7 | 7 | - | 1 |
| GUIDE | 3 | 3 | - | 0 |
| EXPORT | 4 | 4 | - | 0 |
| PUB | 5 | 5 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 17 | 15 | THEME-19✎ | 0 |
| MKT | 5 | 5 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
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

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T308 | the server serves a sequence caption's own frames | CLIP CDS | - | doing@260922.vid |
| T309 | a browser render draws sequence captions from the server's frames | CLIP CDS | T307 T308 | todo |
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T177 | Release QA viewing checklist | CDS CLIP | T176 | blocked@260916 |
| T279 | every hand-kept enum mirror is pinned to the generated enum | ARCH | T008 | todo |
| T282 | the agent maps proto at one adapter and keeps preflight out of main | ARCH | T008 | todo |
| T283 | SmartEditor scripts are files with a DOM test; naver is plan vs driver | ARCH | T282 | todo |
| T284 | the agent generates only the protos it uses and drops the survey harness | ARCH | T008 T281 | todo |

## next
- implement-task T306, then T307; T308 before T309; T008 and T177 stand as they were
- create-task MKT THEME for the existing /about header overflow at 320px/200% text
- create-task CLIP CDS THEME for the r41/r24 Wanted Sans delta, which T305..T309 did not consume
## log
- 260922 T306 done; a rapid phrase keeps its style's drawing, the template refuses a sequence style, V9 checks the drawing; backend gates and the media-smoke image stage pass (vid)
- 260922 T308 claimed (vid)
- 260922 T307 done; a browser render now moves every caption its style declares, the static ones included; 2,295 frontend tests and every gate pass (vid)
- 260922 T307 claimed beside T306, whose media smoke is building (vid)
- 260922 T306 claimed (vid)
- 260922 T305 done; a finalized clip opens ① and ② as readings, the server serves its plan and evidence again, and every write still refuses; frontend 2,295 tests and backend gates pass (vid)
- 260922 CLIP r43: a finalized ① cannot state models the project never recorded; T305 carries the amended CLIP-160 (vid)
- 260922 T305 claimed (vid)
- 260922 T305..T309 created from CLIP r42 CDS r25; tasked stays 40/23 because the r41/r24 Wanted Sans delta is still untasked (vid)
- 260921 create-task CLIP r42 CDS r25 start; the r41/r24 Wanted Sans delta stays untasked (vid)
- 260921 CLIP r42 CDS r25: a finalized clip's ① and ② open read-only, and one style's drawing and motion are owed by the preview and by both render kinds (vid)
- 260921 T304 done; shared comparison review with distinct model/writing pages and durable entry-aware return links; 2,292 tests, 24 browser combinations and frontend gates pass (ret)
- 260921 update-ssot CLIP CDS start: a finalized clip keeps its earlier steps readable, and caption motion and drawing must agree across preview and both render kinds (vid)
- 260921 T304 created and claimed (ret)
- 260921 create-task MODEL-44 MODEL-60 start (ret)
- 260921 MODEL r13: model/writing review routes retain the entry destination, stage and post-list filters (ret)
- 260921 update-ssot MODEL start: preserve the comparison entry point and distinguish model-lab and writing navigation (ret)
- 260921 T303 done; four AI-model destinations, URL stage filters and responsive group menus; 2,271 tests plus 66 final targeted checks and browser checks pass (aim)
- 260921 T303 created and claimed (aim)
- 260921 create-task MODEL start (aim)
