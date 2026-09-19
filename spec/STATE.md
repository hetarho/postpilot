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
| T271 | required collaborators enter through constructors; main is four steps | ARCH | T270 | todo |
| T272 | job and usage expose primitives; clip composes them | ARCH | T270 | todo |
| T273 | the clip root is the domain; use-cases live in clip/app | ARCH | T270 | todo |
| T274 | clip generation orchestration is a sequence of testable stages | ARCH | T273 | todo |
| T275 | job/store, template/rpc and modelcatalog/rpc have tests | ARCH | T272 | todo |
| T276 | voice, post and publishing ports are per use-case | ARCH | - | todo |
| T277 | experiment, usage and auth ports are per use-case | ARCH | T272 | todo |
| T278 | a failure reason is a proto enum both sides compile against | ARCH | - | todo |
| T279 | every hand-kept enum mirror is pinned to the generated enum | ARCH | T008 | todo |
| T280 | six boundary leaks translated at the adapter | ARCH | - | todo |
| T281 | clip.proto becomes one file and service per rpc family | ARCH | T262 | todo |
| T282 | the agent maps proto at one adapter and keeps preflight out of main | ARCH | T008 | todo |
| T283 | SmartEditor scripts are files with a DOM test; naver is plan vs driver | ARCH | T282 | todo |
| T284 | the agent generates only the protos it uses and drops the survey harness | ARCH | T008 T281 | todo |

## next
- implement-task T271 (constructor injection + main shape, dep T270 done); then T272 T273; FE roots with no dep: T258 T259 T260 T263 T264 T266 T268; BE roots: T269 T276 T278 T280
- the clip `release-smoke` stage is red at HEAD on this host: 9 of 28 modes end in `no result` (generation ends on a plan since T255, harness still expects a Result) — needs a fix task (review-code clip-release-smoke or update the harness)
- agent tasks T279 T282 T283 T284 wait for T008; T008 belongs to another session and T177 remains blocked
- post-quality-and-related-links remains open ideation, awaiting conversion when ready
## log
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
- 260918 T257 claimed (rnd)
- 260918 T256 done; narration names each caption style, defaults without extra calls, preserves owner/legacy choices and reports out-of-set fallbacks; all local and image gates pass.
- 260918 T256 claimed (rnd)
- 260918 T254 done; browser rendering stays in ② with encode/store progress, cancellation and navigation cleanup; promotion and orphan cleanup serialize without an encoding time limit, and real Chromium plus local/image gates pass.
- 260918 T254 claimed (rnd)
- 260918 T253 done; MP4 timing excludes measured AAC priming, direct immutable uploads promote atomically after stored-file/verdict checks, and failed attempts preserve the prior result; all local and image gates pass.
- 260918 T253 claimed (rnd)
- 260918 T252 done; retained-source audio uses pitch-preserving rates, exact cut timing, BS.1770 normalization and measured AAC priming; Chromium audio and video checks pass.
- 260918 T252 claimed (rnd)
