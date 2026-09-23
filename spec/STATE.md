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
| ARCH | 9 | 9 | - | 0 |
| AUTH | 8 | 8 | - | 0 |
| QUOTA | 19 | 19 | - | 0 |
| POST | 10 | 9 | POST-73+ POST-74+ POST-75+ POST-76+ POST-77+ POST-78+ POST-79+ POST-80+ POST-81+ POST-82+ POST-83+ POST-84+ POST-85+ POST-13✎ POST-66✎ POST-54✎ POST-51✎ | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 9 | 8 | GEN-48+ GEN-49+ GEN-50+ GEN-51+ GEN-52+ GEN-53+ GEN-54+ GEN-14✎ | 0 |
| MODEL | 16 | 16 | - | 0 |
| TMPL | 9 | 8 | TMPL-50+ TMPL-51+ TMPL-52+ TMPL-53x TMPL-54x TMPL-55+ | 1 |
| GUIDE | 4 | 3 | GUIDE-29+ GUIDE-30+ GUIDE-31+ GUIDE-32+ GUIDE-33+ GUIDE-34+ GUIDE-35+ GUIDE-36+ GUIDE-37+ GUIDE-38+ GUIDE-39+ GUIDE-5✎ GUIDE-14✎ | 0 |
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
| QUAL | 1 | 0 | all | 0 |

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
- create-task QUAL POST GEN GUIDE TMPL for the 발행됨 status and post measurement — QUAL r1 is a new domain (`all`), the other four carry deltas; QUAL and POST come first because GEN, GUIDE and TMPL all reference the status, the 분야 attribute and the phrase list
- no [?] remains in any of the five; TMPL-39 (a photo row of n > 1 through export) predates this work and is unrelated to it
- create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta
## log
- 260923 the five domains' opens all closed ahead of create-task; measurement split into a per-post layer shown from ② and an aggregate over 발행됨 posts only, vocabulary ranked guideline > template > voice with the preset injected last, and 분야 picked on ①'s panel
- 260923 ideation post-quality-and-related-links converted; QUAL r1 created and POST r10, GEN r9, GUIDE r4, TMPL r9 carry the rest; 추천글 and GEO stay parked in the ideation doc
- 260923 update-ssot TMPL complete; r9 adds the optional title area limited to literal text, `<write>` and `<ask>`
- 260923 update-ssot GUIDE complete; r4 adds the preset, the `fields` scope kind and the precedence narrowing, with `templatePrecedence`'s identical wording left as [?]
- 260923 update-ssot GEN complete; r9 adds the frozen 분야 phrase list, the write answer's replacement candidates, the two sourced title prohibitions, the tag rule and the ticked quality rules' position
- 260923 update-ssot POST complete; r10 adds the 발행됨 status and its pasted URL, the content lock that spares export/copy/말투 학습, the fourth badge and filter, ②'s replacement spans and the brief's quality checkboxes
- 260923 create-ssot QUAL complete; r1 carries four metrics over 발행됨 posts with per-metric minimums, stored per-post measurements, and the daily 네이버 검색 API phrase batch; bands, 분야 names and English-target analysis stay [?]
- 260923 ideation post-quality-and-related-links ready; 10 rounds, v1 is the 발행됨 status and its measurement set, four metrics with offered rules, an optional template title area, and per-분야 phrases reaching a post by two routes; 추천글 and GEO stay parked
- 260923 ideation post-quality round 10; Jev examined and rejected for v1 against the owner's own test (price not lower — the write pass adds no call while Jev resends the post per span; quality unevidenced), so the write pass returns candidate spans as JSON; a chosen replacement is a manual edit; the 발행됨 lock covers content edits only
- 260923 ideation post-quality round 9; the 지침 and ②'s offer split by what the source contains (same meaning already stated → substituted in the prompt; absent → offered on hover), so rounds 5-6 stand; candidate spans come from the write pass as JSON; the offer covers body, title and tags; clearing the URL unlocks a 발행됨 post back to 확정
- 260923 ideation post-quality round 7-8; the doc's PUB-15/22/25 footing was retired on 260922, so a manual Naver URL now enters a new terminal 발행됨 status that supplies both the measured title set and the future link pool; body vocabulary leaves the write prompt entirely and becomes a hover replacement offer on ②; every rule is language-blind
- 260923 ideation post-quality round 5-6; 분야 becomes a per-post attribute and a third 지침 scope kind; one fixed 상위 노출 단어 사용 preset with a multi-select 적용할 분야 keeps GUIDE-18 intact; guidelinePrecedence narrows 어휘 out; 11 of 17 opens are parked with 추천글/GEO
- 260923 ideation post-quality round 4; word choice moved off the 말투 tab into per-분야 지침 presets filled by a daily product-side 네이버 검색 API batch; 도배율 numerator = most frequent noun, bands product-owned; rule text = code constant overridable by 지침
- 260923 ideation post-quality-and-related-links round 3; body word rule → a recommendation surface in the 말투 tab adopted into [사용자 규칙]; tags by prompt rule only, no search data; 추천글 deferred and reshaped around explicitly saved links
- 260923 ideation post-quality-and-related-links: GEO deferred out of v1 (citation unobservable); title area optional; blanket phrase bans rejected for measured per-account repetition; word choice extended to tags and body
- 260923 template generation freshness investigation complete; each new StartGeneration reads the current saved template body and freezes it in the job; running jobs keep their frozen copy, while template length/tag defaults seed only when assigned
- 260923 template generation freshness investigation start; trace saved template edits into a subsequent post generation request
- 260922 ideation post-quality-and-related-links continue; adding SEO/GEO methodology, title templates, high-value vocabulary
- 260922 CI guard repaired; completed and pending retirement guides both pass while missing cleanup evidence arguments and restored runtime fail; 11 regressions and the real retirement gate pass locally; remote verification awaits push
- 260922 CI investigation start; run 35739196970 failed after the deployment-checkpoint documentation commit
