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
| ARCH | 11 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 8 | 8 | - | 0 |
| QUOTA | 20 | 20 | - | 0 |
| POST | 16 | 16 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 13 | 13 | - | 0 |
| MODEL | 17 | 17 | - | 0 |
| TMPL | 11 | 11 | - | 1 |
| GUIDE | 7 | 7 | - | 0 |
| EXPORT | 5 | 5 | - | 0 |
| LANG | 5 | 5 | - | 0 |
| THEME | 18 | 15 | THEME-19✎ | 0 |
| MKT | 6 | 6 | - | 0 |
| VIDEO | 3 | 3 | - | 0 |
| CLIP | 45 | 40 | CLIP-13✎ CLIP-163+ | 2 |
| CDS | 26 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 3 | 3 | - | 2 |
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

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T412 | Frontend, styles and lint scripts cite current spec decisions instead of deleted docs and IDs | ARCH THEME | T407 | doing@260926.hc |

## next
- implement-task T412 is in progress
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached
- Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); create-task MKT THEME (/about overflow) and CLIP CDS THEME (Wanted Sans delta); ARCH-5's context list is stale (lacks clip, quality, voucher and others); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; finding (T400): Paperlogy's cmap maps all 11,172 syllables but draws 8,392 empty (e.g. 갂), so the cmap-based faceCoverage passes them and a Paperlogy caption renders them blank; update-ssot POST-39 (DeleteImage drops the row before the object since T358, see T411 result)

## log
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
- 260926 docs-current (hc): spec/ssot/PUB.md and spec/legacy/ deleted; tasks/done, review/ and ideation/ stay as work records (FORMAT/skills keep their archive rules); ARCH-35 drops spec/legacy
- 260926 T405 done (rp): GetClipRegionPresetSamples draws every intro/outro preset through the region layout and overlay with the caller's `{n}` label (≤ 16 chars) numbering each slot, ids prefixed per preset, no scrim, preview lock/timeout, private no-store; ① offers each region as a radiogroup of tiles (renderer drawing on the media ground above the name, arrows move focus and choice, names alone while loading or on failure); 15 preset names + slot label ko/en; CompositionDesignThumbnail removed; BE/FE gates green, gen:proto clean
- 260926 T405 claimed (rp)
- 260926 T404 done (rp): new projects store intro a / outro b (DefaultDesign) while an empty id anywhere it is stored keeps rendering b/e (UnchosenDesign) and migration 0086 writes b/e into every empty project row; ① receives resolved ids, FE preset types are design.json key unions and fall back through CLIP_DEFAULT_REGION_PRESETS; BE/FE gates green, gen:sql clean
- 260926 T404 claimed (rp)
