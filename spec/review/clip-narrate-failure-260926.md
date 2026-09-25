# REVIEW clip-narrate-failure-260926
> st:converted@260926 | scope:backend/internal/clip/ai frontend/src/shared/lib/localization | at:4ef22607 | base:ARCH@11

## summary
- prod job b752f893 (260925 15:30Z, project 카페숲, 30 s vertical, styles bold · word-pop · neon · iridescent) failed at narrate with CLIP_COMPOSITION_INVALID `{element_id: narration-2, line: 0, reason: copy_limit}`; its flow also carried a 1 ms tail cut
- both break CLIP-106 on the server side (deliver the readable plan, remove only what cannot be satisfied), and the owner-facing message pointed at a template line that does not exist

## findings
- F1 [o] P1 `backend/internal/clip/ai/narration_parse.go:239` admitNarration + `narration_prompt.go:18`: bug: a narration caption is admitted and described to the writer against the DEFAULT style's bound (2 × 11) since T256 let the writer name a style per caption, while 9 of the 16 approved styles hold 9 characters per line; `media/composition_layout.go` then refuses the named style's line and `clip.AutomaticCompositionRepair` (Style auto and Basis cut only) never lets a narration caption reach its short_text, so one caption fails the whole paid attempt ← CLIP-106 and CLIP-118 say shortened when a grounded shorter text fits, omitted with a CLIP-108 notice when none does; prod case `서까래 아래 원목 좌석이\n차분하게 놓였어요` (line 1 = 10) under `iridescent` (2 × 9) with short_text `원목 테이블 배치` (7) unused →T398
- F2 [o] P3 `frontend/src/shared/lib/localization/failure.ts` formatAppFailure: bug: CLIP_COMPOSITION_INVALID always renders `영상 구성의 {{line}}번째 줄({{element_id}})`, but a narration caption has no outline line (Span.Line 0), so the owner reads `0번째 줄(narration-2)` ← the owner cannot find what to fix and a template line 0 does not exist →T398
- F3 [o] P2 `backend/internal/clip/ai/plan_ladder.go:191` trimGeneratedOverrun: bug: the tail trim that brings an overrun flow to its target leaves a cut as short as 1 ms (floor `max(1, transition+1)`), below one 30 fps frame and CDS's `timing.cut_min_s` 1.2 ← a sliver cut is kept in the plan, shown in ② and counted, yet plays as nothing; prod case cut 9 (source 931aafb6) 1000–1001 ms, cut 8 trimmed after it →T399

## notes
- adopted at once: the owner asked for these fixes directly (260926)
- styles holding 9 per line: keynote blur-in neon iridescent glitch ember outline pop serif (`design.json` `regions.caption.*.chars`)
- rapid pace keeps its own phrase bound (`Rapid.MaxChars`, one line per phrase); a rapid phrase in a hook-role style was not measured here
- `validatePlan` refuses a plan more than `TargetToleranceMS` over its target, so F3's floor cannot simply leave an overrun the floor would not absorb
