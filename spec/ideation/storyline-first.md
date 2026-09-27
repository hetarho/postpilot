# IDEATION storyline-first
> st:converted@260927 | Posts and clips tell a story instead of describing each source: after observation the owner either has a storyline written first or writes directly, and the system prompt, the guideline and the template each own one question

## vision
- [o] Problem: posts and clips observe well and describe what they observed, but carry no storyline; a post partly holds together, while a clip's captions are one observation per cut (a shot list, measured on prod 260927: 8 captions for 8 cuts, each citing exactly one observation)
- [o] Target: an owner turning one visit's photos and videos into a Naver post or a short clip
- [o] Core value: the result reads as a story a person told about the visit, not as a description of each photo or cut
- [o] Means: prompt engineering, plus an AI in the role of a professional writer that makes the plot before anything is written

## explored
- [o] once every source is in and observed, the owner chooses one of two paths: storyline first (스토리라인 먼저) or write directly (바로 글 쓰기) ← owner direction
- [o] posts and clips share the same two-path UX ← owner direction
- [o] write directly is today's workflow, prompt-engineered to tell a story from the raw sources
- [o] a storyline is prose that says how the story goes, in order: e.g. "먼저 외관 사진들을 보여주며 카페에 들어가는 길을 보여줍니다. 그리고 xxx, xxx, xxx 같은 정보를 보여줍니다. 그 이후에는 음료를 어떻게 주문하는지를 보여주면서 설명합니다 …"
- [o] each authored layer owns one question: the system prompt makes input and output fit the system reliably; the guideline (지침) says what kind of writing is wanted, including the rules that make a storyline (e.g. place things in time order); the template holds only the post's form, so instruction-like parts such as `<note>` leave it ← today the three overlap: the system prompt carries writing-quality rules (grounding, altitude, naming, title and tag rules), and a template carries model-only instructions beside its form
- [o] the storyline call is opt-in and the direct path keeps one write call ← on 260927 the owner rejected an always-on plan call for posts as too costly for posts of this length; an owner choosing the extra call for one post is a different trade
- [o] both paths use the same storyline form; write directly stays ONE AI call, setting the storyline inside that call (today's `flow`) and writing along it, while storyline first is a separate call that stops for the owner before anything is written ← owner 260927
- [o] write directly keeps the storyline it set in its one call and shows it with the result, with no extra call; the owner can change it and write again from it ← owner 260927: both paths then meet at the same place
- [x] discarding the direct write's storyline as today ← a post the owner dislikes would need a storyline made again from nothing
- [o] the storyline is read and changed in ② (글 다듬기 · 클립 수정), not in ①; storyline first opens ② on the storyline before any draft exists ← owner 260927
- [o] in ② the storyline is its own space that opens and closes, with its own AI revision request inside it ← owner 260927: the storyline is a draft (초안), so it lives apart from the post or clip being refined
- [o] the storyline changes only when the owner changes it in its own space; refining the post or clip in ② (by hand or through ②'s revision composer) never rewrites the storyline ← owner 260927: once written, the post is what gets refined, and keeping the draft in step with it buys nothing
- [x] the storyline inside ① under the inputs, or as its own step between ① and ② ← not chosen by the owner
- [o] the owner changes a storyline three ways: edit its text by hand, ask the AI to revise it, or regenerate it from the same material ← owner 260927
- [x] writing a storyline from scratch without the AI ← not chosen by the owner; a storyline always starts from the AI's draft
- [o] the system prompt keeps the input and output format only: the answer shape, attached filenames only, the block rules ← owner 260927: every rule about what may be written is direction, and a system rule would forbid a later kind of writing (fiction) that needs the opposite
- [o] every writing-direction rule becomes a product-owned 기본 지침 (default guideline): altitude (observations are not a list to describe), the two title prohibitions, the tag rule, the Korean naturalness baseline, and the new story rules (e.g. time order) ← owner 260927
- [o] a 기본 지침 is marked (추천) and selected by default, and the owner can deselect it ← owner 260927: the product recommends a direction without forcing it
- [o] grounding (state no fact the material does not carry) and naming (the memo's name beats the observation's) are 기본 지침 too, on by default and switchable ← owner 260927, same reason
- [x] grounding and naming staying in the system prompt as integrity ← owner 260927: they decide what is written, not the format it arrives in
- [x] keeping every rule in the system prompt and adding only the story rules to 지침 ← it breaks the one-question-per-layer split
- [o] a 기본 지침 is switched off for the whole account on the 지침 screen, where it stands above the owner's own guidelines with its (추천) mark and a switch; its text is the product's and cannot be edited — an owner who wants a variant switches it off and writes their own ← owner 260927: the same account-wide way the owner's own guidelines apply
- [x] switching a 기본 지침 off per post in the writing brief, or both ← more switches in the brief, and two places that could disagree
- [o] a template keeps only what each place is about: `<note>` and the video template's invisible guide are removed; a `<write>` block, an `<ask>` field and a composition stage keep one line saying what stands there (e.g. 메뉴 소개), while how to write it (tone, length, emphasis) belongs to 지침 ← owner 260927; the builder and the 형식 안내 teach it, the parser cannot enforce it
- [x] removing only `<note>` and the guide, leaving `<write>` instructions free ← the instructions would move into `<write>` and the overlap would come back
- [x] removing every instruction and keeping a bare place name ← the template could no longer say what a place is for
- [o] clips get their own 영상 지침 under the 영상 group, with their own 기본 지침 (e.g. captions continue a story rather than label a cut), the way 글 템플릿 and 영상 템플릿 are separate ← owner 260927
- [o] a clip project's instruction stays as this clip's own material, the place a post's memo holds ← the layers then line up: system = integrity, 지침 = direction, template = form, memo / instruction = this one piece
- [x] one 지침 screen whose rules each pick 글, 영상 or both ← one more scope choice on every rule
- [x] no 지침 for clips ← posts and clips would have different layers
- [o] the template's order is the frame and the storyline tells the story inside it, following the template's places (가게 정보 칸에서는 …, 메뉴 칸에서는 …); with no template the storyline sets the whole order ← owner 260927: the template is the form
- [x] the storyline setting the order with the template reduced to fixed text and fields ← the shape a template decides would change from post to post
- [o] each storyline paragraph shows the photos (a clip: the cut frames) it uses, and the write follows that placement ← owner 260927: a photo in the wrong part is caught before anything is written, the failure fixed on 260927 for posts
- [x] prose alone, photos chosen again at write time ← a misplaced photo would only show after writing
- [o] a clip caption states facts only, and an impression (taste, mood, satisfaction) only as the owner supplies it; this is a video 기본 지침, not a system rule ← owner 260927
- [o] the server's hard drop of experiential captions leaves; the 기본 지침 alone carries the rule, so switching it off lets impressions through ← owner 260927: a switch that changes nothing would mislead
- [x] keeping the server drop as a system rule that cannot be switched off ← it also drops impressions the owner supplied, whenever a word matches
- [o] an impression is the owner's when it comes from the memo, a clip project's instruction or what the owner wrote into the storyline; the storyline call follows the same 기본 지침, so an AI-drafted storyline carries none of its own ← owner 260927
- [o] posts take the same rule: facts only, impressions only as the owner supplies them, as a 기본 지침 ← owner 260927: one rule for posts and clips, and no impression the owner never had; a short memo may read drier
- [x] posts keeping impressions free as today ← posts and clips would follow different rules
- [o] the post 기본 지침 story rules: tell it in the order it happened (inside each template place when there is a template); open with why the owner went or what they expected when the memo says so; a photo is a moment in the story, standing between the sentences about that moment, with no paragraph opening as a photo description and each paragraph picking up from the last; close by drawing the day together, a verdict or a will to return only when the owner supplied one ← owner 260927 chose all four
- [x] asking the AI to revise the storyline through ②'s one revision composer with the storyline as one more target ← owner 260927 moved the storyline's AI request into the storyline's own space; ②'s composer keeps refining the post (a clip: 흐름 · 내레이션) alone
- [o] rewriting from a changed storyline (이 스토리로 다시 쓰기) replaces the whole post, or a clip's flow and captions, and asks for confirmation only when the owner has edited the result by hand ← owner 260927: there is no undo, and a result nobody touched loses nothing
- [x] a confirmation on every rewrite, or none at all ← the first nags when nothing is lost, the second loses hand edits silently
- [o] the owner moves the storyline's photos (a clip: cut frames) directly: drag one to another paragraph, or a move control on a phone, and take one out or put one in, with no AI call ← owner 260927
- [x] changing the photo placement only through the storyline's AI request ← a call and credits for moving one photo
- [o] the video 기본 지침 story rules: the first caption says in one line what the clip is about (구리에서 찾은 숨은 오리집); each caption carries on from the one before it rather than labelling what is on screen, and may run across cuts; cuts and captions follow the order it happened (inside each template stage when there is a template); the last captions draw together what the owner supplied (name, place, price) ← owner 260927 chose all four
- [o] ①'s two actions (스토리라인 먼저 · 바로 글 쓰기; a clip the same) each observe whatever is not yet observed and then go on, observations stored and reused as today ← owner 260927: as many presses as today
- [x] observation as its own first action with the two paths offered after it ← one more press and two waits
- [o] existing templates lose their `<note>` blocks and video guides outright, with no compatibility path ← pre-alpha data is disposable (owner 260927 standing rule)
- [o] each storyline call (make, AI revision, regenerate) is an LLM job charged at its own cost by the standard formula; hand edits and photo moves are free ← owner 260927: no new price rule, the storyline path costing about one ChargeBase (2 credits) more
- [x] the first storyline counted inside the write's charge ← a storyline made and never written from would need its own rule
- [o] the storyline call takes the memo, the observations, the template with its answers, 지침 and memory, and no voice ← a storyline is a plan written as 「…를 보여줍니다」, not prose in the owner's voice
- [o] the write on the storyline path takes every material the direct write takes (memo, observations, template and answers, 지침, memory, voice) plus the storyline ← owner 260927
- [o] on that path the storyline decides what the post covers and in what order, and the material only fills in the detail of what it covers (names, prices, what a photo shows); a part the owner deleted from the storyline stays out even when the memo still tells it ← owner 260927
- [x] the material adding content the storyline left out wherever it fits ← what the owner deleted would come back
- [o] the clip server's number check (a caption's number, price or unit must match an entered fact exactly) leaves as the experiential drop does; the grounding 기본 지침 carries it ← owner 260927: the system keeps format only
- [x] the number check following the grounding 기본 지침's switch, or staying as a system rule ← owner 260927 chose the guideline alone
- [o] by the same rule, the narration's omission of a caption that cites no observation, states no fact and answers no instruction (CLIP-137) leaves the server too ← it is a rule about what may be written, and it is one of the two causes of the shot list
- [x] the write on the storyline path taking only the template, the storyline and the voice, the rest melted into the storyline to save context ← sentence-level 지침 (title and tag rules, naturalness, vocabulary substitutions, the impression rule) act while sentences are written, IMAGE alt and caption need what each photo shows, and facts would reach the post only as far as the storyline spelled them out
- [o] a photo (a clip: a source) added after the storyline was made: the space says so and offers 다시 만들기, and writing from the storyline as it stands leaves the new one out ← owner 260927
- [x] new photos gathered as unplaced for the owner to drag in, or placed by the writer wherever they fit ← not chosen by the owner
- [o] A/B 비교 opens from 바로 글 쓰기's menu, so ①'s dock keeps two actions; each candidate sets its own storyline and the adopted one stays with the post; no A/B runs from a storyline ← owner 260927
- [x] A/B beside 이 스토리로 글 쓰기, or a third dock button ← more controls in the space, or three buttons across 360 px
- [o] 영상 지침 has a candidate queue like 지침, fed by ②'s clip revision requests; a storyline request feeds no candidate queue ← owner 260927: the same screen for posts and clips; a storyline request edits one draft plan

## shape
- layers: system prompt = format only (answer shape, attached files only, block rules) · 기본 지침 = the product's writing direction including grounding and naming, (추천), on by default, switchable per account · 지침 / 영상 지침 = the owner's writing direction · template = form (order, fixed text, photo places, fields, what each place is about) · memo / clip instruction / owner-written storyline = this piece's material, the only source of impressions
- flow: ① sources in → (스토리라인 먼저 | 바로 글 쓰기) → observe what is not yet observed →
  - 스토리라인 먼저: storyline call → ② opens with the storyline space open and no draft → owner edits text, moves photos, asks the AI in the space, or regenerates → 이 스토리로 글 쓰기 → write along it
  - 바로 글 쓰기: one write call sets the storyline, then writes along it (a clip: its flow call sets it and the narration call follows it)
  → ② the result, the storyline kept in its collapsible space → (refine the result with ②'s composer | change the storyline → 이 스토리로 다시 쓰기, confirmed only over hand edits) → ③
- storyline: prose paragraphs in order, each with the photos (cut frames) it uses; inside a template it follows the template's places; the write places photos as the storyline does
- v1: both paths for posts and clips; 기본 지침 for posts and clips with account switches; 영상 지침; template instruction cut to "what this place is about"; the clip server's content checks removed / not: writing a storyline from scratch, per-post 기본 지침 switches, keeping the storyline in step with later refinements

## domains
- GEN: the storyline call and its revise/regenerate; the write following a given storyline and its photo placement; the direct write storing the storyline its one call sets (GEN-67's `flow` becomes it); the system prompt cut to format (GEN-14 order); grounding (GEN-16, GUIDE-16), naming (GEN-44), altitude (GEN-47), title prohibitions (GEN-49), tag rule (GEN-50) and the naturalness baseline leave for 기본 지침; the write on the storyline path takes all material plus the storyline →GEN
- GUIDE: 지침 redefined from "what a post must avoid" to "what writing is wanted" (GUIDE-1, TMPL-1); product-owned 기본 지침 marked (추천), on by default, switched per account on the 지침 screen, text not editable (GUIDE-16, GUIDE-19's "no seeded guidelines, no toggles" reversed); the post rule set (grounding, naming, altitude, title, tag, naturalness, facts-only impressions, the four story rules); 영상 지침 with its own 기본 지침 (facts-only impressions, the four caption rules) under the 영상 group →GUIDE
- POST: ①'s two actions; ② opening on a storyline with no draft; the collapsible storyline space with its own AI request, photo moves, and 이 스토리로 다시 쓰기 confirmed over hand edits →POST
- CLIP: the same two actions and storyline space for clips; the flow call setting the storyline on the direct path and the narration following it; the project instruction as this clip's material; narration admission with no server content checks left — the experiential drop, the number match and the cite-an-observation omission all leave for 기본 지침 (CLIP-63, CLIP-64, CLIP-72, CLIP-122, CLIP-137); 영상 지침 in the nav (CLIP-3) →CLIP →CDS
- TMPL: form only: `<note>` and 'AI에게만 하는 말' removed (TMPL-18, TMPL-36), a `<write>` or `<ask>` carries what the place is about, the builder copy and 형식 안내 teach it (TMPL-41); existing notes dropped →TMPL
- CLIP (video template): the invisible guide removed (CLIP-4, CLIP-59, CLIP-112), a composition stage's line says what the stage is about (CLIP-141) →CLIP
- QUOTA: the storyline call and its AI revision are LLM jobs admitted and charged like any other (QUOTA-13) →QUOTA

## open
- settled in update-ssot 260927: a photo (a clip: a source) added after the storyline was made makes the space say so and offer 다시 만들기, and writing from the storyline as it stands leaves it out; A/B 비교 moves into 바로 글 쓰기's menu, each candidate setting its own storyline, the adopted one kept, and no A/B runs from a storyline; 영상 지침 gets a candidate queue like 지침, fed by ②'s clip revision requests (a storyline request feeds none)
