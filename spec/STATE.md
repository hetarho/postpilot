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
| QUOTA | 12 | 12 | - | 0 |
| POST | 7 | 7 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 6 | 6 | - | 0 |
| MODEL | 10 | 10 | - | 0 |
| TMPL | 6 | 6 | - | 1 |
| GUIDE | 2 | 2 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 5 | 5 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 13 | 13 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 40 | 40 | - | 1 |
| CDS | 23 | 23 | - | 1 |
| BILL | 4 | 4 | - | 0 |

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
- every unblocked task is done: the review's FE half (T259-T268) and BE half (T275-T281) are all in `tasks/done/`. What is left waits on T008, which belongs to another session — T279 T282 T283 T284 — or is blocked (T177)
- T281 changed the clip rpc PATHS: the next deploy must ship the API image and the web build TOGETHER (ARCH-41), and `buf breaking` will report the removed `ClipService` once, which is that intended break
- the clip `release-smoke` stage is red at HEAD on this host: 9 of 28 modes end in `no result` (generation ends on a plan since T255, harness still expects a Result) — still needs a fix task (review-code clip-release-smoke or update the harness)
- post-quality-and-related-links remains open ideation, awaiting conversion when ready
## log
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
- 260920 T268 done; router.tsx is 70 lines over app/routes/tree.ts + 11 group files, 12 search schemas moved to their pages, and tree.test pins all 45 addresses; ARCH-25 green
- 260920 T268 claimed (clp)
- 260920 T266 done; shared/lib/autosave carries the clip-settings and block-editor queues (223→82, 236→155) with 10 unit tests; save-draft stays bespoke (assignments + mid-flight rekey) and says why; ARCH-25 green
- 260920 T266 claimed (clp)
- 260920 T265 done; the 1.1k-line clips namespace is 17 slice fragments (largest 209 lines/language) and app/providers/i18n keeps only the five cross-cutting namespaces; ARCH-25 green
- 260920 T265 claimed (clp)
- 260920 T264 done; 8 domain namespaces became 63 slice fragments assembled by app/providers/i18n (leaf-module imports, one steiger exception) and resources.test gained a no-duplicate-claim check; ARCH-25 green
- 260920 T264 claimed (clp)
