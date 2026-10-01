# IDEATION template-from-request
> st:converted@261001 | say the template you want in words and get a body in our grammar from the 글 작성 모델, inside the app, while the outside-AI path stays; and see the post a template makes in a live preview beside the composition

## vision
- [o] Problem: a template today is made one of two ways — click blocks together in the builder, or copy the 형식 안내, leave the app for an outside AI, and paste its answer into `원문` (→TMPL-41 →TMPL-42); an owner who can describe the post they want in one sentence still has to do one of the two
- [o] Target: a post owner who knows the shape of the post they want but does not want to assemble blocks or switch apps
- [o] Problem: the editor shows a template only as an outline of blocks (→TMPL-26), so the owner cannot see what a post made from it will look like until one is generated ← owner report: a template without a preview is hard to actually use
- [o] Core value: describe the template in words → a draft in our grammar appears in the editor, ready to review and save, without leaving the app; and every edit, by hand or by request, shows at once what post the template makes

## explored
- [o] the outside-AI path stays as it is (형식 안내 복사 → outside AI → paste into `원문`) ← owner direction; it works with any AI and costs no credit
- [o] in-app generation is an API call to the account's active write model (the 글 작성 모델) ← owner direction; no new model purpose for the operator to curate
- [o] the result is a draft and never saves itself: it fills the editor, the owner reviews it in the builder or `원문`, and the one 저장 commits it (→TMPL-25) ← derived; the one-draft-one-저장 rule already decides this
- [o] the result passes the same parser and field rules as a pasted body and uses only the authorable constructs the 형식 안내 teaches (→TMPL-7 →TMPL-41) ← derived
- [o] a generated `<write>` names what stands at its place, never how to write it (→TMPL-57) ← derived; the form-alone rule does not depend on who wrote the body
- [o] credits follow every other AI job: a free model costs 0, a paid model reserves and settles confirmed usage, and the plan entitlement check applies (→QUOTA-10 →QUOTA-19) ← derived from QUOTA
- [o] input is one free-text box that takes a description, a sample post, or both mixed ("원하는 템플릿을 설명하거나 참고할 글을 붙여넣으세요") ← owner choice; one box keeps the screen short on a 360 px phone and covers "like the post I wrote before"
  - [x] description only ← leaves "make it like this post" unsolved
  - [x] two boxes, description and sample ← clearer roles but a longer screen for the same input
- [o] the generation fills `body`, `title_area`, `name` and `description`, and leaves `target_length` and `tag_count` unset ← owner choice; one request reaches the state just before 저장, while the two numbers are the owner's own opinion and a set number is copied onto every post the template is assigned to (→TMPL-48)
  - [x] body only, as the outside-AI path gives ← the owner would still name the template and author the title form by hand
  - [x] every field including the two numbers ← an AI-picked number would silently seed post options
- [o] follow-up edits: every request sends the current draft with it and says only what to change ("사진 줄을 2장으로 바꿔줘"), so one box serves a new template and a stored one alike ← owner choice
  - [x] one shot, a new request starting over ← every small change would mean regenerating or hand-editing
  - [x] a chat that remembers earlier requests ← conversation history to keep, and every turn's input grows and costs more
- [o] wishes about how to write (tone, length, what to leave out — "친근하게, 짧게, 가격은 빼고") are kept out of the template and shown beside the result as "이 부분은 지침에 넣으세요"; the owner makes the guideline by hand if they want one ← owner choice; the template holds form alone (→TMPL-57) and no model writes a guideline (GUIDE-18 stands)
  - [x] also draft a guideline linked to the template ← would reverse GUIDE-18 as well
  - [x] drop them silently ← the owner could not tell why part of the request had no effect
- [o] a body that does not parse goes back to the model with the parser's line and reason to write again, up to 3 corrections as clip composition does (→QUOTA-54); when it still fails the draft stays untouched and the owner is told the request failed, confirmed usage charged as any failed AI job (→QUOTA-46) ← owner choice; free models get tags wrong more often, and the owner should never be handed a broken body to repair
  - [x] put the broken body into `원문` with its error line (→TMPL-30) ← the owner would be repairing the model's tags
  - [x] one attempt, then fail ← cheapest, but failures would be frequent on free models
- [o] the result replaces the draft at once, and one 요청 전으로 되돌리기 restores the draft as it was before the request ← owner choice; the result is checked in the real builder right away, and hand edits made before the request are never lost
  - [x] a preview with a separate 적용 ← one more press and a preview screen of its own
  - [x] replace with no undo ← hand edits before the request could be lost
