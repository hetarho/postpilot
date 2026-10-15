# IDEATION campaign-reviewer-focus
> st:converted@261015 | Narrow the product to 체험단 creators: fast monetization and 상생 through posts grounded only in the owner's own experience, with always-on rules, a guided template wizard, a friendlier real-speech voice and per-channel outputs and menus.

## vision
- Problem: AI blog-automation services multiply and fill blogs with hallucinated AI slop; the product must not join that, yet its current shape is a general, highly extensible writing assistant without a stated direction.
- Target: 체험단 creators who earn by steadily publishing good posts about sponsored visits and products, rather than creators betting on a YouTube or blog breakout. Three channel groups: Naver Blog, Naver Clip, Instagram.
- Core value: automate the 체험단 work. The product writes only from what the owner actually experienced (memo, photos, answers), so it is a tool that helps write high-quality posts, not a content generator.
- Philosophy: fast monetization (a 체험단 post earns on delivery, not on a breakout) and 상생 (a good post also helps the store; the product does not profit from posts nobody reads).

## explored
- [o] target users are 체험단 creators on Naver Blog, Naver Clip and Instagram ← owner direction
- [o] the existing post generation flow and its quality stay as they are once voice and template exist ← owner direction; quality is acceptable
- [o] the menu and the outputs split by channel from the start: Naver Blog / Naver Clip / Instagram Clip / Instagram Post (images, TBD) ← owner direction
- [o] some stock guidelines tied directly to the philosophy become always-on (not switchable); the remaining preset guidelines are re-decided in this ideation ← owner direction
- [o] template creation becomes a guided multi-step wizard (Toss-like): ask what the template is for, explain the usual composition, check/reorder/add custom sections ← owner direction
- [o] voice creation becomes friendlier and collects real-life speech ← owner direction
- [o] "real-life speech" means an in-product chat: the owner chats casually with the product in a chat UI, the conversation exists only to learn the owner's tone (its content is meaningless and never a post's material), and the owner's chat messages become personal learning material ← owner direction; chatting is easier than pasting posts or answering a questionnaire
  - [x] importing posts from the owner's Naver blog URL ← over-reading the request
  - [x] recording speech for transcription ← same
  - [x] a library of real bloggers' voice samples ← same
  - [x] learning from messenger exports ← same
  - [o] the chat replaces the ten-question questionnaire: the chat partner asks the situational questions casually and the owner's replies are the learning material; voice setup becomes 대화하기 / 내 글 붙여넣기 / AI 말투 ← owner choice; the questionnaire was already a stiff conversation
    - [x] a fourth peer method ← four choices are harder to pick from
    - [x] chat as the only method ← paste and AI styles still serve their users
  - [o] the chat steers replies toward how the owner writes a post ("이웃에게 소개하듯 말해 주세요"), keeping one voice per owner ← owner choice; raw chat tone ("ㅋㅋ 맛있었음") would make blog posts too light
    - [x] learn raw chat tone as is ← too light for a blog post
    - [x] one voice with a blog face and a chat face per channel ← two faces for one voice
- [o] the eight AI-generated styles stay as a starting point for a creator who has never written, labelled AI 말투, and the product nudges toward chat learning so the voice turns personal over time ← owner choice; a new 체험단 creator needs a start, while the philosophy favors the owner's own voice
- [o] the template wizard is a branching one-question-per-screen flow (progressive disclosure): each answer narrows the next question until the target is specific ← owner direction; a first-time user is guided, not handed a builder
  - v1 targets cover all four 체험단 groups, each split further: 맛집 and 카페 are separate targets; 제품 branches by kind (가전, 화장품, 식품, 생활용품 …) and by review form (언박싱, 실사용 후기 …); 뷰티·헤어·네일 and 숙소·여행·체험 branch the same way ← owner direction; one lumped target produces a composition that fits neither
  - [?] the exact branch tree (levels, labels, which branches v1 ships)
- [o] a target's recommended composition is a product-curated list of section roles (지도, 메뉴판, 가격 …); the owner names each section's heading in their own words (지도 → '오시는 길', '어디로 가면 될까요?'), so templates made on this platform differ per owner and posts made from them do too ← owner direction; identical headings across users would make every post on the platform look the same
  - the wizard reverses TMPL's "no shipped template library" for roles only: the product ships role lists, never finished templates or headings
  - [o] the heading step offers AI-suggested headings written in the owner's voice, editable, beside free input ← owner choice; the most personal option, at the cost of a credit and a wait
    - [x] a shuffled pool of stock examples ← shared examples pull headings toward sameness
    - [x] an empty field with a hint only ← owner chose suggestions
- [o] after choosing and ordering sections, the wizard asks about one section per step: its heading, how many photos stand together, and whether it is a field the owner fills in when writing (가격, 별점) or a fixed line ← owner choice; friendliest, at the cost of more screens
  - [x] product defaults with optional expansion ← owner chose one step per section
  - [x] no per-section settings ← same
