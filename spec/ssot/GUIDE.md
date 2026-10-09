# GUIDE writing guidelines (작문 지침)
> r19 | Switchable writing direction with stage-applicable rules, one authoritative tag guideline and stable evidence contracts.

## decisions
- GUIDE-1 [o] guidelines own writing direction beside VOICE expression, TMPL form and the stable request contracts (→GEN-14).
  - guidelines outrank template content direction while leaving register to the voice; specific tag selection remains the switchable tags 기본 지침 (→GEN-50)
  - material boundaries, honest semantic origins and photo-order interpretation are request contracts, not optional taste rules (→GEN-80–84) ← owner preferences must not silently redefine what supplied evidence means

- GUIDE-2 [o] an account owns zero or more guidelines (`guidelines`), each of one kind for good — for posts, the user-facing noun 지침, or for clips, 영상 지침 (→GUIDE-44); a guideline has two authored fields, `text` and an optional `title` (→GUIDE-46), plus its kind, a scope and timestamps; every store query and RPC is scoped by the authenticated user; a foreign guideline id, or a foreign template id named in a scope, is `NotFound`
- GUIDE-3 [o] nothing references a guideline: posts, jobs and experiments carry frozen texts, never ids, so deleting one detaches nothing and leaves every enqueued run readable
- GUIDE-4 [o] nothing seeds `guidelines`, `guideline_templates` or `guideline_candidates`: an account starts with zero of each
  - `guidelines(user_id, kind, text)` is unique so an exact duplicate after trim is refused by the database ← a service check could be passed by two concurrent creates
  - `guidelines(id, user_id)` is the composite target `guideline_templates` points at
  - `guideline_candidates` is unique on `(user_id, kind, text)`, and its `post_slug` — or, for a 영상 지침 candidate, the clip project's id — is a plain nullable column with no foreign key
- GUIDE-5 [o] field rules:
  - `text` trimmed, non-empty, ≤ `GUIDELINE_TEXT_MAX_CHARS` (300) Unicode scalar values, unique among the account's guidelines of its kind after trim (`AlreadyExists`)
  - `scope` is `global` (must carry no template ids), `templates` (must carry ≥ 1 distinct owned template id of the guideline's own kind — post templates for a 지침, video templates for a 영상 지침; an unknown, foreign or other-kind id is `NotFound` and nothing is applied; duplicates in one request collapse to one link) or, for a 지침 alone, `fields` (must carry ≥ 1 분야 from the product's list; an unknown 분야 is `NotFound`, and `fields` on a 영상 지침 is `InvalidArgument`)
  - an unset scope on the wire is refused, never defaulted ← the one shape that must not be guessed is the one that applies a rule to every run
  - creating beyond `GUIDELINE_MAX_PER_ACCOUNT` (100) guidelines of one kind is `FailedPrecondition` naming the cap, checked inside the insert transaction
- GUIDE-6 [o] UpdateGuideline uses presence: a text-only update leaves the scope untouched; a scope patch replaces the whole scope — kind and link set together — atomically in one transaction ← a scope is only meaningful as both halves at once; a text edit runs the same candidate approval as a create
- GUIDE-7 [o] a candidate (후보) is one completed revision's instruction, recorded so a correction accrues instead of vanishing with the tab: a post revision's (→GEN-42) becomes a 지침 candidate and a clip revision request's (→CLIP-131) a 영상 지침 candidate; it is a receipt for something the user wrote, never a model's opinion; recording happens inside the revision's completion path after the revised content or plan is persisted, creates no job and calls no provider, and a recording failure never fails the revision; a storyline request records none (→GEN-69 →CLIP-180) ← it edits one piece's draft plan rather than correcting what was written
- GUIDE-8 [o] the text is stored verbatim, trimmed at the edges and otherwise untouched — nothing rewrites, summarizes, normalizes, generalizes, translates, clusters, scores or ranks it; a candidate carries no scope at all ← scope is a durable decision about every future post and is made at approval, which is what makes automatic recording safe
- GUIDE-9 [o] the bound split: a candidate is stored at the revision instruction bound (500), not the guideline bound (300) ← refusing a long instruction at recording time would lose exactly the most specific corrections; the guideline bound is enforced at approval with the live counter; a candidate is never silently truncated
- GUIDE-10 [o] candidate statuses `pending` · `approved` · `dismissed`, all three kept ← the row itself is what suppresses re-recording
  - each kind keeps its own queue, and deduplication is exact after trim within the kind and nothing else (no similarity, fuzzy or semantic matching):
  | sighting | outcome |
  |---|---|
  | first sighting with room | `pending`, `occurrences = 1` |
  | a repeat of a pending candidate | `occurrences + 1`, `last_seen_at` advances, `post_slug` is not rewritten (the candidate names where the correction was first seen) |
  | text already a saved guideline of the kind, or an approved or dismissed candidate | nothing recorded, nothing revived |
  | `GUIDELINE_CANDIDATE_MAX_PENDING` (50) pending rows of the kind | nothing recorded, the queue stops rather than evicting ← evicting the oldest would discard something the user might have approved |
  - the dedupe read, the guideline check and the pending count run in one transaction
