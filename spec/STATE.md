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
| searchable-details | open@260926 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 12 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-5✎ ARCH-31✎ | 0 |
| AUTH | 9 | 8 | AUTH-17✎ AUTH-36✎ | 0 |
| QUOTA | 20 | 20 | - | 0 |
| POST | 17 | 16 | POST-20✎ POST-26✎ POST-27✎ POST-29✎ POST-39✎ POST-41✎ POST-61✎ POST-73✎ POST-75✎ flow✎ constraints✎ | 0 |
| VOICE | 4 | 3 | VOICE-13✎ VOICE-38✎ VOICE-39✎ VOICE-40✎ VOICE-42✎ VOICE-46✎ VOICE-54✎ constraints✎ | 1 |
| GEN | 14 | 13 | GEN-1✎ GEN-3- GEN-5✎ GEN-23✎ GEN-25✎ GEN-26✎ GEN-30✎ GEN-37✎ GEN-38✎ GEN-40✎ flow✎ constraints✎ | 0 |
| MODEL | 18 | 17 | MODEL-44✎ constraints✎ MODEL-28✎ MODEL-34✎ MODEL-37✎ MODEL-38✎ MODEL-45✎ MODEL-62✎ MODEL-63✎ | 0 |
| TMPL | 12 | 11 | TMPL-6✎ TMPL-10✎ TMPL-17✎ TMPL-19✎ TMPL-20✎ TMPL-21✎ TMPL-22✎ TMPL-23- TMPL-24✎ TMPL-26✎ TMPL-27✎ TMPL-30✎ TMPL-33✎ TMPL-35✎ TMPL-37✎ TMPL-49✎ constraints✎ | 1 |
| GUIDE | 8 | 7 | constraints✎ | 0 |
| EXPORT | 6 | 5 | EXPORT-4- | 0 |
| LANG | 6 | 5 | LANG-6✎ LANG-7✎ | 0 |
| THEME | 19 | 15 | THEME-19✎ THEME-38✎ THEME-26✎ THEME-6✎ THEME-29✎ constraints✎ | 0 |
| MKT | 7 | 6 | MKT-9✎ | 0 |
| VIDEO | 4 | 3 | VIDEO-6✎ | 0 |
| CLIP | 46 | 40 | CLIP-13✎ CLIP-163+ CLIP-3✎ CLIP-42✎ CLIP-111✎ CLIP-116✎ CLIP-129✎ CLIP-146✎ CLIP-70- CLIP-101- CLIP-114- CLIP-140- CLIP-144- constraints✎ | 2 |
| CDS | 27 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ CDS-15✎ CDS-31✎ CDS-60✎ constraints✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 4 | 3 | MEM-7✎ MEM-15✎ MEM-16✎ MEM-19✎ flow✎ constraints✎ | 2 |
| QUAL | 5 | 5 | - | 0 |
| GIFT | 2 | 2 | - | 0 |

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
| conformance-all-260927 | open@260927 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|

## next
- review-code clip: code still implementing retired CDS/CLIP decisions (see T411 result)
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached
- Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); create-task MKT THEME (/about overflow) and CLIP CDS THEME (Wanted Sans delta); ARCH-5's context list is stale (lacks clip, quality, voucher and others); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; finding (T400): Paperlogy's cmap maps all 11,172 syllables but draws 8,392 empty (e.g. 갂), so the cmap-based faceCoverage passes them and a Paperlogy caption renders them blank; update-ssot POST-39 (DeleteImage drops the row before the object since T358, see T411 result); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results)