- [o] the wizard covers both blog templates and clip templates; Naver Clip and Instagram Clip share one clip composition ← owner choice
- [o] 새 템플릿 is the wizard alone; a saved template is edited in the existing editor (blocks, AI editing); 원문 and the outside-AI 형식 안내 move behind an advanced entry; "make a template from this post" becomes one of the wizard's starting points ← owner choice; one creation path for a first-time user, the precise editor kept for later changes
- [o] "체험단 automation" is positioning over the current product, which already does the work; the one functional addition is an advertiser-guide (광고주 가이드) field per post whose requirements the prompt weaves in naturally ← owner direction: the product is the same, the framing is marketing
  - [x] campaign schedule/status management ← beyond what the owner wants now
  - [x] campaign discovery and application on 체험단 sites ← beyond what the owner wants now
  - [o] meaning that comes from the advertiser guide is a fourth semantic origin, 광고주 요청, with its own color in ②'s origin review beside 직접 입력 기반 · 사진에서 추론 · AI가 보탠 내용 ← owner direction; an advertiser's requested claim is not the owner's experience, and the owner should see which sentences carry it
    - like every origin highlight it is review-only and never enters copy or export output (→EXPORT-27)
    - text the owner typed into the guide keeps this origin even though the owner typed it, because origin is the source of meaning, not who typed it (→GEN-80)
  - [o] a pre-submission checklist reads the guide against the finished post: what can be counted (required keywords present, character count, photo count) is marked automatically, everything else is a box the owner ticks; it makes no AI call ← owner choice; a missed guide item means a revision request and a later payout
    - [x] prompt only, no check ← misses are found by the advertiser instead
    - [x] check plus one-click AI fix of a missed item ← keyword-chasing makes sentences awkward
- [o] each channel menu holds that channel's creation, history, templates and guidelines (the two clip channels share clip templates and 영상 지침); voice, memories, narration voices, account, billing and AI models sit in one channel-independent 내 설정 ← owner choice; a template or guideline is channel-shaped, while a voice is the owner's own on every channel
  - [x] channels hold creation and history only, settings stay in one place ← owner chose channel-owned templates and guidelines
  - [x] voices per channel too ← the same voice would be made again per channel
  - derived: the separate 기록 destination dissolves into each channel's history
- [o] menus split by channel, and a finished item offers "make a clip/reel from this experience", carrying its photos, memo and answers into a new item of the other channel; each channel's result is managed independently ← re-uploading the same visit per channel contradicts fast monetization, while the owner wants channel menus
  - [x] one experience/campaign entity holding every channel's output ← the menu's center would become the experience, not the channel
  - [x] fully independent channels ← the same photos uploaded again per channel
- [o] 상생 with a disappointing experience: keep the owner's stated drawbacks and write them politely as information for readers ("이런 분께는 이 점을 참고"), never as blame; never invent praise the owner did not give ← candidate always-on rule; all-praise posts lose reader trust while raw complaints hurt the store and re-selection
  - [x] drawbacks only on explicit request ← hides what the owner actually experienced
  - [x] verbatim intensity without softening ← hurts the store relationship
- [o] four groups of stock rules become always-on Postpilot principles, shown read-only at the top of the guideline screen with no off switch; the same principles apply to clips through their 영상 지침 counterparts (입력한 사실만, 감상은 내가 쓴 것만) ← owner choice; these are the philosophy itself
  - factual accuracy: 재료에 있는 사실만 · 메모의 이름으로 · photo order never stated as visit chronology (the factual half of 알려준 사건 순서대로)
  - impressions and constructive honesty: the owner's impressions stay central and unchanged, no invented praise, verdict or will to return, drawbacks kept politely (the 상생 rule above)
  - no AI tells: 자연스러운 한국어 문체 · 같은 종결어미 세 번 잇지 않기 · 관찰을 나열하지 않기; a voice still outranks them where they conflict
  - search exposure: 제목 규칙 · 태그 규칙 (a store found in search is 상생); a template's title form still outranks the title rule
- [o] inferred flavor/texture (내 감상은 지키고 맛은 재료로 살리기, its inference half) and 기억을 통한 감상 추가 stay switchable stock rules, on by default ← owner choice; they keep posts from going dry while a stricter owner can turn them off
- [o] sponsorship disclosure (대가 표시) is the owner's job, because every 체험단 platform has its own instructions; the output only reserves its place with a placeholder at the very top of the post, like a photo marker ← owner direction
  - [x] the product always writes a disclosure sentence for a 체험단 post ← platform rules differ
  - [x] disclosure only when the advertiser guide asks for it ← same reason
  - [o] each post carries a `체험단 글` switch (on by default); while it is on, the placeholder stands at the very top of the output ← owner choice; precise without forcing a 내돈내산 post to delete it
    - [x] only posts with an advertiser guide ← a 체험단 post written without a guide would lose it
    - [x] every post ← a 내돈내산 post would have to delete it

