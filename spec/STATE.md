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
| TMPL | 7 | 7 | - | 1 |
| GUIDE | 3 | 3 | - | 0 |
| EXPORT | 4 | 4 | - | 0 |
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

## next
- nothing is unblocked: every remaining task waits on T008 (another session) or is blocked (T177)
- the clip `release-smoke` stage is still red at HEAD on this host (9 of 28 modes end in `no result`) and still needs a fix task
- a `review-code` pass over the new memory domain is worth a look before it grows
## log
- 260921 TMPL r7..r7 no-op (no code impact); the same stale marker prose in ExportPanel, BlockList, README and PRD was aligned in passing — comments and reference docs only, ARCH-25 green
- 260921 create-task TMPL start (alt)
- 260921 TMPL r7: TMPL-17 23 39 cite the current Naver photo marker, and a slot placeholder is told from it by its brackets (alt)
- 260921 update-ssot TMPL start (alt)
- 260921 T294 done; the Naver photo marker carries its caption folded to one `_`-joined token (`사진_1_비_뒤의_바다_사진`), captionless blocks keep the bare form, and both Naver snapshots moved by exactly those lines
- 260921 T294 created from EXPORT r4: one task, the converter's fold plus its tests, goldens and the panel's guidance line (alt)
- 260921 create-task EXPORT start (alt)
- 260921 EXPORT r4: the Naver photo marker carries its caption folded into one double-clickable token; EXPORT-24's caption copy stays and its reason follows (alt)
- 260921 update-ssot EXPORT start (alt)
- 260920 T293 done; ① carries the 기억 사용 checkbox (autosaved, flag only) and ③ carries 기억으로 저장 with the candidate sheet that creates only what is checked — the MEM chain is complete
- 260920 T292 done; /memories is the 글 group's fifth destination — an entity, three action features and a list-only page, with the kind and tags saved as one edit and the cap relayed from the server
- 260920 T291 done; extract_memory is a credit-gated post-addressed job whose candidates live on its own job row — it writes no memory and touches no post; the queue gained SaveResult/Result for that one shape of work
- 260920 T290 done; posts carry use_memory, retrieval is tag-overlap over folded substrings in internal/memory, and the frozen texts render as one [기억] section in the per-post half — a post with the option off is byte-identical to T287's goldens
- 260920 T289 done; the memory aggregate exists end to end (0069, internal/memory, MemoryService, the post-delete hook, the four bounds); memory_sources.post_slug is deliberately FK-less — see the task result
- 260920 T288 done; the Naver photo marker is a bare `사진_<n>_사진` and each caption is its own copy control under its photo; TMPL still describes the old marker in three decisions (update-ssot owed)
- 260920 T287 done; the write prompt gained the altitude rule as a fourth grounding constant, the write scope now binds factual claims alone and the naming rule forbids the frame rather than the memo; only the write golden moved, by exactly three lines
- 260920 T287-T293 created from MEM r1 + the five amendments: prompt altitude, Naver marker, the memory store, retrieval, extraction, the 기억 page, the two post surfaces (mem)
- 260920 create-task MEM GEN GUIDE POST EXPORT QUOTA start (mem)
- 260920 GEN r7 GUIDE r3 POST r8 EXPORT r3 QUOTA r13: the altitude rule, grounding bound to factual claims, the opt-in `[기억]` section, ①'s checkbox, ③'s extraction and the numbered photo marker (mem)
- 260920 update-ssot GEN GUIDE POST EXPORT QUOTA start (mem)