- [o] the request box sits at the top of the editor: open as the first action on an empty new template, a collapsed AI에게 요청 button once the draft has content ← owner choice; the first-time path starts from words, while an author editing by hand is not pushed down by a box they are not using
  - [x] always open ← on a phone the builder is pushed below the fold
  - [x] a separate AI로 만들기 entry on `/templates` ← splits the one `새 템플릿` entry (→TMPL-24) in two
- [o] a sample post's sentences are not carried over as 고정 문구 by default: the structure becomes the template's places and every sentence becomes what an AI가 쓰는 글 is about; a sentence becomes a 고정 문구 only when the request says so ("인사말은 그대로") ← owner choice; a pasted post may be someone else's, and their blog name or greeting must not land in the template unasked
  - [x] keep repeated greetings and closings as 고정 문구 automatically ← convenient for the owner's own post, but another blogger's name would come along
  - [x] never carry a sentence ← the owner could not keep their own greeting without typing it in
- [o] while a request runs the draft is locked with a 취소 beside it; leaving the screen warns first and cancels the request, charging confirmed usage only (→QUOTA-49) ← owner choice; the result lands in an unsaved draft, so a draft edited meanwhile or a screen already left has nowhere safe to put it
  - [x] keep editing and let the result overwrite ← edits made while waiting would be lost unless undone
  - [x] keep the result for later and offer it on the next visit ← a pending result per template to store
- [o] the box shows the 글 작성 모델's name and 약 n 크레딧 per request before it is pressed, or 무료 for a free model ← owner choice; a paid model charges every request and the owner should know roughly how much before pressing, as model selectors already show 글 1개당 크레딧 (→QUOTA-64)
  - [x] model name only ← the cost would surface only in usage history
  - [x] 무료 / 크레딧 차감 only ← says that it costs, not how much
- [o] the wishes kept out of the template are listed beside the result with a 지침 만들기 link that opens an empty new guideline; the owner writes the guideline in their own words ← owner choice; the model only separates them, never writes or suggests a guideline (GUIDE-18 stands)
  - [x] open the new guideline prefilled with the separated wishes ← where a guideline starts and ends would be the model's cut, which would reverse GUIDE-18
  - [x] a notice with no link ← one more step to find the guideline screen
- [o] a post screen offers 이 글 형식으로 템플릿 만들기, which opens a new template with that post as its sample ← owner choice; an in-app post carries its photo positions as blocks, which a post pasted from Naver loses
  - the post arrives as a 참고 글: <제목> chip above an empty box kept for extra wishes ("인사말은 그대로"); the post goes as structure with its photo positions, and removing the chip makes an ordinary request ← owner choice
    - [x] fill the box with the post as text ← a long box, and every photo position becomes a text marker such as [사진]
  - the owner presses the request button on the new template themselves; opening it never starts a call ← derived from the cost shown before pressing
  - the sample is the post's own content (title and body with its photo positions); its memo, photos, observations, voice and guideline are not sent ← derived; a template is the form of the finished post
  - TMPL-16's "no template is chosen from a post's material" still holds for anything automatic: this is an explicit owner action that makes a new template, never a pick
