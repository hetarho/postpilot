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
| POST | 14 | 14 | - | 0 |
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
- The media-worker wave T376..T388 and deployment follow-ups T389..T390 are complete; T389 CPU rollout and CI passed in production, and T390 removes unavailable fixture binaries through pinned source builds; future GPU host setup/migration remains operator-controlled
- Later: update-ssot CLIP-163, then create-task ARCH CLIP for real-GPU validation, profile approval/automatic selection and concurrency tuning; ARCH r11 and CLIP r44 remain partially consumed, and physical host setup/migration remain operator actions
- The review wave T354..T375 (review/published-quality-260924) is complete, one commit per task (p42); create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta

## log
- 260925 T390 done (mw): disposable MinIO/mc now build from pinned official source; both real release layouts, 51 deploy tests and all repository gates pass; T389 production rollout/CI and external health are confirmed
- 260925 T390 claimed (mw): T389 is pushed and production rollout/CI passed; repair the previous run's MinIO fixture image pull failure before completing deployment smoke verification
- 260925 T389 done (mw): first CPU deployment initializes missing worker.env and preserves credentials on retry, with a read-only SQLite backup before the forward-only swap; 48 deploy tests and all BE/FE/codegen/spec gates pass; no live deployment
- 260925 T389 claimed (mw): fix missing worker.env during the first automated CPU VPS deployment; preserve established worker configuration and forward-only rollback protection
- 260925 T375 done; the 38 per-control cases now live in their slices' tests (five new slice test files) and the editor page suites keep 74 cases; the T354..T375 review wave is complete; FE gates pass (p42)
- 260925 T375 claimed (p42)
- 260925 T373 done; entity indexes export no wire mapper (fakes map by member name), the draft save takes a domain PostDraftSave, an unknown 분야 or quality tick fails the read, and one exhaustive status split replaces the four chains; ESLint and arch pins guard it; FE gates pass (p42)
- 260925 T373 claimed (p42)
- 260925 T372 done; the title corpus reaches every title refusal (cross-area too_many_asks included) and a test fails when one goes missing; the drain harness runs the worker's registerJobs; template.Limits has one constructor with post.TargetLengthMin; BE and FE gates pass (p42)
- 260925 T372 claimed (p42)
- 260925 T370 done; quality reads one post row through post.Service.CurrentContent and names M2's run only when over band; Repetition.TopNoun and post_measurements.top_noun are gone (migration 0083); BE gate passes (p42)
- 260925 T370 claimed (p42)
- 260925 T368 done; EditorPage's cases are six step suites on shared helpers and row builders; duplicates of slice tests are gone and the lock wiring folds into the published cases; no page test reads a query key by position; FE gates pass (p42)
- 260925 T368 claimed (p42)
- 260925 T367 done; the autosave decides the published lock and owns ①'s text (a locked refusal takes the text back to the screen); DraftEditor has no lock masking and every editor component reads isPublished(post) itself; FE gates pass (p42)
- 260925 T367 claimed (p42)
- 260925 T366 done; the draft queue holds its voice, 템플릿 and target-language assignments as one record each, walked by one channel list; SendDraft takes one request and the handle one assign(channel, value); FE gates pass (p42)
- 260925 T366 claimed (p42)
- 260925 T365 done; the generation gates take one input whose members are all required; /ai-models/compare now refuses a published post with the lock's sentence; FE gates pass (p42)
- 260925 T365 claimed (p42)
