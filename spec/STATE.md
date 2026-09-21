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
| QUOTA | 17 | 17 | - | 0 |
| POST | 8 | 8 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 7 | 7 | - | 0 |
| MODEL | 11 | 11 | - | 0 |
| TMPL | 7 | 7 | - | 1 |
| GUIDE | 3 | 3 | - | 0 |
| EXPORT | 4 | 4 | - | 0 |
| PUB | 5 | 5 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 16 | 15 | THEME-19✎ | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 41 | 40 | CLIP-13✎ | 1 |
| CDS | 24 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ | 1 |
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
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T177 | Release QA viewing checklist | CDS CLIP | T176 | blocked@260916 |
| T279 | every hand-kept enum mirror is pinned to the generated enum | ARCH | T008 | todo |
| T282 | the agent maps proto at one adapter and keeps preflight out of main | ARCH | T008 | todo |
| T283 | SmartEditor scripts are files with a DOM test; naver is plan vs driver | ARCH | T282 | todo |
| T284 | the agent generates only the protos it uses and drops the survey harness | ARCH | T008 T281 | todo |

## next
- create-task MKT THEME for /about at 320px/200% text: the marketing header overflows the viewport and the nested ladder column stays ~38px over
- create-task CDS CLIP THEME for the Wanted Sans delta — the code already carries it, so the task verifies and records rather than implements
- remaining tasks wait on T008 (another session) or remain blocked at T177
- follow-up: group-directory mobile headings duplicate the group row, and the clip release smoke still needs a fix task
## log
- 260921 T300 done; the 320px/200% overflow was the ladder card header pinning min-content at 341.7px, now wrapped; every ARCH gate green (plr)
- 260921 T300 claimed to resume the WIP checkpoint verification (plr)
- 260921 T300 WIP checkpoint requested; stopped verification, released claim to todo, and deferred any additional live billing price work per user (est)
- 260921 T300 created from QUOTA r17 and claimed (est)
- 260921 create-task QUOTA start (est)
- 260921 QUOTA r17: per-item condition editor and four simultaneous blog/clip estimates; finished clip length selected, original sources assume 60 seconds each; THEME needs no new exception (est)
- 260921 update-ssot QUOTA THEME start: separate per-item conditions from four model-level estimates, add blog/clip modes and a floating condition editor (est)
- 260921 blocked T177 release QA predates the face swap: its viewing checklist must be re-read against Wanted Sans before it unblocks (fnt)
- 260921 CDS r24 CLIP r41 THEME r16: Wanted Sans Variable is the one bundled sans face for the app and the renderer; the frontend, backend, bundled assets and 23 goldens already carry it, so the delta is pending verification rather than implementation (fnt)
- 260921 update-ssot CDS CLIP THEME start: Wanted Sans replaces Pretendard as the bundled sans face (fnt)
- 260921 T298 done; viewport plans aurora, mobile estimate sheet, $3/$10/$20 pricing and 1.5x estimate token allowance verified (pln)
- 260921 T299 done; paid cards show extra credits and percentages against $1/100-credit manual top-ups (pln)
- 260921 T299 created and claimed; T298 remains fresh because r16 adds only the separately implemented benefit copy (pln)
- 260921 create-task QUOTA start (pln)
- 260921 QUOTA r16: subscription benefits compared with at-par top-ups (pln)
- 260921 update-ssot QUOTA start: show subscription bonuses against at-par top-ups (pln)
- 260921 T298 created from QUOTA r15 THEME r15 and claimed (pln)
- 260921 create-task QUOTA THEME start (pln)
- 260921 QUOTA r15 THEME r15: $3/$10/$20 ladder, revision-aware estimates and immersive responsive plans; BILL consumes the same price rule without policy changes (pln)
- 260921 update-ssot QUOTA BILL THEME start: immersive plans page, mobile estimate sheet, $3/$10/$20 tiers and 1.5x generation allowance (pln)