## log
- 260927 update-ssot conformance done (hc): CLIP r46 CDS r27 GEN r14 MEM r4 POST r17 EXPORT r6 LANG r6 TMPL r12 GUIDE r8 MKT r7 THEME r19 ARCH r12 AUTH r9 MODEL r18 VOICE r4 VIDEO r4 carry the SSOT side of review/conformance-all-260927 (55 findings closed as SSOT-follows-code; 69 code findings open; F2 F55 await the owner)
- 260927 update-ssot GEN TMPL POST start (sp): photos grouped into spaces after observation, owner reviews the groups before writing, the write builds the day's story first, template photo positions follow the confirmed spaces instead of attachment order
- 260927 update-ssot conformance start (hc): SSOT side of review/conformance-all-260927 across CLIP CDS GEN MEM POST EXPORT LANG TMPL GUIDE MKT THEME ARCH AUTH MODEL VOICE; F2 and F55 await the owner
- 260927 review-code conformance-all-260927 start (hc): every SSOT domain against the code; code≠SSOT is the owner's top priority
- 260926 T412 done (hc): frontend, index.html, styles and lint-style-escapes cite current decisions: 61 legacy doc paths → POST/TMPL/VOICE/MODEL/GEN/GUIDE/MKT ids; ~255 design-language refs incl. bare §N → THEME-n by claim, ARCHITECTURE § → ARCH-13/17; 95 TEMPLATE-n/MARKETING-n → TMPL-n/MKT-n; 19 retired CDS/VIDEO/ARCH/QUAL ids → current ids or dropped; ~180 job/plan/change/AC refs → current ids; the lint:style failure message names THEME-11/15/19/29; unused hasVideoBlock deleted; comment-only otherwise; FE gates green
- 260926 T411 done (hc): backend, proto and RENDER.md cite current decisions: 13 legacy doc paths → TMPL-17..22/TMPL-2, POST-17, MODEL-9+GEN-22; 90 retired CDS/CLIP/PUB ids → current ids or dropped (TestPresetsMatchCDS50 → TestCategoryPresetsKeepTheirValues); ~180 job/plan/change refs → current ids; 74 TEMPLATE-n/BILLING-n → TMPL-n/BILL-n; RENDER.md draws the intro/outro regions (CDS-70); comment-only, migrations untouched; the task result lists clip code still implementing retired decisions; BE gates green, gen:proto and gen:sql clean
- 260926 T411 T412 claimed (hc)
- 260926 T410 done (hc): no phrase batch, Naver search client or phrase config remain: internal/naversearch, quality batch/phrases/stopwords, the PhraseLists/BlogSearch ports, Field.Query, the phrase config and env keys, and the devseed/seed phrase list are deleted; migration 0089 drops field_phrase_lists (Down restores 0077's DDL empty); quality/boundary_test pins QUAL-47 over every module package (migrations aside) and drops the guideline/naversearch entries; a cmd/api boot test serves /health with the retired settings set and no quality goroutine or phrase log; BE gates green, gen:sql and gen:proto clean
- 260926 T410 claimed (hc)
- 260926 T409 done (hc): the guideline preset is gone: ListGuidelines answers the owner's guidelines alone and ForPrompt returns their ordered texts ([]string, no forRevision) with nothing appended; UpdateGuidelinePreset and its messages are deleted and ListGuidelinesResponse reserves 2/"preset"; migration 0088 drops guideline_preset_fields and guideline_presets (Down restores 0078's DDL empty); BE/FE gates green, gen:proto and gen:sql clean
- 260926 T409 claimed (hc)
- 260926 T408 done (hc): the write freezes no 분야 phrases and no preset line (guidelines = owner texts), its answer and the post carry nouns only, SavePostContent takes no taken indices, migration 0087 drops posts.replacement_candidates, post.proto reserves 30/5 and drops ReplacementSurface/ReplacementCandidate; legacy payload, snapshot and candidate-output keys decode as absent; BE/FE gates green, gen:proto and gen:sql clean
- 260926 T408 claimed (hc)
- 260926 T407 done (hc): ② renders plain prose with no replacement marks or takes, /guidelines drops the preset row, and the FE stops reading replacement_candidates/preset and sending taken_candidates/UpdateGuidelinePreset; the queue, autosave, BlockEditor and BlockList return to their pre-T351/T363 form; FE gates green
- 260926 T407 claimed (hc)
- 260926 T406 done (rp): the TS port mirrors Go LayoutRegion for all 15 presets (decoration, chips, list, stamp arcs, rotation) against a 135-case fixture; CompositionDesignFrame draws shapes, textPath arcs, outline text and the turn group; the template preview offers intro/outro presets (preview only); checked in Chrome; FE/BE gates green
- 260926 create-task QUAL GEN GUIDE POST done (hc): QUAL r5 GEN r13 GUIDE r7 POST r16 → T407 (FE marks + preset row) → T408 (frozen phrases, candidates, post.proto, drop column) → T409 (preset, guideline.proto, drop tables) → T410 (phrase batch, naversearch, config, drop table); T411/T412 cite current spec in BE/FE code; T400–T405 restored to tasks/done after deletion at done
- 260926 create-task QUAL GEN GUIDE POST start (hc): QUAL r5 GEN r13 GUIDE r7 POST r16 (분야 phrase feature removal) + stale spec citations in code
- 260926 T406 claimed (rp)
- 260926 create-task T406 (rp): the owner asked for every preview to draw the new presets; the TS port and CompositionDesignFrame drew the four defaults only and the template preview had no preset choice
