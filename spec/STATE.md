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
| T258 | shared/config keeps env and cross-slice constants only | ARCH | - | todo |
| T259 | proto symbols and connect-query reach no further than entity api | ARCH | - | todo |
| T260 | split entities/clip-project into four nouns | ARCH | - | todo |
| T261 | the clip page composes a widget instead of wiring eleven hooks | ARCH | T260 | todo |
| T262 | clip pages and features consume entity hooks, allowlist removed | ARCH | T259 T261 | todo |
| T263 | the draft editor page composes hooks it does not own | ARCH | - | todo |
| T264 | i18n namespaces assembled from slice-owned fragments | ARCH | - | todo |
| T265 | the clips namespace lives in the clip slices | ARCH | T261 T264 | todo |
| T266 | one autosave queue behind three save features | ARCH | - | todo |
| T267 | pages consume hooks; verbs have one home; post owns its cache dependencies | ARCH | T259 | todo |
| T268 | the route tree is assembled from route groups | ARCH | - | todo |
| T269 | platform/config holds env only; limits live in their context | ARCH | - | todo |
| T272 | the clip credit allowance and cancellation policy leave the queue | ARCH | T285 | todo |
| T275 | job/store, template/rpc and modelcatalog/rpc have tests | ARCH | T285 | todo |
| T276 | voice, post and publishing ports are per use-case | ARCH | - | todo |
| T277 | experiment, usage and auth ports are per use-case | ARCH | T286 | todo |
| T278 | a failure reason is a proto enum both sides compile against | ARCH | - | todo |
| T279 | every hand-kept enum mirror is pinned to the generated enum | ARCH | T008 | todo |
| T280 | six boundary leaks translated at the adapter | ARCH | - | todo |
| T281 | clip.proto becomes one file and service per rpc family | ARCH | T262 | todo |
| T282 | the agent maps proto at one adapter and keeps preflight out of main | ARCH | T008 | todo |
| T283 | SmartEditor scripts are files with a DOM test; naver is plan vs driver | ARCH | T282 | todo |
| T284 | the agent generates only the protos it uses and drops the survey harness | ARCH | T008 T281 | todo |
| T286 | the ledger settles by approved ceiling, not by product kind | ARCH | T272 | todo |

## next
- implement-task T272 (clip allowance into clip/app; dep T285 done) then T286 (ledger de-named); other roots meanwhile: BE T269 T276 T278 T280, FE T258 T259 T260 T263 T264 T266 T268; follow-up to log as a task: constructor treatment for voice/experiment/guideline/modelcatalog/billingstore setters and moving jobAdmission/meteredRegistry out of package main
- the clip `release-smoke` stage is red at HEAD on this host: 9 of 28 modes end in `no result` (generation ends on a plan since T255, harness still expects a Result) — needs a fix task (review-code clip-release-smoke or update the harness)
- agent tasks T279 T282 T283 T284 wait for T008; T008 belongs to another session and T177 remains blocked
- post-quality-and-related-links remains open ideation, awaiting conversion when ready
## log
- 260919 T285 done; job addresses work by Subject{Dimension,ID} (Guards stated by the caller, 7 store lookups collapsed, experiment id derived+indexed); ARCH-26 and ARCH-28 green. Deviation: the subject_kind/subject_id pair was dropped as unreadable without behaviour change — see the task result
- 260919 T285 claimed (sub)
- 260919 create-task T272 re-split done: T285 (subject addressing, VIRTUAL generated columns) + T272 (allowance into clip/app) + T286 (ledger de-named, charge math frozen); T275 dep→T285, T277 dep→T286
- 260919 create-task T272 re-split start (owner decided: generated-column bridge, allowance into clip/app, usage de-named with frozen charge math)
- 260919 T274 done; Run is a 44-line sequence over generationRun stages (accept/prepare/analyze/write/layout/save/finish) with stage unit tests, no bare clock left in clip/app; ARCH-26 green, release smoke unchanged (pre-existing 9 `no result`)
- 260919 T273 done; the clip root is the pure domain (deps: design, composition, llm, plan) and clip/app holds the three services, their methods and the worker orchestration (4.6k lines) — T274 keeps only the Run decomposition; ARCH-26 green, release smoke unchanged (pre-existing 9 `no result`)
- 260919 T272 blocked; job/usage clip knowledge reaches the schema (clip_project_id) and the settlement rules — three decisions owed (columns, allowance port, SettlementPolicy) before an implementer can proceed; T277 (dep T272) and T275 (dep T272) wait
- 260919 T271 done; main is loadPlatform→buildContexts→registerJobs→serve (23 lines), post/generation/auth/clip/usage take collaborators in constructors and a cmd/api wiring test builds the whole graph; ARCH-26 green. Out of scope: 7 setters on voice/experiment/guideline/modelcatalog/billingstore, and jobAdmission/meteredRegistry rule bodies still in main (T272's seam)
- 260919 T271 claimed (arc)
- 260919 T270 done; clip sagas live in internal/clip/app over tx-scoped ports, cmd/api keeps wiring; ARCH-26 green; release-smoke 19/28 with the same 9 `no result` failures on an untouched HEAD build (pre-existing, out of scope)
- 260919 T270 claimed (arc)
- 260919 create-task review/arch-260919 + ARCH r5 done: 27 tasks T258-T284 (FE 11 · BE 12 · agent 3 · cross 1), review converted, ARCH tasked=5
- 260919 create-task review/arch-260919 + ARCH r4 start
- 260919 create-architecture ARCH r4 done (ARCH-3/6/14/16/17/21✎, ARCH-40/41+); warning: ARCH-3✎ touches agent enum mirrors that T008 (doing) exercises live — F24-F30 tasks must depend on T008
- 260919 create-architecture ARCH r4 start (review/arch-260919 gaps: ARCH-6 saga home, ARCH-14 verb rule, ARCH-21 config wording, buf breaking rule)
- 260919 review-code arch-260919 ready; owner adopted all 31 (rule: clear anything that accrues per change now); next create-task review/arch-260919
- 260919 review-code arch-260919 FE re-verified at 19c19cc2; F9 widened (6 pages own RPC), F31 added (cross-domain cache keys in 8 features); 31 [?] awaiting triage
- 260919 review-code arch-260919 findings written (30, 1×P1 F13 cmd/api sagas; FE 12 · BE 10 · agent/proto 8); awaiting triage
- 260919 review-code arch-260919 start
- 260918 T257 done; approval quotes the whole target before narration and actual styled captions afterward, in ko/en seconds with no sequence ceiling; all local gates pass.
