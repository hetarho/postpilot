# Design-system route audit — 2026-11-14

Scope: the complete product router, public/recovery routes and administration. THEME r44 (75–78), ARCH r24. This is the dsui user-request audit; unrelated loading-policy and caption-preview work retain their ownership.

## Changes and source review

| Surface family | Sources inspected / meaningful states | Result |
| --- | --- | --- |
| Returns and docks | shared navigation, WorkspaceHeader, StepBar; creation, failure, saved result, local previous-step | T803/T805–807: one round secondary arrow, localized destination name/title, original link/modified-click semantics and previous-step actions; no duplicate shell returns. Authoring duplicate glyph repaired. |
| Generic controls | shared SurfaceButton, MediaAction, RadioTile; pickers, previews, media, financial forms | T804/T810/T813–815/T821: shared focus, touch and selection behavior; no generic native button/text input/textarea remains outside shared/ui. |
| Saved/draft directories | post/video templates, spoken voices, guideline sessions, memories/writing voices | T808–809: DirectoryItem cards, metadata/status/count/availability, separate delete/sample targets and one creation dock; existing disclosure directories retained where already conforming. |
| Post/video template host and wizard | method choice, AI editor, direct editor, pending/conflict/failed publication, examples | T807/T810/T821: shared return, title, choice, card/well and dock recipes; preview text and all editing/publication behavior retained. |
| Full video lifecycle | kind/requirements/sources/template/content/style/note/review, observe/reference, storyline, refine/caption/frame, finish/working/failure | T811–813: question/title/card/progress/step-bar hierarchy matches writing, retained actor/media mounts; no new rendering/provider behavior. |
| Photo/post authoring | attachment carousel, copy targets, inline editor, candidate voices/comparisons | T806/T814/T824–825: shared actions/cards or semantic inner wells; comparison deliberately remains plain on phone and gains the card plane on desktop. |
| Admin/billing/plans | accounts, refund request/review, credit packs, vouchers, costs, model catalog, recommendation editing, paired models, plan conditions | T815/T822–825: shared form/selection/card recipes; chronological/data rows stay rows and retain financial values, filters, request validation and approval semantics. |
| Public/auth/account/recovery/settings/hubs | all manifest paths below; inputs, loading/error/recovery branches and contextual destinations | Already use shared typography/controls, with return exceptions migrated in T805. Promotional composition retains its existing purpose. Browser route/state evidence is recorded below. |

The source audit searched all product TSX for native generic controls, old raised panels, resting borders and return text, then inspected the matches and their parents. Remaining `bg-surface-raised` uses are content filename tokens (`BlockList`, `TemplateRequestBox`), the public header and About illustration steps/example output. They are deliberate content/header drawings, not generic card/control exceptions. Data-table/chronological separators, selected accent rings, actual preview graphics and promotional layouts retain their semantic purposes. No path-wide product control exemptions were added.

## Native element inventory and prevention

The AST-based lint rejects JSX button/input/textarea mounts outside shared/ui, ignores examples/comments/string literals, and exempts test fixtures. Shared native radio/checkbox implementations stay inside shared/ui. Every allowed product native input is a visually hidden file input with the visible action owned by a shared control:

- `features/upload-photos/ui/PhotoPicker.tsx`: photo and video file selectors.
- `features/upload-clip-sources/ui/ClipSourcePicker.tsx`: clip media file selector.
- `features/edit-voice-material/ui/EditVoiceMaterial.tsx`: source material file selector.
- `features/answer-voice-prompt/ui/VoiceAnswerComposer.tsx`: answer material file selector.

`pnpm lint:style:probe` passes 83 scanner cases, including multiline controls, comments/strings, visible file inputs, native radio/checkbox rejection, hidden=false and supported hidden file inputs. `pnpm lint:style` passes. Retired/unregistered surface-sunken and intent-accent aliases are also rejected.

## Router inventory

The browser fixture reads the real assembled router, including layout and redirect records; the latest discovery contains 68 registered records and 63 concrete page/callback/redirect paths. Layout-only records are covered by their rendered children. The manifest requires every discovered concrete path; a missing expectation, unavailable runtime, wrong leaf/path, overflow or wrong named state fails the run. Aliases assert their real canonical destinations.

