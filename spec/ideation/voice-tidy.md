# IDEATION voice-tidy
> st:open@260929 | 말투 is the last prompt layer left unreviewed: learn it only from prose the owner wrote, show what was learned in plain words, and let the owner check that it applies

## vision
- [o] Problem: 말투 was built early as a learning system (edit-diff 대조 규칙, 프로필 검증, 문장 의견, 블라인드 비교, versioned typed profile with six axes and provenance badges) that the owner cannot read and that never received the signal it learns from
  - measured on prod 260929, owner account, default voice `맛집 리뷰 블로거 학습`: 35 posts, 1 learning run, 1 learned source, 1 pasted sample (another blogger's post, `작성자 블린이`)
  - 28 of the 35 posts end byte-identical to the last AI output: the owner corrects through AI 수정 (12 revise runs, every instruction about facts, photo order or section order), which overwrites the machine baseline, so an edit diff has nothing to learn
  - 7 learning runs on all voices, 0 contrast rules ever created, 0 validations, 0 comparisons, 0 manual overrides
  - the one learned source is an AI-written post with ~5% hand edits — learning from it feeds the model's own prose back as "the owner's voice"
  - 문장 의견 is stored and never read by anything
- [o] Target: an owner writing Naver posts who wants them to sound like themselves — from posts they wrote by hand, or from short writing prompts when they have none
- [o] Core value: a voice holds only the owner's own prose, reads back to the owner in plain words, and can be checked side by side against the owner's own answer

## explored
- [o] check (260929, no paid call): the voice does reach the model — the write and revise system prompts carry it on every run; storyline, observe and the clip writers never see it
  - what reaches it: `[스타일가이드]` (English sub-headers, every value tagged `(measured)`/`(analyzed)`), `[활성 대조 규칙]`, `[글 예시 발췌]` up to 3 excerpts, `[사용자 규칙]`, `[종결어미 제약]`
  - the six axes reach it as bare `involvement=2 narrativity=2 persuasion_overtness=1 abstractness=-2 addressee_focus=2 humor=1`: no scale, no words, so the model guesses what 2 means
  - the measured ending mix does not steer the output (rough count over the 35 posts): profile 다 28% · 해요 31% · 습니다 7% · 기타 34%, output 다 0% · 해요 48% · 습니다 38% · 기타 14%
  - the default voice's `[사용자 규칙]` holds two content/order instructions saved through 규칙으로 저장 (입간판 사족 금지, 입구 전경 → 밑반찬 → 음식 순서): 지침 material living in the voice
  - the posts' most visible style (emoji question headings, `어쩌다 가게 됐냐면요` in 26 of 35) comes from the template, not the voice
  - an empty profile still prints its section headers with empty bodies
  - a true "does it apply" test needs the same post generated with and without the voice: paid calls, only with the owner's yes
- [o] a voice is learned from prose the owner wrote by hand, gathered when the voice is created ← owner direction; only the owner's own writing carries the owner's voice, AI output does not
  - the owner has hand-written posts ⇒ paste them
  - the owner has none ⇒ the product shows a photo or a situation and the owner describes it in writing; those answers are the samples
- [x] the owner edits the analysis field by field (pencil per field, 직접 설정, axis sliders) ← owner: too complex to hand to a user to edit
- [o] the writing prompts come from the product, one set per 분야 (the post's 분야 list): photo prompts ("이 음식을 블로그에 쓰듯 2~5문장으로") and situation prompts without a photo ("처음 가 본 식당에 들어섰을 때") ← a new owner can start at once, and the answers are the owner's own prose
- [o] after creation a voice keeps learning only from more pasted posts and more answered prompts (re-analysis → `다시 분석`) ← owner kept exactly these two
- [x] learning from a finished post (확정하고 말투 학습 / 말투 학습) and 문장 의견, which only appears after such a run ← owner choice once each was explained: material comes only from pasted posts and answered prompts; on prod 6 of the 7 learned posts were ≥ 95% the AI's own output, and 문장 의견 was read by nothing
- [x] 대조 규칙 and 블라인드 비교, which exists only to test one rule ← owner choice once explained: 말투 분석 already describes how the owner sounds, and an explicit instruction is a 지침; the edit diff it learned from is gone
- [o] 프로필 검증 becomes the owner's own check that the voice applies: one AI call writes a piece in this voice and the owner judges it ← owner direction ("ai호출1회 해서 글 생성하는식으로 사용자가 검증")
  - [o] the AI rewrites one prompt the owner answered, in this voice, and shows it beside the owner's own answer ← same subject, so only the voice differs
- [o] a voice may be marked 기본, and marking one is optional: new posts start in the 기본 voice when there is one, otherwise in 말투 없음 ← owner choice
- [o] after the voice is made, new material shows `새 학습 글 N편 · 다시 분석`; re-analysis runs only when pressed (one model call, credits) ← owner choice
- [o] no 버전 기록 tab: after a re-analysis the voice offers one `이전 분석으로 되돌리기` ← owner choice; it exists for a re-analysis the owner dislikes
- [o] no voice is created automatically, and a post may have no voice (말투 없음): without one the writer picks a voice that fits the post ← owner direction
  - retires the bootstrap `기본 말투` (VOICE-4) and "at least one active voice and exactly one default" (VOICE-2); a post's voice becomes optional (POST, GEN)
  - no voice ⇒ no voice section in the prompt at all, `[종결어미 제약]` included
- [o] gathering makes no model call: a deterministic readiness meter shows `말투 학습에 필요한 정보 N% 확보` by the product's own rule, and at 100% `이제 말투를 만들 수 있어요`; the analysis runs once, when the owner then makes the voice ← owner: no AI analysis per answer
  - [?] meter rule (proposal, numbers calibrated in the SSOT): pasted posts and answers count alike; 100% needs enough sentences for a stable ending mix and material covering an opening, a description and a closing
- [x] the 말투 설명 field at creation (a seeded profile from a wish) ← a wish is not writing; wishes belong in 지침; the one seeded voice doubled the write prompt (46 s → 81 s)
- [o] the analysis is shown read-only as 말투 분석 in its own tab ← owner direction
- [o] keep the typed profile data (ending distribution, sentence length, paragraph shape, intro/closing and heading/list/emoji habits, six axes) and keep using it in the write and revise prompts; 말투 분석 explains every value to the owner in plain words ← owner: the structure is research-derived and worth keeping, it just needs explaining
  - the six axes take their names from Biber's multidimensional analysis (1988): involvement, narrativity, overt persuasion, abstractness; addressee focus and humor are added
  - Biber's scores are computed from ~60 counted linguistic features; here the analysis model estimates -3..3 with no definition of either pole, and the writer receives bare `key=N`
- [o] every axis is defined as its source research defines it, and both the analysis model and the writer receive that definition (poles and what a value means) instead of a bare name and number ← owner: 논문에서 쓰는대로 정의를 제대로 해서 전달
  - [o] source rule: research on Korean only; a paper older than 10 years is used only when highly cited (a classic, judged by citation count); an old paper that is not well cited is dropped and the product defines its own axes instead ← owner rule
  - [?] which set passes the rule — research 260929 (web; citation counts from Google Scholar via a summarising fetch, not re-checked):
    - Kim & Biber 1994 (OUP chapter): ~126 citations, borderline; its Korean dimensions are reported in Biber 1995 *Dimensions of Register Variation* §6.3 (~2,815 citations, a classic ⇒ passes)
    - Korean dimensions (Biber 1995, read through an unofficial excerpt; loadings for D2–D6 unseen): D1 on-line interaction vs planned exposition · D2 overt vs implicit logical cohesion · D3 overt expression of personal stance · D4 narrative vs non-narrative · D5 on-line reportage of events (sportscasts, tentative) · D6 honorification
    - Biber 1995: a persuasion dimension exists only in English and Somali, an abstract-style dimension only in English ⇒ the product's persuasion, abstractness and humor axes have no Korean research behind them
    - post-2016: Kang Beomil 2024 (언어과학 31(1), 59 features, 17 registers) — 대화적/비공식적 스타일 · 학술적/격식적 담화 대 정보성 담화 · 공적 견해 표출 · 감정적 상호작용 · 서술적 이야기; the features per dimension are behind a paywall (unverified)
    - no study defines dimensions for Korean blog or review writing
    - none of these papers uses a -3..3 scale; they report factor scores
- [o] 규칙으로 저장 leaves the voice entirely; 지침으로 저장 (GUIDE-21) stays the only way to keep a revision instruction ← owner: it has nothing to do with the voice (resolves VOICE-7)
- [o] 말투 list uses 내 글's row: the whole row is one link, name + 기본 / language badges + a meta line; 기본으로 설정 and 삭제 move to the voice page ← one list design across the app
- [o] a 학습 글 (pasted post or answered prompt) opens to its full text; today the body is never returned (VOICE-8) and the row is a truncated label with only 삭제 ← owner: "상세보기 할 수가 없고 삭제만돼"
- [o] `[종결어미 제약]`'s fixed "no third identical ending in a row" becomes a 기본 지침, on for every post and switchable on the 지침 screen; "follow the measured ending mix" stays with the voice, being the analysis itself ← the system-prompt-format-only split: a fixed writing rule blocks writing that needs its opposite
- [o] the voice page has three tabs: 말투 분석 (opens first) · 학습 글 · 검증; renaming, 기본 and 삭제 sit on the title row ← owner choice
- [o] 새 말투 만들기 asks the name and the 분야 first and lists the voice at once as `만드는 중`; gathering can stop and resume; a voice below 100% cannot be picked for a post ← owner choice
- [o] an owner with no voice finds 새 말투 만들기 on the empty 말투 list and inside the post's voice picker (말투 없음 · the owner's voices · 새 말투 만들기) ← owner choice; no sign-up onboarding step

## shape
- flow create: 새 말투 만들기 → name + 분야 → voice listed as `만드는 중` → 글 붙여넣기 | 문항 풀기 (photo prompt | situation prompt) → `말투 학습에 필요한 정보 N% 확보` (no model call) → 100% → 말투 만들기 (one analysis call) → 말투 분석
- flow grow: 학습 글 → paste a post | answer more prompts → `새 학습 글 N편 · 다시 분석` → re-analysis (one call) → `이전 분석으로 되돌리기` while the previous one is kept
- flow check: 검증 → pick an answered prompt → one AI call writes it in this voice → shown beside the owner's answer
- flow write: new post → the 기본 voice, or 말투 없음 when none is 기본 → voice picker (말투 없음 · voices at 100% · 새 말투 만들기); 말투 없음 ⇒ no voice section in the prompt
- v1: 내 글-style voice list; three tabs; read-only 말투 분석 in plain words with research-defined axes, given to the writer with the same definitions; per-분야 photo and situation prompts; readiness meter; optional voice and optional 기본; 다시 분석 + one-step undo; 검증; 학습 글 readable in full; 규칙으로 저장 removed; the ending-run rule as a 기본 지침
- not: 대조 규칙, 블라인드 비교, learning from finished posts (확정하고 말투 학습 / 말투 학습), 문장 의견, 버전 기록 tab, the 말투 설명 seed, per-field editing and overrides, the auto-created 기본 말투, a sign-up onboarding step

## domains
- VOICE: what a voice is made of (pasted posts + prompt answers), `만드는 중` and the readiness meter, per-분야 prompt sets, the list row, three tabs, read-only analysis in plain words, 다시 분석 and one-step undo, 검증, optional 기본, no bootstrap voice; removes contrast rules, comparisons, finished-post learning, sentence feedback, versions tab, seed, overrides, 규칙으로 저장 text
- GEN: the voice projection in plain Korean with the axis definitions, no voice section for 말투 없음, the ending-run line leaving the voice section, 규칙으로 저장 leaving revision (GEN-39)
- POST: a post's voice becomes optional; the voice picker's 말투 없음 and 새 말투 만들기; ② loses 확정하고 말투 학습; ③ loses the 말투 학습 panel and 문장 의견
- GUIDE: a new 기본 지침 for the ending run; 지침으로 저장 is the only save beside a revision
- AUTH: adduser no longer creates a voice
- QUOTA: credits for 말투 만들기, 다시 분석 and 검증 (one call each)
- MODEL: the analyze-model experiment that runs on a voice's corpus (VOICE-49) — pending

## open
- which axis set passes the owner's source rule (research running 260929)
- the readiness meter's rule and numbers (sentences, characters, coverage of opening / description / closing)
- the prompt photos: a source the product may use [?] feasibility (licence)
- how 말투 분석 words each value for the owner (one plain sentence per item; whether a value computed by counting reads differently from one the AI judged)
- the analyze-model experiment (VOICE-49) that compares two analysis models on a voice's corpus: not yet explained to the owner
- existing voices on prod (pre-alpha, data disposable): keep pasted samples, drop learned finished posts and empty bootstrap voices?
- run the paid with/without-voice A/B?
