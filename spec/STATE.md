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
| ARCH | 5 | 5 | - | 0 |
| AUTH | 5 | 5 | - | 0 |
| QUOTA | 13 | 13 | - | 0 |
| POST | 8 | 8 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 7 | 7 | - | 0 |
| MODEL | 10 | 10 | - | 0 |
| TMPL | 6 | 6 | - | 1 |
| GUIDE | 3 | 3 | - | 0 |
| EXPORT | 3 | 3 | - | 0 |
| PUB | 5 | 5 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 13 | 13 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 40 | 40 | - | 1 |
| CDS | 23 | 23 | - | 1 |
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
| T289 | memories are a table, a context and a directory RPC family | MEM | - | todo |
| T290 | retrieval, the frozen [기억] section and the draft's opt-in | MEM GEN GUIDE POST | T289 | todo |
| T291 | extract_memory is a job that proposes candidates and stores none | MEM QUOTA | T289 | todo |
| T292 | 기억 is the fifth destination of the 글 group | MEM | T289 | todo |
| T293 | ①'s 기억 사용 checkbox and ③'s candidate sheet | POST MEM | T290 T291 | todo |

## next
- the memory chain is next and it is all that is left outside T008: T289 → (T290 · T291 · T292) → T293; only T289 blocks anything
- `update-ssot TMPL` owes a rev: TMPL-17 TMPL-23 TMPL-39 still call the Naver photo marker `[사진 …]`, and TMPL-23 still tells a slot placeholder from it "by their filename suffix" — EXPORT r3 moved that to the brackets (README.md and PRD.md carry the old marker too)
- T279 T282 T283 T284 still wait on T008 (another session); T177 is blocked; the clip `release-smoke` stage is still red at HEAD on this host (9 of 28 modes end in `no result`) and still needs a fix task
## log
- 260920 T288 done; the Naver photo marker is a bare `사진_<n>_사진` and each caption is its own copy control under its photo; TMPL still describes the old marker in three decisions (update-ssot owed)
- 260920 T287 done; the write prompt gained the altitude rule as a fourth grounding constant, the write scope now binds factual claims alone and the naming rule forbids the frame rather than the memo; only the write golden moved, by exactly three lines
- 260920 T287-T293 created from MEM r1 + the five amendments: prompt altitude, Naver marker, the memory store, retrieval, extraction, the 기억 page, the two post surfaces (mem)
- 260920 create-task MEM GEN GUIDE POST EXPORT QUOTA start (mem)
- 260920 GEN r7 GUIDE r3 POST r8 EXPORT r3 QUOTA r13: the altitude rule, grounding bound to factual claims, the opt-in `[기억]` section, ①'s checkbox, ③'s extraction and the numbered photo marker (mem)
- 260920 update-ssot GEN GUIDE POST EXPORT QUOTA start (mem)
- 260920 MEM r1 written: memories are the write prompt's fourth grounding source, opt-in per draft, tag-retrieved with no embedding (mem)
- 260920 create-ssot MEM start (mem)
- 260920 T281 done; clip.proto is five files/services (template·source·generation·plan·render), buf breaks on PACKAGE, one BE handler serves all five and each FE entity names its family. BE+FE deploy together: the rpc paths changed
- 260920 T281 claimed (clp)
- 260920 T278 done; FailureReason is a 222-value proto enum both sides compile against, the 212-line allowlist is gone and two tests hold the contract at both ends; the wire is unchanged
- 260920 T278 claimed (clp)
- 260920 T277 done; experiment/usage/auth traded three 23-24 method Stores for 14 behaviour ports, none over 10, with the usage tx port kept as WriteScope; ARCH-26 green
- 260920 T277 claimed (clp)
- 260920 T276 done; post/voice/publishing traded four table-shaped Stores (31+22+37+29) for 22 behaviour ports, none over 10 methods, with the composites left only as the composition root handle; ARCH-26 green
- 260920 T276 claimed (clp)
- 260920 T280 done; billing/plan/publishing boundaries translate at the adapter, provision takes Settings, health moved under platform, and the generate payload is pinned by a golden test. The json-tag item is a mapper-in-the-same-package fact, not a leak — see the task result
- 260920 T280 claimed (clp)
- 260920 T275 done; job/store has 7 lifecycle+authorization tests (dispatch vs cancellation serialization included) and template/rpc + modelcatalog/rpc have handler tests for every mapping and refusal; ARCH-26 green
- 260920 T275 claimed (clp)