| Concrete path | Expected destination | Primary source |
| --- | --- | --- |
| `/login` | `/login` | `pages/login/ui/LoginPage.tsx` |
| `/signup` | `/signup` | `pages/signup/ui/SignupPage.tsx` |
| `/login/google/callback` | `/login/google/callback` | `pages/google-sign-in-callback/ui/GoogleSignInCallbackPage.tsx` |
| `/verify-email` | `/verify-email` | `pages/verify-email/ui/VerifyEmailPage.tsx` |
| `/forgot-password` | `/forgot-password` | `pages/forgot-password/ui/ForgotPasswordPage.tsx` |
| `/reset-password` | `/reset-password` | `pages/reset-password/ui/ResetPasswordPage.tsx` |
| `/gift/gift` | `/gift/gift` | `pages/gift/ui/GiftPage.tsx` |
| `/about` | `/about` | `pages/about/ui/AboutPage.tsx` |
| `/` | `/create` | `pages/create/ui/CreatePage.tsx` |
| `/library` | `/my` | `app/routes/creation.ts` |
| `/settings` | `/settings` | `pages/settings/ui/SettingsPage.tsx` |
| `/setup` | `/setup` | `pages/setup/ui/SetupPage.tsx` |
| `/tests` | `/tests` | `pages/writing-tests/ui/WritingTestsPage.tsx` |
| `/tests/history` | `/tests/history` | `pages/writing-tests/ui/WritingTestsPage.tsx` |
| `/tests/records/record` | `/tests/records/record` | `pages/model-experiment/ui/WritingTestRecordPage.tsx` |
| `/tests/test` | `/tests/test` | `pages/writing-tests/ui/WritingTestsPage.tsx` |
| `/posts` | `/naver-blog` | `pages/posts/ui/PostsPage.tsx` |
| `/posts/new` | `/posts/new` | `pages/editor/ui/EditorPage.tsx` |
| `/posts/experiments/record` | `/tests/records/record` | `pages/model-experiment/ui/WritingTestRecordPage.tsx` |
| `/posts/review` | `/posts/review` | `pages/editor/ui/EditorPage.tsx` |
| `/voices` | `/voices` | `pages/voices/ui/VoicesPage.tsx` |
| `/templates` | `/templates` | `pages/templates/ui/TemplatesPage.tsx` |
| `/templates/new` | `/templates/new` | `pages/template/ui/TemplatePage.tsx` |
| `/templates/template` | `/templates/template` | `pages/template/ui/TemplatePage.tsx` |
| `/guidelines` | `/guidelines` | `pages/guidelines/ui/GuidelinesPage.tsx` |
| `/memories` | `/memories` | `pages/memories/ui/MemoriesPage.tsx` |
| `/voices/voice-default` | `/voices/voice-default` | `pages/voice/ui/VoiceAnalysisPage.tsx` |
| `/voices/voice-default/materials` | `/voices/voice-default/materials` | `pages/voice/ui/VoiceMaterialsPage.tsx` |
| `/clips` | `/naver-clip` | `pages/clips/ui/ClipsPage.tsx` |
| `/voices/voice-default/checks` | `/tests/history` | `pages/writing-tests/ui/WritingTestsPage.tsx` |
| `/spoken-voices` | `/spoken-voices` | `pages/spoken-voices/ui/SpokenVoicesPage.tsx` |
| `/spoken-voices/new` | `/spoken-voices/new` | `pages/spoken-voices/ui/NewSpokenVoicePage.tsx` |
| `/clips/review` | `/clips/review` | `pages/clip/ui/ClipPage.tsx` |
| `/video-templates/new` | `/video-templates/new` | `pages/video-template/ui/VideoTemplatePage.tsx` |
| `/video-templates` | `/video-templates` | `pages/video-templates/ui/VideoTemplatesPage.tsx` |
| `/video-templates/video-template` | `/video-templates/video-template` | `pages/video-template/ui/VideoTemplatePage.tsx` |
| `/video-guidelines` | `/video-guidelines` | `pages/video-guidelines/ui/VideoGuidelinesPage.tsx` |
| `/publishing-agents` | `/naver-blog` | `pages/posts/ui/PostsPage.tsx` |
| `/admin` | `/admin` | `pages/admin/ui/AdminPage.tsx` |
| `/admin/models` | `/admin/models` | `pages/admin/ui/AdminModelsPage.tsx` |
| `/admin/estimator` | `/admin/estimator` | `pages/admin/ui/AdminEstimatorPage.tsx` |
| `/admin/vouchers` | `/admin/vouchers` | `pages/admin/ui/AdminVouchersPage.tsx` |
| `/admin/costs` | `/admin/costs` | `pages/admin/ui/AdminCostsPage.tsx` |
| `/billing` | `/billing` | `pages/billing/ui/BillingPage.tsx` |
| `/plans` | `/plans` | `pages/plans/ui/PlansPage.tsx` |
| `/billing/checkout` | `/billing/checkout` | `pages/billing-checkout/ui/BillingCheckoutPage.tsx` |
| `/billing/method/success` | `/billing/method/success` | `pages/billing-method-success/ui/BillingMethodSuccessPage.tsx` |
| `/billing/method/fail` | `/billing/method/fail` | `pages/billing-method-fail/ui/BillingMethodFailPage.tsx` |
| `/account` | `/account` | `pages/account/ui/AccountPage.tsx` |
| `/voice` | `/voices/voice-default` | `pages/voice/ui/VoiceAnalysisPage.tsx` |
| `/voice/materials` | `/voices/voice-default/materials` | `pages/voice/ui/VoiceMaterialsPage.tsx` |
| `/ai-models/compare` | `/tests` | `pages/writing-tests/ui/WritingTestsPage.tsx` |
| `/ai-models` | `/ai-models` | `pages/ai-models/ui/AIModelsPage.tsx` |
| `/ai-models/leaderboard` | `/tests/history` | `pages/writing-tests/ui/WritingTestsPage.tsx` |
| `/ai-models/experiments` | `/tests/history` | `pages/writing-tests/ui/WritingTestsPage.tsx` |
| `/ai-models/experiments/record` | `/tests/records/record` | `pages/model-experiment/ui/WritingTestRecordPage.tsx` |
| `/clips/new` | `/clips/new` | `pages/clip/ui/ClipPage.tsx` |
| `/naver-blog` | `/naver-blog` | `pages/channel/ui/ChannelPage.tsx` |
| `/naver-clip` | `/naver-clip` | `pages/channel/ui/ChannelPage.tsx` |
| `/insta-clip` | `/insta-clip` | `pages/channel/ui/ChannelPage.tsx` |
| `/create` | `/create` | `pages/create/ui/CreatePage.tsx` |
| `/my` | `/my` | `pages/my-work/ui/MyWorkPage.tsx` |
| `/style` | `/style` | `pages/style/ui/StylePage.tsx` |