- GUIDE-11 [o] approval is `CreateGuideline` of the candidate's kind — there is no Approve procedure ← the create already owns every field rule, bound, cap and refusal an approval needs
  - it takes an optional `from_candidate_id` and marks the candidate approved in the same transaction as the insert, by that id when present and by the saved text either way (which is what marks an on-the-spot `지침으로 저장` recorded without the client learning its id)
  - only a `pending` candidate may be moved and a terminal one reads `NotFound`, rolling the create back, so a stale tab can neither approve twice nor dismiss an approved one
  - an unmatched `from_candidate_id` is `GUIDELINE_CANDIDATE_NOT_FOUND`
  - a refused create (bound, cap, duplicate) approves nothing
- GUIDE-12 [o] 무시 marks the row `dismissed`: nothing is deleted, there is no confirmation dialog, and no undo beyond writing the guideline by hand; the same instruction from a later revision does not reappear
- GUIDE-13 [o] deleting the source post drops the candidate's `post_slug` and keeps its text (listed without a link), after the post row is actually gone, with a detach failure logged and never failing the delete (→POST-29), and deleting a clip project does the same for its 영상 지침 candidates (→CLIP-24); deleting a saved guideline neither revives nor resets its approved candidate — re-creating the rule is an explicit create
- GUIDE-14 [o] scope resolution:
  - a post with template P and 분야 F receives the enabled 기본 지침 of the 지침 kind, the account's `global` 지침, those linked to P and those linked to F
  - a clip project with video template V receives the enabled 영상 지침 기본 지침, the `global` 영상 지침 and those linked to V
  - a run missing a template or a 분야 receives only the groups it has
  - a `templates` guideline whose every template was deleted (적용 대상 없음) reaches no prompt until it is rescoped
  - injection order is the 기본 지침 in the product's order, then the global group, then the template group, then the 분야 group, each owner group by `created_at, id` ascending, and each management screen lists its guidelines in exactly that order ← what the user sees is what the writer is given
- GUIDE-15 [o] applicable enabled guidelines render in one `[작문 지침]` section at the defined position, after template or voice when present, before per-post material.
  - preserve GUIDE-14 source order as verbatim hyphen-bulleted lines; close with explicit guideline-over-template, voice-register and owner-over-stock precedence, omitting absent voice clauses
  - section heading and precedence sentence remain Korean for every target; localized stock text follows the output target, owned text is not translated, and the frozen target outranks conflicting language instructions
  - for no voice/template, place the section after static contracts; keep voice/template sections byte-identical with and without guidelines
  - code-owned stock rules use declared stage applicability; storyline omits stock final title/tag/prose-only directions (→GEN-68 →GEN-85)
  - scope-resolved owner lines remain verbatim in their frozen order; never infer stage exclusion from arbitrary owner text, split it, translate it or ask a paid classifier to filter it
  - no enabled applicable rules means no section; observe receives none; clip writing carries applicable 영상 지침 under CLIP-183, preserving its own stage semantics

