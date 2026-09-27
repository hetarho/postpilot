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
| QUOTA | 23 | 22 | QUOTA-13✎ QUOTA-45✎ | 0 |
| POST | 21 | 20 | POST-18✎ POST-44✎ POST-46✎ POST-48✎ POST-52✎ POST-53✎ POST-54✎ POST-74✎ POST-86✎ POST-95+ POST-96+ POST-97+ POST-98+ POST-99+ | 0 |
| VOICE | 4 | 4 | - | 1 |
| GEN | 17 | 16 | GEN-5✎ GEN-14✎ GEN-15✎ GEN-16✎ GEN-17✎ GEN-25✎ GEN-30✎ GEN-40✎ GEN-44✎ GEN-47✎ GEN-49✎ GEN-50✎ GEN-67✎ GEN-68+ GEN-69+ GEN-70+ GEN-71+ GEN-72+ | 0 |
| MODEL | 18 | 18 | - | 0 |
| TMPL | 15 | 14 | TMPL-1✎ TMPL-18✎ TMPL-19✎ TMPL-20✎ TMPL-21✎ TMPL-36✎ TMPL-41✎ TMPL-43✎ TMPL-44✎ TMPL-45✎ TMPL-50✎ TMPL-52✎ TMPL-57+ | 1 |
| GUIDE | 9 | 8 | GUIDE-1✎ GUIDE-2✎ GUIDE-4✎ GUIDE-5✎ GUIDE-7✎ GUIDE-10✎ GUIDE-11✎ GUIDE-13✎ GUIDE-14✎ GUIDE-15✎ GUIDE-16✎ GUIDE-17✎ GUIDE-18✎ GUIDE-19✎ GUIDE-20✎ GUIDE-26✎ GUIDE-37✎ GUIDE-41+ GUIDE-42+ GUIDE-43+ GUIDE-44+ GUIDE-45+ | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 6 | 6 | - | 0 |
| THEME | 19 | 15 | THEME-19✎ | 0 |
| MKT | 7 | 7 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 49 | 40 | CLIP-13✎ CLIP-163+ CLIP-4✎ CLIP-14✎ CLIP-42✎ CLIP-111✎ CLIP-130✎ CLIP-139✎ CLIP-166+ CLIP-167+ CLIP-168+ CLIP-169+ CLIP-170+ CLIP-171+ CLIP-172+ CLIP-53✎ CLIP-173+ CLIP-174+ CLIP-175+ CLIP-176+ CLIP-3✎ CLIP-5✎ CLIP-11✎ CLIP-31✎ CLIP-36✎ CLIP-38✎ CLIP-39✎ CLIP-40✎ CLIP-59✎ CLIP-61✎ CLIP-63- CLIP-64✎ CLIP-69✎ CLIP-72✎ CLIP-90✎ CLIP-93✎ CLIP-112✎ CLIP-121✎ CLIP-122- CLIP-123✎ CLIP-131✎ CLIP-133✎ CLIP-134✎ CLIP-135✎ CLIP-136✎ CLIP-137- CLIP-141✎ CLIP-160✎ CLIP-177+ CLIP-178+ CLIP-179+ CLIP-180+ CLIP-181+ CLIP-182+ CLIP-183+ CLIP-184+ CLIP-185+ | 2 |
| CDS | 29 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ CDS-61✎ CDS-1✎ CDS-37✎ CDS-42✎ CDS-63- | 1 |
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
| T414 | Template authoring, the 형식 안내 and paste follow the outline grammar alone | CLIP | T413 | todo |
| T415 | Clip ① and ② behave and speak as CLIP-14, CLIP-21/23, CLIP-39 and CLIP-121 decide | CLIP | - | todo |
| T416 | A caption face's coverage is the set of characters it actually draws | CDS | - | blocked@260927 |
| T428 | Template photo places bind no photo and a repeat renders once | TMPL GEN | T424 | todo |
| T429 | The write sets the day's flow first and places every photo along it | GEN | T428 | todo |
| T430 | The template builder describes photo places the writer fills | TMPL | T425 | todo |

