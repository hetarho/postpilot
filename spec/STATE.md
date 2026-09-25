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
| post-quality-and-related-links | converted@260923 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 11 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 8 | 8 | - | 0 |
| QUOTA | 19 | 19 | - | 0 |
| POST | 15 | 15 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 12 | 12 | - | 0 |
| MODEL | 17 | 17 | - | 0 |
| TMPL | 11 | 11 | - | 1 |
| GUIDE | 6 | 6 | - | 0 |
| EXPORT | 5 | 5 | - | 0 |
| PUB | 6 | 6 | - | 0 |
| LANG | 5 | 5 | - | 0 |
| THEME | 18 | 15 | THEME-19✎ | 0 |
| MKT | 6 | 6 | - | 0 |
| VIDEO | 3 | 3 | - | 0 |
| CLIP | 44 | 40 | CLIP-13✎ CLIP-163+ | 2 |
| CDS | 25 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 3 | 3 | - | 2 |
| QUAL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |
| arch-260919 | converted@260919 |
| publishing-260922 | converted@260922 |
| published-quality-260924 | converted@260925 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|

## next
- POST r15 is implemented (T392, T393): /posts pages with server-side search/filter and restores its scroll; ListClipProjects still answers whole and can follow the same shape when needed
- Later: update-ssot CLIP-163, then create-task ARCH CLIP for real-GPU validation, profile approval/automatic selection and concurrency tuning; ARCH r11 and CLIP r44 remain partially consumed, and physical host setup/migration remain operator actions
- The review wave T354..T375 (review/published-quality-260924) is complete, one commit per task (p42); create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta

## log
- 260925 note (pg): the dev API on 7678 still runs a 15 h old binary — every Air rebuild since T389..T391 exits at boot for want of MEDIA_WORKER_CREDENTIALS (the container predates .env.media.dev); `pnpm dev` recreates it
- 260925 T393 done (pg): /posts reads server-narrowed pages, loads the next as its end comes within half a screen, reports list-end loading/failure with 다시 시도, and restores rows and scroll on return (router restoration now on for every screen, /posts keyed by address); FE gate green; verified in Chrome at 390 and 1280
- 260925 T393 claimed (pg)
- 260925 T392 done (pg): ListPosts pages by a keyset token over the stored updated_at+slug, narrows query/status on the server over every owned post, reads json_extract title/tags only; page_size 0 stays unpaged; BE and FE gates green
- 260925 T392 claimed (pg)
- 260925 create-task POST done: r15 → T392 (paged ListPosts, server-side query/status, keyset token) and T393 (infinite /posts, list-end loading/retry, scroll restoration); POST tasked=15
- 260925 create-task POST start (r15: POST-90..93)
- 260925 update-ssot POST done: r15 adds POST-90..93 (incremental /posts at every width, owned-post-wide narrowing, list-end loading/retry, kept rows and scroll on return); no cross-SSOT references, no doing tasks affected
- 260925 update-ssot POST start: /posts list moves to incremental loading with server-side search and status filter
- 260925 T391 done (mw): Deploy backend succeeded in 201s; separate parallel Verify media and CI are green, release logs confirm cached media tools, and DEPLOY.md records measured evidence
- 260925 T391 production timing confirmed (mw): e933ea73 completed Deploy backend in 201s, API health is ok, and three independent Verify media jobs started afterward; remaining CI/media verification is monitored separately
- 260925 T391 local verification passed (mw): 60 deployment tests, workflow lint, both prebuilt-image release layouts and all BE/FE/codegen/spec gates; delivery timing remains to be measured
- 260925 T391 claimed (mw): restore fast deployment by separating long media checks, narrowing production serialization and reusing fixture build caches; preserve all media coverage and health/rollback gates
- 260925 T390 done (mw): disposable MinIO/mc now build from pinned official source; both real release layouts, 51 deploy tests and all repository gates pass; T389 production rollout/CI and external health are confirmed
- 260925 T390 claimed (mw): T389 is pushed and production rollout/CI passed; repair the previous run's MinIO fixture image pull failure before completing deployment smoke verification
- 260925 T389 done (mw): first CPU deployment initializes missing worker.env and preserves credentials on retry, with a read-only SQLite backup before the forward-only swap; 48 deploy tests and all BE/FE/codegen/spec gates pass; no live deployment
- 260925 T389 claimed (mw): fix missing worker.env during the first automated CPU VPS deployment; preserve established worker configuration and forward-only rollback protection
- 260925 T375 done; the 38 per-control cases now live in their slices' tests (five new slice test files) and the editor page suites keep 74 cases; the T354..T375 review wave is complete; FE gates pass (p42)
- 260925 T375 claimed (p42)
- 260925 T373 done; entity indexes export no wire mapper (fakes map by member name), the draft save takes a domain PostDraftSave, an unknown 분야 or quality tick fails the read, and one exhaustive status split replaces the four chains; ESLint and arch pins guard it; FE gates pass (p42)