- GUIDE-16 [o] the product ships 기본 지침 for each kind — code constants, never rows or config: each carries a fixed name and text, is marked (추천), reaches every account with zero setup until its owner switches it off, and is never edited, rewritten, learned or model-written; the 지침 set is →GUIDE-41 and the 영상 지침 set →GUIDE-42 ← a rule about what may be written is the product's recommendation rather than its format, so the owner may turn one off where the writing they want needs its opposite
- GUIDE-17 [o] freezing: StartGeneration, StartRevision and the storyline jobs resolve the post's current `template_id` and 분야 once — the same value the template brief is resolved from — and write the enabled 기본 지침 and applicable ordered texts into the payload beside target length/template; unified tests freeze this ordered set, with only the selected guideline slot varying and no scope recalculation when templates vary
  - tests keep nonvaried guideline texts byte-identical, changing only the designated guideline slot when selected; the complete variant set affects the snapshot hash
  - handlers read only the payload, so editing, rescoping, deleting or switching after a start changes nothing in flight, across restart-resume and retry
  - payloads without the member decode as no guidelines (→GEN-15)
  - a clip freezes its 영상 지침 with the attempt the same way (→CLIP-69)
- GUIDE-18 [o] completed-revision candidates remain verbatim owner requests and are never inferred, ranked or auto-approved; ordinary generation uses only saved guidelines and enabled stock rules; isolated MODEL tests may use an explicitly validated owned unpublished contender under EDIT-23. Explicit AI authoring under EDIT creates separate private drafts and may generate or refine a guideline, published only after Save. Nothing validates generated post output against a guideline.
- GUIDE-28 [o] a guideline outranks a memory exactly as it outranks a template instruction: a fact the memory bank carries is not written when a guideline forbids it, and the precedence sentence beside the `[기억]` section says so (→MEM-21) ← a prohibition the user added is the reason the fact must stay out of this post, and the memory bank has no per-post opt-out of its own
- GUIDE-19 [o] ordinary post/clip generation has no per-item guideline opt-out, owner-rule toggles or implicit selection; stock rules retain their explicit account on/off controls. Unified tests may vary one designated guideline slot in an isolated snapshot and disclose the compared rule without rescoping ordinary settings. Stock text editing, cross-voice scoping and automatic candidate approval remain outside guidelines.
- GUIDE-20 [o] post guidelines are a named writing-settings directory in injection order: enabled stock rules then owned rules. Rows show name/title or readable text fallback, application scope and owned EDIT saved/unpublished/job state; stock rules retain recommendation and on/off behavior.
- GUIDE-21 [o] after a completed revision (`done`, not merely terminal ← a failed revision produced nothing worth saving), `지침으로 저장` — the one save a revision offers — opens a dialog seeded with the revision instruction, editable, offering 전역 (default) and the post's current template when it has one (read from the loaded post, no new query)
  - it calls the standard create
  - `AlreadyExists` renders as already-saved information
- GUIDE-22 [o] the 후보 queue sits below the saved list as a closed disclosure whose summary carries its pending count (`지침 후보 (3)`) ← the saved rules are what the screen is for and an unreviewed suggestion is not a form
  - rows follow the server's review order (occurrences descending, then last-seen descending), each showing its text, its occurrence count when above one, and its source post as a link (plain text when the post is gone)
  - the disclosure renders nothing when nothing waits and the queue has room, and nothing on a failed candidate read ← the saved list owns the page's error state
  - when the pending queue is full the summary says so from the server's `queue_full` — the client owns no copy of the bound
- GUIDE-23 [o] 승인 opens the entity's scope control (전역 preselected) over an editable text with the live 300-character count and calls the create with `from_candidate_id`
  - a duplicate-text refusal keeps the dialog open, says the rule already exists, and re-reads the candidate list ← another tab usually saved it and thereby approved this row
  - 무시 has no dialog
  - 전부 수락 and 전부 거절 sit once at the head of the open disclosure (→GUIDE-27)
  - the candidate list is keyed `(accountId, 'guideline-candidates')` with `staleTime: 0` and `refetchOnMount: 'always'` ← a candidate arrives from a revision run in another tab
  - a create invalidates both lists, a dismissal only the candidate list
