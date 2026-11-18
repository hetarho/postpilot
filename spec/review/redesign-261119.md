# REVIEW redesign-261119
> st:converted@261119 | scope:frontend/src/pages frontend/src/widgets frontend/src/features | at:83fdb76 | base:ARCH@29

## summary
Two read-only scouts compared the screens outside the ready-home renewal (merge 14acf5d3e) with its language: the kit/section header with a picture, `ListEmptyState`, dashed placeholders, one target per item and choices opening from the button that needs them. Eight proposals change a screen's flow and were adopted; four were held as cleanup or separate planning.

## findings
- F1 [o] P2 frontend/src/pages/posts/ui/PostsPage.tsx:296-352 frontend/src/pages/clips/ui/ClipsPage.tsx:214-318: 글 and 영상 items are tall cards with two or three links to the same item ← one gesture per intent (THEME-80) and about half the items per phone screen →T903
- F2 [o] P2 frontend/src/widgets/writing-test/ui/WritingTestHistory.tsx:120-472: 지난 비교 has three refresh buttons, a small 이어보기 target per row, a separate paid section and a full-reload 새 테스트 ← resuming a comparison needs a hunt →T904
- F3 [o] P2 frontend/src/pages/ai-models/ui/AIModelsPage.tsx:35-76: AI 토너먼트 opens on two help cards and a help line, the owner's own comparisons one page away ← resume takes three taps, nothing says one waits →T905
- F4 [o] P3 frontend/src/features/writing-test/model/test-flow-machine.ts:152: a comparison started for a named factor opens on the factor choice it already made ← a dead step →T906
- F5 [o] P2 frontend/src/pages/video-templates/ui/VideoTemplatesPage.tsx:107-252 frontend/src/pages/video-template/ui/VideoTemplatePage.tsx:193-341: 영상 템플릿 kept the pre-renewal directory, per-card delete and a reading that shows THEME-76's forbidden boilerplate ← two template directories built two ways →T907
- F6 [o] P2 frontend/src/pages/settings/ui/SettingsPage.tsx:74-98 frontend/src/pages/billing/ui/BillingPage.tsx:79-123: two copies of the credits card, and 충전 leaves the screen and scrolls to packs ← UI reuse rule; buying more takes two screens →T908
- F7 [o] P3 frontend/src/pages/account/ui/AccountPage.tsx:13-86: 계정 is two always-open forms under intro sentences ← the screen reads as forms, not state →T909
- F8 [o] P3 frontend/src/features/offer-creation-setup/ui/CreationOfferSheet.tsx:48-89: the missing-piece offer and 홈's next-step card do one job with two interfaces ← UI reuse rule →T910
- F9 [?] P3 frontend/src/pages/ai-models/ui/ModelComparisonPage.tsx frontend/src/pages/library: unrouted legacy pages (redirect-only routes) still ship with their i18n ← dead code, not a redesign
- F10 [?] P3 frontend/src/widgets/creation-setup/config/i18n.ts: /setup's ready step still says 만들러 가기 / 첫 클립 만들기 from the retired 만들기 tab ← touches the setup machine and POST-121; needs its own planning
- F11 [?] P3 frontend/src/pages/login frontend/src/pages/signup: each signed-out page hand-writes its frame with drifting padding ← one shared frame would satisfy the reuse rule; the owner kept login out of the last redesign
- F12 [?] P3 frontend/src/widgets/work-list/ui/WorkListFrame.tsx:24-26: a static 최신순 label beside 찾기 names an order that cannot be changed

## notes
- Self-improve run 261119 (tag redesign), invoked by the owner to renew the remaining screens the way 홈 was renewed, with Codex drawing new illustrations, within 5 hours, landing on main. Budget: 5 h wall clock; implementation by Claude, review by Codex (AGENTS.md provider preference).
- Illustrations drawn by Codex `$imagegen` in the existing set's style block (soft matte clay, violet #7B4FC4, lavender #E9DFFB, off-white, one peach #FFB38A accent, transparent), converted to 480 px WebP through Chrome's canvas encoder: posts, tournament, credits, video-template, video-guideline.
- F9–F12 are held: cleanup or separate planning, outside a flow redesign.
