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
| storyline-first | converted@260927 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 12 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 9 | 9 | - | 0 |
| QUOTA | 23 | 23 | - | 0 |
| POST | 21 | 21 | - | 0 |
| VOICE | 4 | 4 | - | 1 |
| GEN | 17 | 17 | - | 0 |
| MODEL | 18 | 18 | - | 0 |
| TMPL | 15 | 15 | - | 1 |
| GUIDE | 9 | 9 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 6 | 6 | - | 0 |
| THEME | 19 | 15 | THEME-19✎ | 0 |
| MKT | 7 | 7 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 49 | 43 | CLIP-163+ | 2 |
| CDS | 29 | 29 | - | 1 |
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
| T415 | Clip ① and ② behave and speak as CLIP-14, CLIP-21/23, CLIP-39 and CLIP-121 decide | CLIP | T441 | todo |
| T416 | A caption face's coverage is the set of characters it actually draws | CDS | - | blocked@260927 |
| T429 | The direct write opens with a storyline the post keeps | GEN POST TMPL | T431 | todo |
| T432 | The 지침 screen lists the 기본 지침 first, each with 추천 and a switch | GUIDE | T431 | todo |
| T433 | 스토리라인 먼저, 다시 만들기 and the storyline request run as storyline jobs (backend) | GEN GUIDE QUOTA POST | T429 | todo |
| T434 | Writing from the storyline, and the owner's own storyline edits (backend) | GEN POST | T433 | todo |
| T435 | ①'s 스토리라인 먼저 · 바로 글 쓰기, and the storyline steps (frontend) | POST GEN | T434 | todo |
| T436 | ②'s storyline space: reading it and editing it by hand (frontend) | POST | T435 | todo |
| T437 | ②'s storyline actions: the AI request, 다시 만들기 and writing from it (frontend) | POST GEN | T436 | todo |
| T438 | 영상 지침: guidelines of the clip kind (backend) | GUIDE CLIP | T431 | todo |
| T439 | /video-guidelines, 영상 지침 후보 and 영상 지침으로 저장 (frontend) | GUIDE CLIP | T438 T432 | todo |
| T440 | Clip writing calls carry 영상 지침, and the server checks no caption's content | CLIP CDS GUIDE QUOTA | T438 T414 | todo |
| T441 | A video template carries a starting design selection, and a project takes it on selection | CLIP CDS | T443 | todo |
| T442 | The video-template preview plays an illustrative timed clip | CLIP | T441 | todo |
| T443 | Every video-template builder entry opens in place with its own delete | CLIP | T414 | todo |
| T444 | ②'s flow simulation over still cut frames | CLIP | - | todo |
| T445 | 바로 만들기 sets a storyline in its flow call and the narration follows it (backend) | CLIP | T440 | todo |
| T446 | The clip storyline call, the storyline request, building from the storyline and the owner's edits (backend) | CLIP QUOTA CDS | T445 | todo |
| T447 | Clip ①'s 스토리라인 먼저 · 바로 만들기 and ②'s storyline space for editing (frontend) | CLIP | T446 T436 | todo |
| T448 | Clip ②'s storyline actions: the AI request, 다시 만들기 and building from it (frontend) | CLIP | T447 | todo |

## next
- implement-task T429 (direct-write storyline) → T433 → T434 → T435 → T436 → T437 for the post storyline, with T432 (기본 지침 screen) and T438 → T439 · T440 → T445 → T446 → T447 → T448 for 영상 지침 and the clip storyline; independent starts T443 (→ T441 → T442, T415) and T444
- update-ssot CDS for T416 (blocked): what a caption does when its face — the default 크게 강조 included — has no ink for a syllable; update-ssot AUTH-36: name VerifyEmail among the throttled writes (T427 throttles it); implement-task T414 T415 after their refresh
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); create-task MKT THEME (/about overflow) and CLIP CDS THEME (Wanted Sans delta); ARCH-5's context list is stale (lacks clip, quality, voucher and others); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; finding (T400): Paperlogy's cmap maps all 11,172 syllables but draws 8,392 empty (e.g. 갂), so the cmap-based faceCoverage passes them and a Paperlogy caption renders them blank; update-ssot POST-39 (DeleteImage drops the row before the object since T358, see T411 result); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results)

## log
- 260928 T431 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260928 T431 claimed (ia)
- 260928 T430 done
- 260928 T430 claimed (ia)
- 260928 T414 done: decisions in /IMPLEMENTATION-DECISIONS.md
- 260927 T414 claimed (ia)
- 260927 create-task GUIDE GEN TMPL POST CLIP CDS QUOTA done (sl): T431–T448 created, T429 T430 rewritten (flow-first → stored storyline; builder copy → template form only), T414 T415 refreshed to CLIP@49 (guide entry removed, r47's preview selectors kept, template design copy); GUIDE r9 GEN r17 TMPL r15 POST r21 QUOTA r23 CDS r29 fully tasked, CLIP tasked to r43 with CLIP-163+ still open
- 260927 CLIP r41 CLIP-13✎ and CDS r24 CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ consumed without a task: implemented in b18e73e8 (Wanted Sans, the glyph fallback and refusal); T416 stays blocked on its own CDS question
- 260927 create-task GUIDE GEN TMPL POST CLIP CDS QUOTA start (sl): GUIDE r9 GEN r17 TMPL r15 POST r21 CLIP r49 (with r41/r47/r48 pending) CDS r29 (with r24/r28 pending) QUOTA r23; refresh T429 T430 T414 T415, T428 (doing, ia) untouched
- 260927 warning (sl → ia): T428 (doing) implements TMPL-21, which r15 rewords — photo places are filled along the storyline the writer follows (→GEN-67 →GEN-70) instead of the flow it sets, and `write`/`note` tags become `write` tags as `<note>` leaves the grammar; finish T428 on its base, the rest lands through create-task
- 260927 update-ssot GUIDE GEN TMPL POST CLIP CDS QUOTA done (sl): GUIDE r9 GEN r17 TMPL r15 POST r21 CLIP r49 CDS r29 QUOTA r23 — ideation storyline-first converted; todo T429 T430 T414 T415 need a refresh
- 260927 T428 done: on its base per sl's warning; the legend keeps 흐름/요구하는 until the storyline task
- 260927 T428 claimed (ia)
- 260927 T427 done: AUTH-36's list lacks VerifyEmail, now throttled (update-ssot AUTH)
- 260927 T427 claimed (ia)
- 260927 T426 done
- 260927 T426 claimed (ia)
- 260927 update-ssot GUIDE GEN TMPL POST CLIP QUOTA start (sl): from ideation/storyline-first — system prompt = format only, 기본 지침 + 영상 지침, template = form, two paths with a storyline in ②
- 260927 ideation storyline-first ready: system prompt = format only, 기본 지침 (추천, on by default, account switch) + 지침 / 영상 지침 = direction incl. grounding, template = form (note/guide gone, a place says what it is about); ①'s 스토리라인 먼저 · 바로 글 쓰기 for posts and clips, the storyline in ②'s own collapsible space with photos per paragraph; clip server content checks removed
- 260927 T425 done: out of scope, .env.production.example still lists PURPOSE_*/VITE_PURPOSE_* ceilings no code reads
