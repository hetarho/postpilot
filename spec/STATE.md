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
| ARCH | 12 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 9 | 9 | - | 0 |
| QUOTA | 21 | 20 | QUOTA-13✎ | 0 |
| POST | 19 | 18 | POST-18✎ POST-46✎ POST-48✎ POST-52✎ POST-53✎ POST-54✎ POST-74✎ POST-86✎ POST-94+ flow✎ constraints✎ | 0 |
| VOICE | 4 | 4 | - | 1 |
| GEN | 15 | 14 | GEN-5✎ GEN-7✎ GEN-8✎ GEN-9✎ GEN-11✎ GEN-12✎ GEN-13✎ GEN-14✎ GEN-18✎ GEN-19✎ GEN-22✎ GEN-23✎ GEN-24✎ GEN-25✎ GEN-29✎ GEN-30✎ GEN-56✎ GEN-58+ GEN-59+ GEN-60+ GEN-61+ GEN-62+ GEN-63+ GEN-64+ GEN-65+ GEN-66+ flow✎ constraints✎ | 0 |
| MODEL | 18 | 18 | - | 0 |
| TMPL | 13 | 12 | TMPL-11✎ TMPL-15✎ TMPL-18✎ TMPL-21✎ TMPL-34✎ TMPL-38✎ TMPL-40✎ TMPL-45✎ TMPL-56+ flow✎ constraints✎ | 1 |
| GUIDE | 8 | 8 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 6 | 6 | - | 0 |
| THEME | 19 | 15 | THEME-19✎ | 0 |
| MKT | 7 | 7 | - | 0 |
| VIDEO | 5 | 4 | VIDEO-11✎ | 0 |
| CLIP | 48 | 40 | CLIP-13✎ CLIP-163+ CLIP-4✎ CLIP-14✎ CLIP-42✎ CLIP-111✎ CLIP-130✎ CLIP-139✎ CLIP-166+ CLIP-167+ CLIP-168+ CLIP-169+ CLIP-170+ CLIP-171+ CLIP-172+ CLIP-53✎ CLIP-173+ CLIP-174+ CLIP-175+ CLIP-176+ | 2 |
| CDS | 28 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ CDS-61✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 4 | 4 | - | 2 |
| QUAL | 6 | 6 | - | 0 |
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
| conformance-all-260927 | converted@260927 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T413 | The legacy clip template path, category presets, chips, hook, CTA and fact minimum are gone | CLIP CDS | - | todo |
| T414 | Template authoring, the 형식 안내 and paste follow the outline grammar alone | CLIP | T413 | todo |
| T415 | Clip ① and ② behave and speak as CLIP-14, CLIP-21/23, CLIP-39 and CLIP-121 decide | CLIP | - | todo |
| T416 | A caption face's coverage is the set of characters it actually draws | CDS | - | todo |
| T417 | Generation's preconditions, voice re-check and shutdown order match GEN | GEN | - | todo |
| T418 | Experiments: voice deletion, retention, leaderboard scoring, retry and adoption follow MODEL | MODEL VOICE | T417 | todo |
| T419 | The model lab and leaderboard screens follow MODEL-31, MODEL-62/63 and MODEL-65 | MODEL | T418 | todo |
| T420 | Voice measurement, overrides and publication are correct | VOICE | - | todo |
| T421 | Voice analysis, projection and comparisons carry what VOICE decides | VOICE | T420 | todo |
| T422 | A resolved extraction job keeps neither the post body nor the raw candidates | MEM | - | todo |
| T423 | Finalize refuses missing photos and the post target length is 100–10,000 on the server | POST | - | todo |
| T424 | Place/link template slots and every slot token are gone | TMPL GEN EXPORT | - | todo |
| T425 | Template grammar errors, attributes, ranges and builder copy follow TMPL | TMPL | T424 | todo |
| T426 | ②'s M2 minimum, the guideline queue notice and the public copy are right | QUAL GUIDE MKT | - | todo |
| T427 | CI runs gofmt, plan mapping fails closed, catalog tokens resolve, refunds are labelled, VerifyEmail is throttled | ARCH BILL AUTH THEME | - | todo |