## Browser and integration evidence

Coordinator command (Vite running from the normal, isolated checkout on port 2583):

```sh
MOBILE_WEB_URL=http://127.0.0.1:2583 MOBILE_OUTPUT=tmp/browser-media/design-system-final node scripts/mobile-route-audit.mjs --serial
```

Matrix: light/dark; 320, 360, 390, 430, 1440 and 1920 CSS pixels. Desktop contexts have a fine pointer and 900/1080 height; phone contexts retain coarse touch and 800 height. Every route checks actual mounted path/leaf/headings, no horizontal overflow, readable form input size, return glyph/round target/name/title/href, at most one action dock, and shared saved-directory cards. Additional deterministic states cover empty/error/draft/running/failed/finalized, valid checkout and invalid recovery, direct template edits, catalog population, 2/4/8/16 model candidates and design selection/read-only behavior. Each of the five persisted video lifecycle states runs at every matrix width/theme. Input DOM/value/focus/caret continuity crosses desktop and keyboard-height viewports; video states additionally check keyboard focus and 200% CSS-zoom reflow. Screenshots include both themes, directories/templates and five clip stages on phone/desktop.

Task-stage probes found stale harness expectations (canonical hubs, current Korean headings, retained guide summary and named render-download revision); these were repaired rather than counted as product failures. No unavailable route was counted as checked. Visual inspection of the light template directory and storyline screenshot confirmed the new composition; final receipts and selected screenshots still require coordinator completion.

Independent Codex fallback review at `018f7c690` found no substantiated product regression and three harness gaps: effective day/night theme mapping, shell-return presence/exact context, and desktop lifecycle device contexts. All three were repaired before the final gate. Strengthened context checks additionally found and repaired a missing-channel video-template return (`channel=undefined`); default Naver and explicit Instagram destinations now have behavioral regressions.

Final full-suite/build/browser receipts: pending coordinator gate. This line is updated only from actual completed receipts.

Limits: local deterministic RPC fixtures verify presentation and interaction, not live provider calls, payment authorization, physical software keyboards, native browser chrome zoom, remote storage/CORS, GPU qualification or media export fidelity. The zoom check uses 200% CSS zoom and the keyboard check resizes available height; they do not certify real-device browser behavior. Existing unqualified media tasks remain separate.