- [o] an account already at `TEMPLATE_MAX_PER_ACCOUNT` sees the box disabled on a new template with the reason, while a stored template's box stays usable ← derived; credits must not be spent on a draft whose save is refused (→TMPL-6)
- [o] with no usable 글 작성 모델 (none selected, disabled, outside the plan, or short of credits) the box is disabled with the same reason post generation gives and a link to model selection ← derived; one model, one set of refusals (→QUOTA-19 →MODEL-24)
- [o] the 형식 안내 for an outside AI stays as it is, body-only ← derived from keeping the outside-AI path unchanged
- [o] one 형식 안내 is shared by both paths: the backend owns it as code (prose in both UI languages, the example, the limits), the client fetches it for 형식 안내 복사, and the template request teaches the 글 작성 모델 with the same text ← owner asked for one shared guide and left the store to this session; it must change in the same commit as the grammar (→TMPL-41), and the example stays parsed by the real parser in a test
  - [x] a DB row ← editable without a deploy, but a row could teach a grammar the parser refuses, and prompt wording in this product is code (the template section's legend, 기본 지침 →GUIDE-16)
  - [x] the client text as it is plus a second server copy ← two teachers of one grammar that must change together
- [o] a live preview stands to the right of the composition and redraws on every edit, filled with hardcoded stand-ins — no model call, no credit ← owner direction; a template is used by seeing the post it makes, and a preview that called a model would charge for every keystroke
  - it shows the post the way the post's reading view shows one, so what the owner sees is what a post made from the template looks like ← derived
  - it follows the one draft whatever edits it: the builder, `원문` as typed (the last parsable state with the parse error noted while it does not parse →TMPL-42), a request's result and an undo ← derived from one draft, one representation (→TMPL-2)
  - it shows no grammar syntax (→TMPL-26) and reads none of the two generation numbers ← derived
  - stand-ins are placeholder boxes: an AI가 쓰는 글 is a grey paragraph box carrying what its place is about (메뉴 소개), a 사진 is a row of `count` grey photo cells, a 고정 문구 is its own text, and a 데이터 받기 field shows its 제목 with an 입력한 내용 placeholder ← owner choice; what is fixed and what the AI writes stays visibly apart, and a box fits a template on any subject
    - [x] plausible sample sentences and photos ← fit only one subject (맛집) and read as if those sentences would come out
    - [x] neutral sample sentences with the AI parts tinted ← shows the density of a post, but a set of sample sentences to maintain
  - a 사진마다 반복 is drawn twice, marked 사진 그룹마다 반복 ← owner choice; two passes show that it repeats, with nothing to operate
    - [x] a 1 … 5 photo-group stepper ← an operable control on a hardcoded preview
    - [x] drawn once with the mark ← how one pass leads into the next stays unseen
  - on a wide screen it stands to the right of the composition and stays in view while the composition scrolls; on a phone a 구성 / 미리보기 SegmentedControl switches the two ← owner choice; a 360 px screen has no room beside the composition, and each view keeps the full width
    - [x] the preview below the composition on a phone ← a long composition pushes it far down, and the edit and its result are never seen together
    - [x] no preview on a phone ← phone owners would have none
- [o] calibration: the request box takes 12,000 characters (room for a 10,000-character post and a description), the wishes list at most 5 of 200 characters each, and 약 n 크레딧 is the one-call catalog estimate (→TMPL-58 →TMPL-61 →QUOTA-67) ← owner choice on the box; the estimate because QUOTA-64's figures are per post from generate jobs
- [o] the reversal is scoped to this one action: TMPL-16 and TMPL-34's "no model writes a template, no template surface makes a provider call" is lifted for an explicit owner request only; nothing is learned from posts, suggested, defaulted or picked automatically ← keep the rest of TMPL's guarantees

## shape
- flow: template editor (new or stored) → one box: a description and/or a sample post, or what to change → 글 작성 모델 with the current draft → parse(fail → back to the model with line + reason, ≤ 3 corrections | still failing → draft untouched + failure shown) → name · description · title area · body in the draft + the wishes that belong to 지침 shown beside it → review in the builder or `원문`(요청 전으로 되돌리기 restores the previous draft) → 저장
- box: open at the top of an empty new template, a collapsed AI에게 요청 button once the draft has content; the 글 작성 모델's name and 약 n 크레딧 (or 무료) beside the request button; while running the draft is locked with 취소, and leaving warns then cancels
- from a post: post screen → 이 글 형식으로 템플릿 만들기 → new template with a 참고 글: <제목> chip above an empty box → the owner presses 요청
- preview: composition | live preview to its right (kept in view while the composition scrolls; 구성 / 미리보기 switch on a phone), redrawn on every draft change from placeholder boxes, a 사진마다 반복 drawn twice
- beside the result: the wishes that belong to 지침 + 지침 만들기 (an empty new guideline)
- v1: in-app generation and follow-up edits beside the unchanged outside-AI path, automatic correction, one-step undo, the cost before pressing, an entry from a post, a live hardcoded preview / not: a model-written preview, the two generation numbers, a drafted guideline, a remembered conversation, a preview step, sample sentences carried over unasked

## domains
- TMPL: the generation action, its input and output, the scoped reversal of TMPL-16 / TMPL-34 / TMPL-41; the live preview beside the composition →TMPL
- QUOTA: the 약 n 크레딧 figure for one template request; charging, failure and cancellation follow the existing AI-job rules →QUOTA
- POST: the 이 글 형식으로 템플릿 만들기 entry on a post screen and what of the post is sent as the sample →POST
- MODEL: the account's active write selection also serves template requests, with the same entitlement and availability checks →MODEL
- GUIDE: no rule change (GUIDE-18 stands); only the 지침 만들기 link from the template result, written in TMPL-61 →TMPL

## open
- a template request is a job like every other kind, and how long any job keeps its content is open for every kind in `JOB-RETENTION-TODO.md` ← owner direction
