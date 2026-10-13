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
| searchable-details | open@261007 |
| prompt-engineering | converted@261007 |
| creation-and-comparison-ux | open@261007 |
| storyline-first | converted@260927 |
| voice-tidy | converted@260929 |
| daily-credit-plans | converted@260929 |
| template-from-request | converted@261001 |
| familiar-video-editing-and-dubbing | converted@261004 |
| friendly-model-names | open@261013 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 20 | 20 | - | 0 |
| AUTH | 15 | 15 | - | 0 |
| QUOTA | 38 | 38 | - | 0 |
| POST | 40 | 40 | - | 0 |
| VOICE | 16 | 16 | - | 0 |
| GEN | 30 | 30 | - | 0 |
| MODEL | 36 | 36 | - | 0 |
| TMPL | 27 | 27 | - | 0 |
| GUIDE | 21 | 21 | - | 0 |
| EXPORT | 11 | 11 | - | 0 |
| LANG | 9 | 9 | - | 0 |
| THEME | 33 | 33 | - | 1 |
| MKT | 10 | 10 | - | 0 |
| VIDEO | 7 | 7 | - | 0 |
| CLIP | 59 | 59 | - | 2 |
| CDS | 33 | 33 | - | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 7 | 7 | - | 2 |
| QUAL | 8 | 8 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |
| EDIT | 5 | 5 | - | 0 |
| INFRA | 2 | 0 | all | 1 |

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
| clip-narrate-failure-260926 | converted@260926 |
| conformance-all-260927 | converted@260927 |
| perf-cost-261004 | converted@261005 |
| desktop-ux-policy-261007 | converted@261007 |
| all-261008 | converted@261008 |
| structure-261008 | converted@261008 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | blocked@261007 |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | blocked@261007 |

## next
- ideation friendly-model-names 계속: decide the automatic alias word list (theme, size, contents) and whether speech profiles join; then create-ssot/update-ssot MODEL.
- 6e5039c8 is live with CI, Deploy backend and all three Verify media jobs green. 76b619df's Media worker parity failure did not reproduce (four local runs, green on 6e5039c8); its log needs GitHub auth if it recurs.
- Review the 17 unadopted choices in review/structure-261008 (T654–T663 done); editorial follow-up doc-review ARCH.

## log
- 261013 ideation friendly-model-names paused (open): grade + model-wide neutral alias (가성비 · 소나무), auto-assigned and operator-editable, never reused; operator-confirmed succession moves the alias to a new version and same-grade eligible selections follow with a notice; per-post credits/badges in options, 자세히 under the field. Open: the alias word list.
- 261013 ideation friendly-model-names start: users see grade+number names (무료1, 가성비2) instead of full model names/versions; details only on request.
- 261013 6e5039c8 pushed (T676 + media-release sampling fix): CI ok, Deploy backend ok, Verify media ok (image, release, worker); production runs 6e5039c8.
- 261013 media-release.py resource sampling: an exec failure is judged by the container's state, not Docker's wording (409 attach upgrade and runc procReady forms seen locally); two full gate runs passed in both layouts.
- 261013 T676 done on main: the title opens with its search words and closes with a short implicative phrase.
- 261013 T676 start on main at 6fe0333c: titles 기본 지침 first two sentences.
- 261013 create-task GEN r30: T676 (titles 기본 지침 opens with search words, then an implicative phrase).
- 261013 update-ssot GEN r30 done: GEN-49✎ the title opens with the search words the material supports (area/station, business or product, category or key menu) and closes with a short implicative phrase; GUIDE-41 and TMPL-52 only reference it; no doing task; next create-task GEN.
- 261013 update-ssot GEN start: implicative-only titles drop the post's search terms.
- 261012 76b619df pushed from this tree mid-gate; local gate on it passed (CI list, Compose, 7 images, worker smoke, release in both layouts; usage/store and clip/store load flakes passed on rerun). Remote: CI ok, Deploy backend ok, Verify media Media worker failed at CPU parity like c01b5169.
- 261012 T675 done on main: 최소 태그 수 / Minimum tag count everywhere; the tag help now explains the minimum and the broad-label tail.
- 261012 T675 start on main at efc7f59a: 최대 태그 수 copy becomes 최소 태그 수.
- 261012 T674 done on main: tag_count is a minimum the prompts ask for, tags kept up to 30, broad labels fill the tail, taste inferred from inputs as experience; write/revise no-voice system prompts shrank 148/163 runes.
- 261012 T674 start on main at 8fe42b0e: tag minimum, broad-label tail, inferred taste, prompt rules edited in place.
- 261012 create-task GEN r29 POST r40 TMPL r27 GUIDE r21: T674 (BE prompts, parser, 기본 지침), T675 (FE 최소 태그 수 copy); no dep between them.
- 261012 create-task GEN POST TMPL GUIDE start.
- 261012 update-ssot GEN r29 POST r40 TMPL r27 GUIDE r21 done: GEN-46✎ tag_count upper bound→minimum (≤30 kept) · GEN-50✎ broad labels fill the rest of the count · GEN-83✎ flavor/texture/aroma inferred from inputs written into the experience · GEN-85✎ rule changes edit in place · POST-63✎ TMPL-47✎ TMPL-49✎ 최대→최소 태그 수 · GUIDE-41✎ impressions 기본 지침 renamed; no doing task in the blast radius; next create-task.
- 261012 update-ssot GEN start: tags fell to 5–7 with no broad label once the count became a maximum; prose shrank under the AI-suggestion separation rules; the owner wants a tag minimum and inferred sensory description from supplied inputs, editing existing 기본 지침 instead of appending.
- 261011 T671 done on main: pick-order confirms; ① strip moves a photo by drag or 옮기기 → 여기로 / 맨 뒤로.
- 261011 T670 done on main: images.position, confirm append, ReorderImages with POST_PHOTO_ORDER_STALE.