- GUIDE-24 [o] server refusal messages are rendered through the shared failure catalogue; the client predicts none of them and mirrors neither the per-account cap nor the pending-candidate bound
- GUIDE-25 [o] the scope control lives in `entities/guideline/ui` ← the create form and the whole-scope edit need the identical control and a feature may not import a sibling feature; it reads the template directory through `entities/template/@x/guideline.ts`; the write callers live in `entities/guideline/api` because the revision capture needs the create one
- GUIDE-26 [o] 지침 and 영상 지침 are writing and video settings destinations under THEME-38, each retaining the 44 px pointer floor at 320 px.
- GUIDE-27 [o] 전부 수락 runs the standard create once per listed pending candidate with 전역 scope and `from_candidate_id`, keeping every one that passes and leaving each refusal (over 300 characters, a duplicate, the account cap) in the queue with its catalogue reason for a single 승인 to fix ← one over-long candidate must not hold back the rest, and a rule that needs a scope other than 전역 is exactly the one worth opening
  - 전부 거절 dismisses every listed candidate behind one `Dialog` naming the count ← there is no undo and a single 무시 stays dialog-free (→GUIDE-12)
  - neither action is offered while the list is empty and both re-read the candidate list, a create additionally invalidating the saved list
- GUIDE-35 [o] vocabulary carries its own precedence, stated in the prompt: a guideline's substitution beats a template's, a template's beats the voice profile, and 문체 and 종결어미 stay with the voice either way ← otherwise the voice profile's vocabulary would overrule every substitution
- GUIDE-36 [o] only a concrete substitution carries that authority: a guideline or a template may say to write B where the source already says A, while an abstract instruction about better words carries none ← a vague vocabulary instruction cannot be checked against the source, which is what the grounding constraint requires
- GUIDE-37 [o] the owner's own guideline outranks a conflicting 기본 지침; within the same source the earlier applicable line wins.
  - this preference precedence cannot override evidence boundaries, semantic-origin honesty or the photo chronology contract (→GEN-80–84)

- GUIDE-41 [o] the 지침 kind's 기본 지침, in this order:
  | 기본 지침 | rule |
  |---|---|
  | 재료에 있는 사실만 | →GEN-16 |
  | 내 감상을 지키고 AI 제안 구분 | →GEN-16 →GEN-83 |
  | 기억을 통한 감상 추가, only in a prompt carrying `[기억]` | →GEN-73 |
  | 메모의 이름으로 | →GEN-44 |
  | 알려준 사건 순서대로 | follow event order explicitly supplied by the owner inside template places; otherwise propose subject arrangement, never infer chronology from photo order (→GEN-82) |
  | 첫머리에 이유와 기대 | opening with why the owner went or what they expected, when the memo says so |
  | 사진은 이야기의 한 장면 | a photo stands between related sentences, no paragraph opens as a photo description and paragraphs connect without inventing actions or chronology (→GEN-82) |
  | 사진은 한 장씩 | most photos stand alone, one at a time; consecutive photos stand as one photo group only in two cases — one composition in which only what sits with the subject changes (the same meat once with green onion, once with kimchi), or one subject shot from several angles — and only under a caption naming one subject (다양한 밑반찬과 함께한 고기, 잘 차려진 한 상); a caption that must join different photos' subjects with 와/과 or 및 (가게 입구와 첫 상차림) means each photo stands alone with its own caption, even at a template's suggested group place or inside one storyline paragraph; a group is 콜라주 to see its photos side by side and 슬라이드 to follow them in order (→GEN-77) ← side-by-side photos shrink on a phone screen, and a group has one caption to say what it shows |
  | 끝에서 한 번 정리 | closing by drawing the day together, a verdict or a will to return only as the owner gave one |
  | 관찰을 나열하지 않기 | →GEN-47 |
  | 제목 규칙 | →GEN-49 |
  | 태그 규칙 | →GEN-50 |
  | 같은 종결어미 세 번 잇지 않기, for a Korean target alone | →GEN-75 |
  | 자연스러운 한국어 문체, for a Korean target alone | →GEN-17 |