- [o] a clip channel's result is the vertical video plus that channel's upload text — a Naver Clip description and tags, an Instagram caption and hashtags — written from the same material under the same always-on principles, each with a copy control ← owner choice; the upload text is the other half of what a channel needs
- [o] both clip channels are vertical 9:16, so the 16:9 and 1:1 ratio choice leaves the product ← derived from channel-fit outputs; owner acknowledged
- [o] outputs that belong to no target channel (Tistory HTML, standalone site HTML, Markdown) leave the product; the Naver Blog output keeps its rendered preview, photo/caption copies and tag field ← derived from "outputs fit the chosen channel from the start"
- [o] 작성 테스트 (single-factor knockout writing tests) moves under 내 설정 as an advanced feature; the top navigation is the four channels plus 내 설정 ← owner choice; most 체험단 creators never use it, while those who do keep it
  - [x] keep it in the top navigation ← dilutes the channel-centered structure
  - [x] remove it ← existing test history would be lost
- [o] home offers the channels as its entries in place of today's two creation choices ← derived from channel menus
- [?] Instagram Post (images) stays TBD: its menu entry, output (photo set, caption, hashtags) and v1 inclusion are undecided ← owner marked it TBD

## shape
- flow (blog): 네이버 블로그 → 새 글(체험단 글 switch, 광고주 가이드, template, voice, memo, photos) → generate(always-on principles + switchable defaults + guide woven in) → review/revise → 제출 전 체크리스트(counted marks + owner ticks) → copy(disclosure placeholder on top when 체험단 글, body with photo markers, photo/caption copies, tags) → 이 체험으로 클립 만들기(optional)
- flow (clip): 네이버 클립 | 인스타 클립 → 새 클립(9:16; fresh material | carried over from a post's photos, memo and answers) → existing clip workflow → video + that channel's upload text(copy)
- flow (template wizard): channel → 템플릿 → 새 템플릿 → target(branching one question per screen) → recommended section roles(check, add custom) → order(drag) → one step per section(heading: AI suggestions in the owner's voice | own words; photos together; owner-filled field | fixed line) → name + preview → save → later edits in the existing editor (원문 / 형식 안내 behind 고급)
- flow (voice): 내 설정 → 말투 → 대화하기(chat; partner asks situational questions steering toward blog-style replies; replies become learning material) | 내 글 붙여넣기 | AI 말투(starting point) → explicit analysis → voice → nudges to keep chatting so an AI 말투 turns personal
- navigation: 네이버 블로그 · 네이버 클립 · 인스타 클립 · 인스타 게시물(TBD) · 내 설정; each channel = 만들기 + 기록 + 템플릿 + 지침 (the clip channels share clip templates and 영상 지침); 내 설정 = 말투 · 기억 · 내레이션 목소리 · AI 모델 · 계정 · 결제 · 고급(작성 테스트)
- v1: the four channel menus (Instagram Post as decided later), always-on principles, switchable defaults, advertiser guide + pre-submission checklist, 체험단 글 switch with disclosure placeholder, material carry-over between channels, template wizard for blog and clip templates, chat voice learning, clip upload text, 9:16 only
- not: campaign schedule/status, campaign discovery/application, product-written disclosure, Tistory/site/Markdown exports, 16:9 and 1:1 clips, 작성 테스트 in the top navigation, changes to the post generation pipeline itself

## domains
- GUIDE →GUIDE: always-on principle groups (read-only, no switch) with their clip counterparts; the constructive-honesty rule; inferred flavor and memory impressions as switchable defaults; guidelines owned per channel (blog vs the shared clip set)
- GEN →GEN: prompt wording for constructive honesty, weaving the advertiser guide in naturally, the fourth semantic origin 광고주 요청 (GEN-80), the disclosure placeholder block when 체험단 글 is on, clip upload text generation per channel
- POST →POST: 체험단 글 switch, advertiser guide field, 광고주 요청 as its own colored origin in ② (POST-112), pre-submission checklist (counted marks + owner ticks), "make a clip from this experience" carry-over
- EXPORT →EXPORT: Naver Blog output only, disclosure placeholder at the top; Tistory, site and Markdown converters leave
- TMPL →TMPL: the wizard (branching target tree, curated section-role catalogs per target, one step per section, owner headings with AI suggestions in the owner's voice), wizard-only creation, advanced 원문/형식 안내; reverses "no shipped library" for roles only
- CLIP →CLIP: clip template wizard (shared Naver/Instagram composition), channel per clip, 9:16 only, upload text with copy, carry-over from a post
- VOICE →VOICE: chat learning replacing the ten-question questionnaire, blog-scene steering, AI styles as a nudged starting point
- THEME →THEME: navigation of four channels + 내 설정, per-channel history replacing 기록, 작성 테스트 under 내 설정 고급, home with channel entries, the always-on principle card, Toss-like wizard steps
- MKT →MKT: /about positioning — 체험단 automation, fast monetization and 상생, posts only from the owner's own experience as the opposite of AI slop

## open
- Instagram Post (images): menu entry, output and v1 inclusion (owner: TBD)
- the wizard's branch tree per target group, and each target's section-role catalog (blog and clip)
- the disclosure placeholder's wording/format beside the photo markers
- each clip channel's upload text shape (length, tag count, fixed lines)
- how AI heading suggestions behave for an owner with no voice yet
- whether an English output target (LANG) still fits a Korean 체험단 product
- [?] feasibility: chat learning must still cover the readiness parts (opening, description, closing) the analysis needs
