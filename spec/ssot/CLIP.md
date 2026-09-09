# CLIP generated video projects and templates
> r1 | An account-owned workflow turns multiple pieces of experience footage into a downloadable, Naver Clip-ready video through a reusable video template, AI-selected cuts, exact styled copy and a lightweight correction pass, independently of blog posts.

## decisions
- CLIP-1 [o] a clip project is independent of a post and owns its title, chosen video template, template answers, target duration, aspect ratio, analysis, edit plan and latest successful result
- CLIP-2 [o] only the authenticated owner may list, view, change, generate, download or delete a clip project or video template; an unknown or foreign id is presented as not found
- CLIP-3 [o] navigation groups 글 · 말투 · 글 템플릿 · 지침 under 글 and 클립 · 영상 템플릿 under 영상; account, plan, billing and administration remain common destinations
- CLIP-4 [o] a video template is a reusable account-owned recipe that names the information to collect, the cut-composition guidance and the approved copy style choices
- CLIP-5 [o] every generation selects exactly one video template and collects that template's requested information before it can start
- CLIP-6 [o] a project accepts up to 20 source videos whose combined duration is at most 30 minutes
- CLIP-7 [o] the owner chooses a target result duration from 15 to 90 seconds before generation
- CLIP-8 [o] the owner chooses one project ratio at creation: 세로 9:16, 가로 16:9 or 정방형 1:1
- CLIP-9 [o] a project's ratio is fixed; producing another ratio starts another project ← reframing after cut and copy placement would make the approved composition unreliable
- CLIP-10 [o] generation analyzes each source for usable time ranges, events, visual subjects, quality and audible speech
- CLIP-11 [o] generation uses the target duration, source analysis, template guidance and template answers to choose source ranges, order the cuts and write the on-screen copy automatically
- CLIP-12 [o] generated copy is rendered as exact typeset text through approved visual templates ← AI-drawn Korean lettering can be misspelled or visually inconsistent
- CLIP-13 [o] the copy renderer uses a product-bundled, fixed-version Pretendard Variable font and never depends on a device or host system font ← the same project must render identically across environments
- CLIP-14 [o] the initial copy styles are 깔끔하게 (lower dark translucent card), 기록처럼 (small cream label) and 강조형 (large outlined title), with an optional accent colour from an approved palette
- CLIP-15 [o] copy stays within platform-safe regions and uses only approved positions; generation prefers a position that does not cover the scene's principal person, food or place
- CLIP-16 [o] scene changes use a short restrained fade
- CLIP-17 [o] the correction screen lets the owner reorder or delete cuts, extend or shorten each cut within its source range, edit its copy, change its approved copy position or style and adjust its original-audio volume
- CLIP-18 [o] original audio is retained by default and volume is controlled per cut
- CLIP-19 [o] AI analysis and AI composition regeneration consume the account's existing credits; admission, settlement and exhaustion follow QUOTA
- CLIP-20 [o] manual corrections, rerendering the corrected plan, preview and download consume no additional credits
- CLIP-21 [o] source videos are transient processing inputs and are never retained as project assets; the project retains only its metadata, latest successful result, analysis and edit plan until deletion ← source footage dominates storage and already remains on the owner's device
- CLIP-22 [o] the upload surface states before selection that source video is sent to an external video-analysis provider and is discarded after the active processing attempt
- CLIP-23 [o] after an attempt ends, any later generation or correction that needs source pixels requires the owner to select the source videos again; closing or refreshing the working page may require the same reselection
- CLIP-24 [o] deleting a project deletes its retained result, analysis, edit plan and metadata
- CLIP-25 [o] deleting a video template detaches it from projects but preserves their answers, analysis, edit plan and result; another AI generation requires a new template
- CLIP-26 [o] a failed analysis or render keeps the project and its previous successful result unchanged, identifies the failed stage and offers retry; retry requests the source videos again when they are no longer present
- CLIP-27 [o] the result is previewable and downloadable in a form accepted by Naver Clip for the selected ratio; the product does not submit it to Naver
- CLIP-28 [x] direct Naver publishing, generated source footage, AI-drawn copy images, per-project fonts, arbitrary decorations, automatic background music, custom transitions and advanced motion templates — out of scope

## flow
- create: 영상 → 클립 → 새 클립 → choose video template → answer its information fields → choose ratio and target duration → disclose external analysis → select source videos → generate
- generate: analyze sources → choose and order ranges → write and place copy → render → preview
- correct: preview → change cuts · copy · placement · style · volume → reselect sources if absent → rerender → download
- failure: analysis or render fails → keep project and previous result → identify stage → reselect sources if absent → retry
- delete: delete project → remove retained result · analysis · edit plan · metadata

## constraints
- source limits: 20 videos per project · 30 minutes combined
- result limits: 15–90 seconds · exactly one of 9:16, 16:9 or 1:1 per project
- retained project data excludes every source-video byte
- delivery must satisfy the current Naver Clip upload contract for all three ratios

## chg
- r1 260909 initial