- GUIDE-42 [o] the 영상 지침 kind's 기본 지침, in this order:
  | 기본 지침 | rule |
  |---|---|
  | 입력한 사실만 | a name, number, unit, currency or price stated exactly as one entered global or item fact states it, never assembled from several answers, naming the item it belongs to where the narration would leave it ambiguous, and no other fact the material does not carry |
  | 감상은 내가 쓴 것만 | a taste, a mood or a satisfaction only as the owner gave it in the instruction or the storyline |
  | 첫 자막에 무엇을 보여줄지 | the first caption says in one line what the clip is about (구리에서 찾은 숨은 오리집) |
  | 자막은 이어지는 말 | each caption carries on from the one before rather than labelling what is on screen, and may run across cuts |
  | 일어난 순서대로 | cuts and captions follow the order it happened, inside each template stage when there is a template |
  | 끝에서 정보 정리 | the last captions draw together what the owner entered (name, place, price) |
  | 같은 홍보 문구 되풀이하지 않기 | no generic promotion said twice |
- GUIDE-43 [o] whether a 기본 지침 is in use belongs to the account and its kind: `추가` in the 기본 지침 sheet (→GUIDE-48) puts one in use and `적용 안함` on its open row takes it out of use, each saving on press with no confirmation ← putting one back is one press in the sheet
  - out of use it leaves the list and stays offered in the sheet, and it leaves every run started afterwards, while work already enqueued keeps the texts it froze (→GUIDE-17)
  - a new account starts with every 기본 지침 in use, and no other surface changes one
- GUIDE-44 [o] `/video-guidelines` (nav 영상 지침, a video settings destination under THEME-38, lazily split) is `/guidelines` for the 영상 지침 kind:
  - the same one list of closed rows (→GUIDE-20), open rows (→GUIDE-47) and 기본 지침 sheet (→GUIDE-48), an owner row's scope badge reading `전역`, `영상 템플릿` or `적용 대상 없음` and its open row carrying the video-template-name chips
  - its dock holds `기본 지침` and `새 영상 지침`, whose Sheet's scope control offers 전역 / 특정 영상 템플릿
  - below the list the `영상 지침 후보` disclosure behaving as GUIDE-22, GUIDE-23 and GUIDE-27 describe, each candidate linking the clip it came from (plain text once that clip is gone)
  - its queries are keyed `(accountId, 'video-guidelines')` and `(accountId, 'video-guideline-candidates')` with `staleTime: 0` and `refetchOnMount: 'always'`, and a video template rename or delete marks the list stale
- GUIDE-45 [o] after a completed clip revision request (→CLIP-131), ②'s composer offers `영상 지침으로 저장`, which opens the standard create seeded with that request, offering 전역 (default) and the project's video template when it has one, as `지침으로 저장` does for a post (→GUIDE-21)
- GUIDE-46 [o] a guideline's `title` is optional: trimmed, ≤ `GUIDELINE_TITLE_MAX_CHARS` (40) Unicode scalar values, empty meaning none, and never unique; a row without one shows its text cut to one line in its place ← 전부 수락 and the revision capture stay one step, which a required title would break
  - every surface that creates a guideline offers the title field empty — `새 지침` and `새 영상 지침`, 승인 (→GUIDE-23), `지침으로 저장` (→GUIDE-21) and `영상 지침으로 저장` (→GUIDE-45) — and 전부 수락 creates without one (→GUIDE-27)
  - the title never enters the generation prompt; tests may snapshot it as reveal/history metadata while only rule text is injected (→GUIDE-17)
- GUIDE-47 [o] closed guideline rows open independently to readable text and explicit scope.
  - stock rules expose their target note and on/off action, not editing
  - owned rules offer named AI editing/direct editing of one shared draft and explicit deletion; Save changes title/text/scope atomically under domain bounds and preserves failed input
  - confirm deletion explains that already-admitted work keeps its frozen text
- GUIDE-48 [o] the dock's `기본 지침` opens the shared `Sheet` listing every 기본 지침 of the page's kind in the product's order (→GUIDE-41 →GUIDE-42), each with its name, text and target note ← choosing one needs its text, which the closed list no longer shows
  - one out of use carries `추가`, which puts it in use at once (→GUIDE-43) and in the list at its product position, and the sheet stays open ← adding several is one visit
  - one in use reads `적용 중` and carries no control; taking one out of use happens on its row, never in the sheet

