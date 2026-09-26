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
| T407 | The frontend drops ②'s replacement marks and the 상위 노출 단어 사용 preset row | POST GUIDE | - | todo |
| T408 | Generation stops freezing 분야 phrases and posts stop carrying replacement candidates | GEN POST | T407 | todo |
| T409 | The 상위 노출 단어 사용 guideline preset leaves the backend and the guideline proto | GUIDE GEN | T408 | todo |
| T410 | The 분야 phrase batch, its Naver search client and its table are removed | QUAL | T409 | todo |
| T411 | Backend, proto and build docs cite current spec decisions instead of deleted docs and IDs | ARCH | T410 | todo |
| T412 | Frontend, styles and lint scripts cite current spec decisions instead of deleted docs and IDs | ARCH THEME | T407 | todo |

## next
- implement-task T407 → T408 → T409 → T410 (분야 phrase feature removal, FE first so the proto removal cannot break it); T412 after T407 and T411 after T410 (code citations of deleted docs and IDs)
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached
- Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); create-task MKT THEME (/about overflow) and CLIP CDS THEME (Wanted Sans delta); ARCH-5's context list is stale (lacks clip, quality, voucher and others); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width

## log
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
- 260926 T403 done (rp): intro cover/serif/frame/outline/lower/sticker and outro credits/sidebar/chips/list/stamp drawn, verified and admitted per CDS-89..99 through overlay region-v2 (shapes, textPath arcs, one rotation group, radial scrim); decoration is manifest `plate` parts named by kind, V20 checks them and each part's turn; every entry of a block samples the block's text and only its owner draws the CDS-32 scrim; a left-set block's measure stops at the safe edge; assets-v3; BE/FE gates, host region smokes, pnpm smoke:media green
- 260926 T403 claimed (rp)
- 260926 T402 done (rp): the writer is told each generated region row's slot (role, size, floor, lines, max_syllables from RegionSlotBudget) by its CLIP-147 slot, the repair keeps a row that shrinks or wraps and replaces only an Over one, ① bounds a region-bound answer by the slot's syllable budget on the project ratio, slot notices no longer say one line; BE and FE gates green
- 260926 T402 claimed (rp)
- 260926 T401 done (rp): region presets are anchored gap stacks fitted by width (design.LayoutRegion, V20 recomputes the whole region), A/B/outro B/E restated by CDS gaps with short text within 6 px of the old baselines, browser preview on a TS port held to a Go fixture, assets-v2 + digest incl. metrics.json; BE/FE gates, host and image region smokes, pnpm smoke:media green
- 260926 T401 claimed (rp)
- 260926 T400 done (rp): design/metrics.json (+FE clip-metrics.json mirror) generated by resvg for paperlogy 800, wantedsans 600/700, nanummyeongjo 800, jua 400; TextWidth/Covers/InkOf and FitRegionSlot/RegionSlotBudget; resvg cross-check within 0.5 % Hangul, ≤ 3 % over on kerned digits; BE and FE gates green
- 260926 finding (rp): Paperlogy maps all 11,172 syllables in its cmap but draws 8,392 of them empty (e.g. 갂), so media faceCoverage (cmap-based) passes them and a Paperlogy caption with such a syllable renders it blank instead of falling back under CDS-84; the new metrics table already excludes them for region slots
- 260926 T400 claimed (rp)
- 260926 create-task CDS CLIP done (rp): CDS r26 + CLIP r45 → T400 (metrics table + width fit), T401 (gap stack layout for A/B/B/E across renderer, V20, admission and FE preview), T402 (writer budgets, repair and answer limits by width), T403 (11 presets, decoration, region scrim), T404 (defaults A/B, existing projects keep b/e), T405 (① radiogroup of renderer-drawn slot drawings); CDS r24 and CLIP-13✎ CLIP-163+ stay pending
