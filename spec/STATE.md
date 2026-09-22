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
| ARCH | 9 | 9 | - | 0 |
| AUTH | 8 | 8 | - | 0 |
| QUOTA | 19 | 19 | - | 0 |
| POST | 9 | 9 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 8 | 8 | - | 0 |
| MODEL | 16 | 16 | - | 0 |
| TMPL | 8 | 8 | - | 1 |
| GUIDE | 3 | 3 | - | 0 |
| EXPORT | 5 | 5 | - | 0 |
| PUB | 6 | 6 | - | 0 |
| LANG | 5 | 5 | - | 0 |
| THEME | 17 | 15 | THEME-19✎ | 0 |
| MKT | 6 | 6 | - | 0 |
| VIDEO | 3 | 3 | - | 0 |
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
| publishing-260922 | converted@260922 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|

## next
- ideation post-quality-and-related-links 계속 (6 live opens): the exact 주제 names from Naver's own picker, whether title+excerpt is a large enough vocabulary sample, the tag rule against POST-65, an English target, 확정 rewriting `posts.title`, and what ①'s dock shows before an account has published enough to measure
- create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta
- publishing retirement implementation is complete; DEPLOY.md records the prod checkpoint and receipts at c6af0418, with staging not deployed
## log
- 260923 ideation post-quality round 5-6; 분야 becomes a per-post attribute and a third 지침 scope kind; one fixed 상위 노출 단어 사용 preset with a multi-select 적용할 분야 keeps GUIDE-18 intact; guidelinePrecedence narrows 어휘 out; 11 of 17 opens are parked with 추천글/GEO
- 260923 ideation post-quality round 4; word choice moved off the 말투 tab into per-분야 지침 presets filled by a daily product-side 네이버 검색 API batch; 도배율 numerator = most frequent noun, bands product-owned; rule text = code constant overridable by 지침
- 260923 ideation post-quality-and-related-links round 3; body word rule → a recommendation surface in the 말투 tab adopted into [사용자 규칙]; tags by prompt rule only, no search data; 추천글 deferred and reshaped around explicitly saved links
- 260923 ideation post-quality-and-related-links: GEO deferred out of v1 (citation unobservable); title area optional; blanket phrase bans rejected for measured per-account repetition; word choice extended to tags and body
- 260923 template generation freshness investigation complete; each new StartGeneration reads the current saved template body and freezes it in the job; running jobs keep their frozen copy, while template length/tag defaults seed only when assigned
- 260923 template generation freshness investigation start; trace saved template edits into a subsequent post generation request
- 260922 ideation post-quality-and-related-links continue; adding SEO/GEO methodology, title templates, high-value vocabulary
- 260922 CI guard repaired; completed and pending retirement guides both pass while missing cleanup evidence arguments and restored runtime fail; 11 regressions and the real retirement gate pass locally; remote verification awaits push
- 260922 CI investigation start; run 35739196970 failed after the deployment-checkpoint documentation commit
- 260922 T321 done; short email-free seed ids and independent automatic-login/saved-id controls; 2328 FE tests, full BE gate, codegen and CI support checks pass (lgn)
- 260922 T321 claimed (lgn)
- 260922 create-task AUTH complete; T321 consumes AUTH r8
- 260922 create-task AUTH start; AUTH r8 login convenience and development accounts
- 260922 update-ssot AUTH complete; AUTH r8 records short email-free seed ids and independent automatic-login/save-id choices; no active tasks overlap
- 260922 update-ssot AUTH start; short dev seed login ids, opt-in automatic login and saved login ids
- 260922 T320 done; the backend gate carries a 30m bound and now reports per-package results (gate)
- 260922 found with T320: internal/clip/store fails TestTheSoundSettingDoesNotInvalidateAnInterruptedCandidate in a package run and passes alone; it was invisible while the package only reported a timeout, and needs its own task
- 260922 T320 claimed (gate)
- 260922 create-task ARCH complete; T320 created from r9
- 260922 update-ssot ARCH complete; ARCH r9 gives the backend gate a 30m bound because cmd/api (1018s) and internal/clip/store (828s) pass the Go default on their own; create-task pending
