# CLIP generated video projects and templates
> r3 | An account-owned workflow turns multiple pieces of experience footage into a downloadable, Naver Clip-ready video through bounded analysis copies, an approved credit ceiling, AI-selected cuts, exact styled copy and a lightweight correction pass, independently of blog posts.

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
- CLIP-19 [o] AI analysis and AI composition regeneration consume the account's existing credits within the maximum shown and approved before generation under QUOTA-45; a durable preparation stage verifies sources and prepares bounded analysis copies before reserving the complete AI run under QUOTA-43; no AI call may precede a successful reservation
- CLIP-20 [o] manual corrections, rerendering the corrected plan, preview and download consume no additional credits
- CLIP-21 [o] source videos and their analysis copies are transient processing inputs and are never retained as project assets; the project retains only its metadata, latest successful result, analysis and edit plan until deletion ← source footage dominates storage and already remains on the owner's device
- CLIP-22 [o] the upload surface states before selection that compressed copies containing the selected footage and its audible speech are sent through OpenRouter to an external video-analysis provider, and that the service discards originals and analysis copies after the active attempt; local cleanup is not a promise about an external provider's retention
- CLIP-23 [o] after an attempt ends, any later generation or correction that needs source pixels requires the owner to select the source videos again; closing or refreshing the working page may require the same reselection
- CLIP-24 [o] deleting a project deletes its retained result, analysis, edit plan and metadata
- CLIP-25 [o] deleting a video template detaches it from projects but preserves their answers, analysis, edit plan and result; another AI generation requires a new template
- CLIP-26 [o] a failed attempt keeps the project and its previous successful result unchanged, identifies the failed stage and credit settlement outcome, and offers explicit retry; an AI retry requires a newly approved ceiling, a manual rerender remains credit-free under CLIP-20, and retry requests absent source videos again without starting automatically
- CLIP-27 [o] the result is previewable and downloadable in a form accepted by Naver Clip for the selected ratio; the product does not submit it to Naver
- CLIP-28 [x] direct Naver publishing, generated source footage, AI-drawn copy images, per-project fonts, arbitrary decorations, automatic background music, custom transitions and advanced motion templates — out of scope
- CLIP-29 [o] analysis sends only bounded compressed copies directly through the existing OpenRouter connection as inline video data, not a signed object URL or the original file; each copy covers at most 60 seconds, has a long edge at most 720 pixels and occupies at most 8 MiB before encoding ← an input modality flag alone does not establish signed-video-URL compatibility
- CLIP-30 [o] video analysis uses a fixed sampling mode compatible with the selected model and explicitly selects `static` where supported; adaptive video exploration is excluded, and an incompatible input or processing mode is refused before a model call
- CLIP-31 [o] the composition model receives only bounded structured source analysis, template guidance and answers, never video bytes or video URLs; final rendering uses the original footage and audio, not the compressed analysis copies
- CLIP-32 [o] preparation verifies every source and every analysis copy before the first AI call; bounded recompression may occur only during preparation, and a copy still exceeding a limit fails without AI rather than silently increasing the chunk count, omitting footage or changing its timing
- CLIP-33 [o] clip media work runs one job at a time on the shared service and processes originals sequentially through bounded temporary disk storage; source verification and analysis-copy preparation share the same source read, analysis copies are not uploaded back to object storage, and transmission never accumulates whole original files or all encoded copies in memory
- CLIP-34 [o] a selected original's local preview stays available on the working page until the attempt terminates, with filenames and processing state visible throughout; acceptance by the worker does not remove it, while termination releases local media references and leaves a filename/status summary; closing, refreshing or leaving the page releases local media without persisting it or restarting the server job
- CLIP-35 [x] reusing source analysis for a separate composition-only AI regeneration action — deferred; manual correction and credit-free rerendering under CLIP-20 remain available

## flow
- create: 영상 → 클립 → 새 클립 → choose video template → answer its information fields → choose ratio and target duration → disclose external analysis → select source videos → view maximum credit charge → approve and generate
- generate: approve ceiling → create job → verify sources and prepare all bounded analysis copies → validate required reservation against approved ceiling → reserve credits (refused: fail without AI or debit, preserve previous result, clean sources) → sequential inline analysis → text-only composition and copy → render originals → result preview and settlement → source/local-preview cleanup
- correct: preview → change cuts · copy · placement · style · volume → reselect sources if absent → rerender → download
- failure: preparation, ceiling validation, analysis or render fails → keep project and previous result → identify stage and settle AI work under QUOTA-46 → clean transient media → reselect sources if absent → AI retry(view and approve new ceiling) | manual rerender retry(no credits)
- delete: delete project → remove retained result · analysis · edit plan · metadata

## constraints
- source limits: 20 videos per project · 30 minutes combined
- result limits: 15–90 seconds · exactly one of 9:16, 16:9 or 1:1 per project
- retained project data excludes every source-video byte
- analysis limits apply before provider transmission: at most 60 seconds · long edge at most 720 pixels · at most 8 MiB per compressed copy; the encoded request and temporary workspace have separate finite bounds, and compression never authorizes extra paid calls
- analysis copies preserve source timing, visible events and audible speech; codec, frame-rate and bitrate choices require media-quality and resource verification under ARCH-24 and provider-contract verification under ARCH-33, not a change to original-footage rendering
- original uploads remain browser-to-private-storage transfers; inline analysis is a worker-to-provider exception and does not enlarge public RPC bodies or make source objects public
- temporary originals, analysis copies and encoded request bodies are cleaned on success, failure and crash recovery; their bytes and signed links never enter logs or durable job payloads, and local preview references never enter persistent browser storage
- credit authority is QUOTA-43 through QUOTA-47; a provider unit-price limit and a per-attempt user-credit ceiling are distinct protections
- delivery must satisfy the current Naver Clip upload contract for all three ratios

## chg
- r3 260910 CLIP-19✎ duration-only preparation and hold→bounded-copy preparation and approved ceiling; CLIP-21✎ transient originals→transient originals and analysis copies; CLIP-22✎ original-video disclosure→compressed-copy disclosure; CLIP-26✎ stage-only retry→stage and settlement with explicitly reapproved retry; CLIP-29+ CLIP-30+ CLIP-31+ CLIP-32+ CLIP-33+ CLIP-34+ CLIP-35+ bounded inline/static analysis, original-quality rendering, serialized preparation, attempt-long local previews and deferred analysis reuse
- r2 260910 CLIP-19✎ generic QUOTA admission→durable preparation then full-run credit reservation before any AI call
- r1 260909 initial