## next
- create-task GUIDE GEN TMPL POST CLIP CDS QUOTA (GUIDE r9 GEN r17 TMPL r15 POST r21 CLIP r49 CDS r29 QUOTA r23, from ideation/storyline-first, with CLIP r47–r48's pending design selection, template preview and flow simulation): system prompt = format only; 기본 지침 (추천, on, account switch) + 영상 지침 with its candidates at /video-guidelines; template = form (`<note>` and the video guide gone — stored bodies drop them, no compat path); ①'s 스토리라인 먼저 · 바로 글 쓰기 (A/B in its menu) / 바로 만들기; ②'s storyline space; no server check on a caption's content — refresh T429 (flow→stored storyline) T430 (flow wording) T414 (guide entry) T415 (CLIP-121) before they are implemented; T428 (doing) keeps its base and TMPL-21's storyline wording lands in a new task
- update-ssot CDS for T416 (blocked): what a caption does when its face — the default 크게 강조 included — has no ink for a syllable; update-ssot AUTH-36: name VerifyEmail among the throttled writes (T427 throttles it); implement-task T414 T415 after their refresh
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); create-task MKT THEME (/about overflow) and CLIP CDS THEME (Wanted Sans delta); ARCH-5's context list is stale (lacks clip, quality, voucher and others); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; finding (T400): Paperlogy's cmap maps all 11,172 syllables but draws 8,392 empty (e.g. 갂), so the cmap-based faceCoverage passes them and a Paperlogy caption renders them blank; update-ssot POST-39 (DeleteImage drops the row before the object since T358, see T411 result); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results)

## log
- 260927 warning (sl → ia): T428 (doing) implements TMPL-21, which r15 rewords — photo places are filled along the storyline the writer follows (→GEN-67 →GEN-70) instead of the flow it sets, and `write`/`note` tags become `write` tags as `<note>` leaves the grammar; finish T428 on its base, the rest lands through create-task
- 260927 update-ssot GUIDE GEN TMPL POST CLIP CDS QUOTA done (sl): GUIDE r9 GEN r17 TMPL r15 POST r21 CLIP r49 CDS r29 QUOTA r23 — ideation storyline-first converted; todo T429 T430 T414 T415 need a refresh
- 260927 T427 done: AUTH-36's list lacks VerifyEmail, now throttled (update-ssot AUTH)
- 260927 T427 claimed (ia)
- 260927 T426 done
- 260927 T426 claimed (ia)
- 260927 update-ssot GUIDE GEN TMPL POST CLIP QUOTA start (sl): from ideation/storyline-first — system prompt = format only, 기본 지침 + 영상 지침, template = form, two paths with a storyline in ②
- 260927 ideation storyline-first ready: system prompt = format only, 기본 지침 (추천, on by default, account switch) + 지침 / 영상 지침 = direction incl. grounding, template = form (note/guide gone, a place says what it is about); ①'s 스토리라인 먼저 · 바로 글 쓰기 for posts and clips, the storyline in ②'s own collapsible space with photos per paragraph; clip server content checks removed
- 260927 T425 done: out of scope, .env.production.example still lists PURPOSE_*/VITE_PURPOSE_* ceilings no code reads
- 260927 T425 claimed (ia): base TMPL@12→14 (TMPL-18/21/38/40 photo-place meaning is T428/T430's)
- 260927 T424 done
- 260927 T424 claimed (ia): base TMPL@12→14 GEN@14→16 (TMPL-22/37, GEN-1/40 unchanged)
- 260927 T423 done
- 260927 T423 claimed (ia): base POST@18→20 (r19/r20 no code impact)
- 260927 T422 done
- 260927 T422 claimed (ia)
- 260927 T421 done
- 260927 T421 claimed (ia)
- 260927 T420 done