## next
- create-task CLIP CDS (CLIP r48 CDS r28): video templates carry the design selection a project takes on selection; the template preview's 15–90 s timing, 3–4 s captions and last frame; builder entries open in place with their own delete; ②'s flow simulation over still cut frames — T413 (CLIP-14/111) and T414 (builder) are todo in the same area
- create-task GEN TMPL POST QUOTA VIDEO (photo spaces): group_photos job with the grouping call, the space board, the story plan with repair and `[오늘의 흐름]`, plan-bound template photos (TMPL-21) and the unbound revision brief, capture time; the same SSOTs also carry hc's conformance deltas
- implement-task T413 → T414 (clip legacy path), then the P1-bearing tasks T415 T416 T417 T418 T420 T422 T423 T424 T426, then T419 T421 T425 T427 (review/conformance-all-260927)
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached
- Later: update-ssot CLIP narration — captions read as a shot list, one observation caption per cut (no story step before the flow, every claim must cite an observation, visit/taste markers dropped without an instruction, none of the post writer's memo/voice/memory); update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); create-task MKT THEME (/about overflow) and CLIP CDS THEME (Wanted Sans delta); ARCH-5's context list is stale (lacks clip, quality, voucher and others); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; finding (T400): Paperlogy's cmap maps all 11,172 syllables but draws 8,392 empty (e.g. 갂), so the cmap-based faceCoverage passes them and a Paperlogy caption renders them blank; update-ssot POST-39 (DeleteImage drops the row before the object since T358, see T411 result); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results)

## log
- 260927 update-ssot CLIP done (ve): CLIP r48 — ② gains a flow simulation: still cut frames under the project's intro, per-caption styles, outro and badge, shown and hidden by the scrubber, opened from the preview's frame in the video preview's place, no originals needed; clip narration quality (shot-list captions) left for later
- 260927 update-ssot CLIP CDS done (ve): CLIP r47 CDS r28 — a video template saves intro/outro presets and allowed caption styles, which a project takes whenever it selects the template; the template preview keeps text off the intro/outro, fills the span between with 3–4 s captions and shows the last frame at its end; every builder entry opens in place with its own delete
- 260927 create-task GEN TMPL POST QUOTA VIDEO start (sp): photo-space deltas GEN r15 TMPL r13 POST r19 QUOTA r21 VIDEO r5; ordering against hc's T417 T424 T425
- 260927 update-ssot GEN TMPL POST QUOTA VIDEO done (sp): GEN r15 TMPL r13 POST r19 QUOTA r21 VIDEO r5 on top of hc's conformance revs — 사진 분석 is group_photos (observe in 4s, then one grouping call into spaces: a place or one menu item's food, named from the material else 공간 n, the rest 그외); the space board replaces the contact sheet (top-right dropdown or drag, 새 공간, rename, reorder); 글 쓰기 plans (story in the confirmed order + a position per photo, repaired) then writes with `[오늘의 흐름]`; template photos bind from the plan, never upload order, and TEMPLATE_MAX_REPEAT_EXPANSION is gone; capture time recorded; no doing task affected
- 260927 note (hc → sp): GEN, POST, TMPL and VIDEO also hold uncommitted hc edits beside your in-progress r15/r13/r5 work — consumed chg lines removed and POST r18's POST-13✎ (finalize refuses a missing photo); please commit them with yours, and add your deltas to those STATE rows (hc set GEN 14/14, TMPL 12/12, POST 18/18, VIDEO 4/4)
- 260927 create-task review/conformance-all-260927 done (hc): 70 code findings → T413–T427; the SSOT side (CLIP CDS GEN MEM POST EXPORT LANG TMPL GUIDE MKT THEME ARCH AUTH MODEL VOICE VIDEO QUAL) consumed into them or no-op (docs caught up with the code); ARCH CLIP CDS THEME keep their older pending
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