- GUIDE-49 [o] owned post/video guidelines share EDIT named saved-item state, AI/direct editing and explicit publication. Creation displays the chosen scope; AI changes retain captured scope/version, while an explicit owner direct edit may change permitted scope under the same version fence and conflicts preserve drafts. Post-guideline tests vary one rule slot under MODEL-30 and publish a champion only with explicit scope; generated drafts never enter the completed-revision candidate queue.
- GUIDE-50 [o] code-owned rule applicability is declared by stage and output responsibility, while its text and preference authority remain with its owning guideline.
  - arbitrary owned rules retain GUIDE-14/17 scope, order and verbatim freezing; stage contracts determine the current output without silently selecting or editing those lines (→GUIDE-19)
  - do not duplicate stock tag quality in a static format contract or erase useful long rules merely for their length
  - one inspection can identify included/omitted rules and reasons from the same effective composition (→MODEL-93/94); origin validation is distinct from automatically judging compliance with guidelines (→GUIDE-18)

## flow
- author: `/guidelines` | `/video-guidelines` → 새 지침 | 새 영상 지침 → title(optional) + text + scope(전역 | templates | 분야, posts only) → CreateGuideline(bound, cap, dedupe within the kind → row; approves a matching candidate)
- 기본 지침: dock 기본 지침 → sheet(every 기본 지침; in use | 추가) → 추가 → in use, in the list at its product position → open row → 적용 안함 → out of use, offered in the sheet only → the next run's frozen set
- browse: list(closed rows: name | title | text cut to one line) → press a row → open(text, scope or target note, 수정 · 삭제 | 적용 안함) → AI editing | direct editing → shared draft(title + text + scope) → explicit named Save | Cancel
- inject: StartGeneration / StartRevision / storyline job(resolve template id and 분야 once → ForPrompt(enabled 기본 지침 + global + template-linked + field-linked) → freeze texts) → `[작문 지침]` after the template section → model; clip attempt(enabled 영상 지침 기본 지침 + global + video-template-linked → freeze) → storyline, flow, narration and revision calls
- accrue: post revise done | clip revision request done → record instruction(kind; dedupe → pending +1 | nothing | queue full) → 지침 후보 | 영상 지침 후보 disclosure(closed, counted) → 승인(create with from_candidate_id) | 무시 | 전부 수락(전역, refusals kept) | 전부 거절

## constraints
- config: `GUIDELINE_TEXT_MAX_CHARS` 300 and `GUIDELINE_TITLE_MAX_CHARS` 40 (BE `platform/config` env · FE `shared/config` `VITE_GUIDELINE_TEXT_MAX_CHARS` and `VITE_GUIDELINE_TITLE_MAX_CHARS`, falling back to the defaults) · `GUIDELINE_MAX_PER_ACCOUNT` 100 · `GUIDELINE_CANDIDATE_MAX_PENDING` 50, each per kind (BE only; malformed values are boot-fatal); the heading, precedence sentence, every 기본 지침's name, text and order, injection ordering, candidate statuses and review order, the candidate storage bound (the revision instruction bound) and the proto/SQL schema are code
- schema: `guidelines(id, user_id, kind, title, text, scope, created_at, updated_at, UNIQUE(user_id, kind, text), UNIQUE(id, user_id))` · `guideline_templates` (composite FKs to guideline and a template of its kind) · `guideline_fields` (guideline ↔ 분야) · `guideline_candidates(id, user_id, kind, text, status, occurrences, post_slug NULL, clip_id NULL, first/last_seen_at, UNIQUE(user_id, kind, text))` · the account's switched-off 기본 지침 by kind and name; migrations 0014, 0023, 0078
- placement BE: `backend/internal/guideline` (`ForPrompt` resolving by template and 분야, candidates, store, rpc; consumed by generation and post through consumer-declared ports; template names through the directory port, never a SQL join)
- placement FE: `entities/guideline` (model, api, ui scope control) · `features/create-guideline` `edit-guideline` `delete-guideline` `review-guideline-candidate` · `features/edit-with-ai` (the capture) · `pages/guidelines` · `pages/video-guidelines`
- contract: `proto/postpilot/v1/guideline.proto`

## chg
- r19 261009 GUIDE-41✎ 사진은 한 장씩 grouping condition: one caption fully describes every photo (one dish from several angles) → only one composition with changing companions or one subject from several angles, under a caption naming one subject; a 와/및 caption joining different photos' subjects splits them, even at a template's suggested group place or inside one storyline paragraph
