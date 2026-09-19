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
| T265 | the clips namespace lives in the clip slices | ARCH | T261 T264 | todo |
| T266 | one autosave queue behind three save features | ARCH | - | todo |
| T268 | the route tree is assembled from route groups | ARCH | - | todo |
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

## next
- implement-task: T265 (clips namespace into the clip slices) T266 (i18n per slice, then T265) T266 (autosave queue) T268 (route groups) T263 T264 T266 T268; BE roots T276 (voice/post/publishing ports), T277 (experiment/usage/auth ports), T278 (typed failure reasons), T280 (boundary leaks), T275 (missing test packages)
- the clip `release-smoke` stage is red at HEAD on this host: 9 of 28 modes end in `no result` (generation ends on a plan since T255, harness still expects a Result) — needs a fix task (review-code clip-release-smoke or update the harness)
- agent tasks T279 T282 T283 T284 wait for T008; T008 belongs to another session and T177 remains blocked
- post-quality-and-related-links remains open ideation, awaiting conversion when ready
## log
- 260920 T264 done; 8 domain namespaces became 63 slice fragments assembled by app/providers/i18n (leaf-module imports, one steiger exception) and resources.test gained a no-duplicate-claim check; ARCH-25 green
- 260920 T264 claimed (clp)
- 260920 T263 done; DraftEditor is 253 lines / 3 useState over useDraftSteps + useBriefMirror + useCaretHandoff + useDraftAssignments, five inline components became eleven files and the last ESLint warning is gone (lint now 0 problems); ARCH-25 green
- 260920 T263 claimed (clp)
- 260920 T262 done; the clip ESLint/vitest allowlist is deleted, 12 clip slices trade transports for entity call hooks (project/source/plan/render families) and ClipRenderKind stops at clip-preview; ARCH-25 green
- 260920 T262 claimed (clp)
- 260920 T261 done; ClipPage is 98 lines over widgets/clip-workspace, useClipWorkspace returns 11 handles, ClipCorrectionWorkspace takes 6 props (was 22) and the four page-held rules are named model functions with 27 DOM-free assertions; ARCH-25 green
- 260920 T261 claimed (clp)
- 260920 T260 done; five slices (clip-design is the config leaf the owner approved), the clip @x graph is acyclic and clip-project's barrel is 55 symbols (was 116); preview fetching moved to features/preview-clip-draft and the entity player renders from props; ARCH-25 green
- 260920 out of scope (T260): entities/@x holds a pre-existing cycle generation-job → voice → post → generation-job
- 260920 T260 owner decision: config becomes a fifth leaf slice `entities/clip-design` — the four-noun @x graph cannot be acyclic while the aggregate embeds plan/observation types and both read the design config
- 260920 T260 claimed (clp)
- 260920 T267 done; one `invalidatePostsDependingOn` entry in entities/post replaces the post keys 8 verbs restated, entities/observation folded into post, and pages/clip gave up the last page-held transport; ARCH-25 green
- 260920 T267 claimed (ent)
- 260920 T259 done; an ESLint block + a source-tree vitest hold ARCH-17 over pages/widgets/features (clip allowlisted for T262), 45 slice files traded descriptors for entity hooks, and 7 slices whose only hook moved were deleted; ARCH-25 green
- 260920 T259 claimed (ent)
- 260920 T258 done; shared/config is 80 lines of env + cross-slice values, product limits and slice tuning live in 24 new `config` segments, clip-design.json moved into entities/clip-project and reaches clip-template through a new @x; ARCH-25 green
- 260920 T269 done; platform/config imports nothing under internal/ (guard test), clip/post/voice/generation own their limits and cmd/api merges env via clipEnvironment; ARCH-26 green and the release smoke is the pre-existing 19/28
- 260920 T258 T269 claimed (cfg)
- 260920 T286 done; usage names no product (approved-kinds list from the root, ceiling read off the admission), charge math frozen and pinned by a new parity table; ARCH-26/28 green
