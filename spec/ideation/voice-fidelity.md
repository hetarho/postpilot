# IDEATION voice-fidelity
> st:open@261116 | An owner can tell whether a post is in their own voice, and the writer follows that voice closely enough to call it theirs.

## vision
- problem: the post's fingerprint reading showed raw value pairs whose headline was the largest relative gap (0% vs 2%), counted template headings as prose and treated once-seen first lines as habits, so an owner could not tell whether the voice held; the projection hands the writer percentages a model cannot follow, and the strongest signal, three excerpts, is invisible to the owner
- target: an owner who learned a voice from a chat or pasted posts and reviews a generated post in ④
- core value: whether the voice held is visible in the post itself, and only claims the product can show are made

## explored
- [o] fix the counting: prose-only post fingerprint, recurring-only opening/closing lines, verdict per item with a dominant-value lead and an n-of-m summary ← most reported gaps came from counting and display, not from the writer
- [o] 출처 보기 gains a 말투 view that underlines habit phrases found in code, with habit, rates and a 학습 글 example ← shows where the voice holds without an AI call or a change to the four meaning origins
- [o] the origin legend names 출처 미확인; unconfirmed style-only sentences stay unconfirmed rather than gaining a new origin state ← GEN-80 origins are about meaning, and the 말투 view covers style
- [o] observation fields serve as visual evidence when the observation holds no origin spans; the memo's name wins in every media caption and is owner-input there ← photo captions were all unconfirmed, and 치즈버거 must not read as the observer's 햄버거
- [?] grounded style guide: the analysis call writes 6–8 rules with quoted evidence, the counted fingerprint verifies each rule and drops one it contradicts, and the projection carries the rules plus representative excerpts instead of percentage lines; the comparison reads per rule (kept / not kept)
- [?] AI voice scoring with a matching revision: an explicit owner action compares the post with the 학습 글, shows a score and drifting sentences, and offers 말투에 맞게 다듬기 — needs VOICE-62's no-AI-judge rule and VOICE-16 changed, costs a call and varies run to run
- [x] a full AI-only analysis replacing the counted fingerprint ← tends toward generic descriptions, varies per run and leaves nothing to verify the writer against

## shape
- flow: generate → ④ 출처 보기 → 말투 view (summary + underlined habits) → owner judges whether the voice held → if weak, reanalysis with a grounded style guide (later)
- v1: counting fixes, 말투 view, legend, photo evidence, caption naming / not: style-guide analysis, AI scoring, automatic voice correction

## domains
- [o] VOICE: counting fixes, comparison verdicts, habit phrase finding →VOICE
- [o] POST: 말투 view in 출처 보기, legend names 출처 미확인 →POST
- [o] GEN: caption naming across media, observation fields as visual evidence →GEN
- [o] THEME: legend and voice-habit role →THEME
- [?] VOICE: grounded style guide replacing percentage projection lines, with GEN's write projection
- [?] VOICE: explicit AI voice scoring and matching revision, with MODEL and QUOTA costs

## open
- measure with real posts after the 말투 view ships: which habits appear, which do not, and whether chat-only voices yield blog-length rhythm
- whether a grounded style guide should replace or sit beside the counted lines in the projection
- whether excerpts should prefer pasted posts over short chat answers when both exist
- whether the learning chat's examples (→VOICE-79) pull replies toward the examples' registers: compare the ending mix of chat replies with pasted posts of the same owners once the examples ship
