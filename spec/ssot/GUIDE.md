# GUIDE writing guidelines (작문 지침)
> r10 | Account-owned writing direction — what kind of writing is wanted — for posts (지침) and for clips (영상 지침): the product's recommended 기본 지침, on until the owner switches one off, beside the owner's own rules applied to every run or scoped to templates or 분야, frozen at enqueue, capturable from the revision flow, accrued verbatim as candidates from completed revisions, and never learned.

## decisions
- GUIDE-1 [o] a guideline is the direction layer: it says what kind of writing is wanted — what a post or a clip states and leaves out, the order it tells things in and how its sentences are written beyond their register — beside the voice (how sentences sound, VOICE), the template (the form, TMPL) and the system prompt, which holds the input and output format alone (→GEN-14); a guideline outranks the template on content while leaving register to the voice ← a rule about what may be written, fixed in the system prompt, would forbid every kind of writing that needs its opposite
- GUIDE-2 [o] an account owns zero or more guidelines (`guidelines`), each of one kind for good — for posts, the user-facing noun 지침, or for clips, 영상 지침 (→GUIDE-44); a guideline has exactly one authored field, `text`, plus its kind, a scope and timestamps; every store query and RPC is scoped by the authenticated user; a foreign guideline id, or a foreign template id named in a scope, is `NotFound`
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
- GUIDE-15 [o] the write, revise and storyline prompts render one `[작문 지침]` section at one position — after the `[글 템플릿]` section when the post has a template, otherwise directly after the voice profile's `[종결어미 제약]`, or after the static rules in the storyline prompt, which carries no voice (→GEN-68), always before `[이번 글]` — as hyphen-bulleted verbatim lines closed by a fixed precedence sentence: a guideline outranks the template where they conflict, register stays with the voice, and the owner's own line outranks a conflicting 기본 지침 (→GUIDE-37)
  - the heading and the sentence stay Korean for every target language and the target language outranks a conflicting language instruction inside a guideline's text (LANG)
  - with no enabled 기본 지침 and no applicable guideline there is no section
  - the voice prefix and the template section are byte-identical with and without guidelines
  - a clip's writing calls carry its 영상 지침 the same way (→CLIP-183)
- GUIDE-16 [o] the product ships 기본 지침 for each kind — code constants, never rows or config: each carries a fixed name and text, is marked (추천), reaches every account with zero setup until its owner switches it off, and is never edited, rewritten, learned or model-written; the 지침 set is →GUIDE-41 and the 영상 지침 set →GUIDE-42 ← a rule about what may be written is the product's recommendation rather than its format, so the owner may turn one off where the writing they want needs its opposite
- GUIDE-17 [o] freezing: StartGeneration, StartRevision and the storyline jobs resolve the post's current `template_id` and 분야 once — the same value the template brief is resolved from — and write the enabled 기본 지침 and the applicable ordered texts into the payload beside `target_length` and the template
  - StartWriteExperiment freezes the same texts into the shared snapshot, so both candidates get byte-identical prompts and a different applicable set is a different input hash
  - handlers read only the payload, so editing, rescoping, deleting or switching after a start changes nothing in flight, across restart-resume and retry
  - payloads without the member decode as no guidelines (→GEN-15)
  - a clip freezes its 영상 지침 with the attempt the same way (→CLIP-69)
- GUIDE-18 [o] nothing is learned, inferred or auto-generated: no model writes, suggests, ranks or retires a guideline or a candidate; no evidence pipeline, similarity dedupe, auto-approval or threshold at which a candidate becomes a rule ← recording user text is not learning about it; no candidate in any state reaches a prompt, post or job — only a saved guideline or an enabled 기본 지침 is injected, and an account with candidates but no new guidelines produces byte-identical prompts (asserted directly); no guideline surface calls a provider or enqueues a job (I5); nothing validates output against a guideline
- GUIDE-28 [o] a guideline outranks a memory exactly as it outranks a template instruction: a fact the memory bank carries is not written when a guideline forbids it, and the precedence sentence beside the `[기억]` section says so (→MEM-21) ← a prohibition the user added is the reason the fact must stay out of this post, and the memory bank has no per-post opt-out of its own
- GUIDE-19 [x] voice scoping, per-post or per-clip selection or opt-out (of a 기본 지침 too), switches on the owner's own guidelines, manual ordering, version history, import/export, seeded owner guidelines (the owner list's empty state shows one worked example as copy), editing a 기본 지침's text, rendering frozen guidelines in experiment or job views, counting affected guidelines in the DeleteTemplate confirmation, manual candidate creation, editing occurrence counts — out of scope
- GUIDE-20 [o] `/guidelines` (nav 지침, after 템플릿, lazily split) lists the 지침 kind in injection order: its 기본 지침 first, each row carrying the name, the text, a (추천) badge and a switch (→GUIDE-43), then the account's guidelines with a scope badge — `전역`, template-name chips, 분야 chips, or `적용 대상 없음` — a read-first text edit, a whole-scope edit, and a delete whose confirmation states that already-enqueued work keeps its frozen text
  - the page carries no standing form — one docked `새 지침` (→THEME-24, the shape its sibling directories use) opens the shared `Sheet` holding a textarea with a live remaining count from `shared/config` and the scope control (a 전역 / 특정 템플릿 / 분야 choice and a checkbox list of the account's templates or of the product's 분야), and the page itself is the list
  - the list query is keyed `(accountId, 'guidelines')` with `staleTime: 0` and `refetchOnMount: 'always'` and is marked stale by a template rename or delete ← chip names are a projection
- GUIDE-21 [o] after a completed revision (`done`, not merely terminal ← a failed revision produced nothing worth saving), `지침으로 저장` sits beside `규칙으로 저장` and opens a dialog seeded with the revision instruction, editable, offering 전역 (default) and the post's current template when it has one (read from the loaded post, no new query)
  - it calls the standard create
  - `AlreadyExists` renders as already-saved information
  - `규칙으로 저장` stays a pre-flight checkbox ← the voice learns from the run itself, a guideline is a plain create that can wait for the result
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
- GUIDE-26 [o] 지침 is the fourth destination of the 글 group (→CLIP-3 →THEME-38) and 영상 지침 the third of the 영상 group (→GUIDE-44), each still clearing the 44 px pointer floor at 320 px
  - `guidelines.sql` stays ASCII-only ← sqlc mis-slices a query file containing any multi-byte character
- GUIDE-27 [o] 전부 수락 runs the standard create once per listed pending candidate with 전역 scope and `from_candidate_id`, keeping every one that passes and leaving each refusal (over 300 characters, a duplicate, the account cap) in the queue with its catalogue reason for a single 승인 to fix ← one over-long candidate must not hold back the rest, and a rule that needs a scope other than 전역 is exactly the one worth opening
  - 전부 거절 dismisses every listed candidate behind one `Dialog` naming the count ← there is no undo and a single 무시 stays dialog-free (→GUIDE-12)
  - neither action is offered while the list is empty and both re-read the candidate list, a create additionally invalidating the saved list
- GUIDE-35 [o] vocabulary carries its own precedence, stated in the prompt: a guideline's substitution beats a template's, a template's beats the voice profile, and 문체 and 종결어미 stay with the voice either way ← otherwise the voice profile's vocabulary would overrule every substitution
- GUIDE-36 [o] only a concrete substitution carries that authority: a guideline or a template may say to write B where the source already says A, while an abstract instruction about better words carries none ← a vague vocabulary instruction cannot be checked against the source, which is what the grounding constraint requires
- GUIDE-37 [o] inside the guideline section the owner's own line outranks a conflicting 기본 지침, and of two lines from the same source the earlier wins ← a recommendation the owner contradicts in their own words has to yield
- GUIDE-41 [o] the 지침 kind's 기본 지침, in this order:
  | 기본 지침 | rule |
  |---|---|
  | 재료에 있는 사실만 | →GEN-16 |
  | 감상은 내가 쓴 것만 | →GEN-16 |
  | 기억을 통한 감상 추가, only in a prompt carrying `[기억]` | →GEN-73 |
  | 메모의 이름으로 | →GEN-44 |
  | 일어난 순서대로 | told in the order it happened, inside each template place when the post has a template |
  | 첫머리에 이유와 기대 | opening with why the owner went or what they expected, when the memo says so |
  | 사진은 이야기의 한 장면 | a photo stands between the sentences about its moment, no paragraph opens as a description of a photo and each paragraph picks up from the last |
  | 끝에서 한 번 정리 | closing by drawing the day together, a verdict or a will to return only as the owner gave one |
  | 관찰을 나열하지 않기 | →GEN-47 |
  | 제목 규칙 | →GEN-49 |
  | 태그 규칙 | →GEN-50 |
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
- GUIDE-43 [o] a 기본 지침's switch belongs to the account and its kind and saves on change: switched off it stays listed, dimmed, and leaves every run started afterwards, while work already enqueued keeps the texts it froze (→GUIDE-17); a new account starts with every 기본 지침 on, and no other surface switches one
- GUIDE-44 [o] `/video-guidelines` (nav 영상 지침, the third destination of the 영상 group after 영상 템플릿 →CLIP-3, lazily split) is `/guidelines` for the 영상 지침 kind:
  - its 기본 지침 first with (추천) and a switch, then the account's 영상 지침 with scope badges — `전역`, video-template-name chips or `적용 대상 없음` — the same read-first edit, scope edit and delete
  - one docked `새 영상 지침` whose Sheet's scope control offers 전역 / 특정 영상 템플릿
  - below the list the `영상 지침 후보` disclosure behaving as GUIDE-22, GUIDE-23 and GUIDE-27 describe, each candidate linking the clip it came from (plain text once that clip is gone)
  - its queries are keyed `(accountId, 'video-guidelines')` and `(accountId, 'video-guideline-candidates')` with `staleTime: 0` and `refetchOnMount: 'always'`, and a video template rename or delete marks the list stale
- GUIDE-45 [o] after a completed clip revision request (→CLIP-131), ②'s composer offers `영상 지침으로 저장`, which opens the standard create seeded with that request, offering 전역 (default) and the project's video template when it has one, as `지침으로 저장` does for a post (→GUIDE-21)

## flow
- author: `/guidelines` | `/video-guidelines` → 새 지침 | 새 영상 지침 → text + scope(전역 | templates | 분야, posts only) → CreateGuideline(bound, cap, dedupe within the kind → row; approves a matching candidate)
- 기본 지침: row → switch(on | off) → saved for the account and kind → the next run's frozen set
- inject: StartGeneration / StartRevision / storyline job(resolve template id and 분야 once → ForPrompt(enabled 기본 지침 + global + template-linked + field-linked) → freeze texts) → `[작문 지침]` after the template section → model; clip attempt(enabled 영상 지침 기본 지침 + global + video-template-linked → freeze) → storyline, flow, narration and revision calls
- accrue: post revise done | clip revision request done → record instruction(kind; dedupe → pending +1 | nothing | queue full) → 지침 후보 | 영상 지침 후보 disclosure(closed, counted) → 승인(create with from_candidate_id) | 무시 | 전부 수락(전역, refusals kept) | 전부 거절

## constraints
- config: `GUIDELINE_TEXT_MAX_CHARS` 300 (BE `platform/config` env · FE `shared/config` `VITE_GUIDELINE_TEXT_MAX_CHARS`, falling back to the default) · `GUIDELINE_MAX_PER_ACCOUNT` 100 · `GUIDELINE_CANDIDATE_MAX_PENDING` 50, each per kind (BE only; malformed values are boot-fatal); the heading, precedence sentence, every 기본 지침's name, text and order, injection ordering, candidate statuses and review order, the candidate storage bound (the revision instruction bound) and the proto/SQL schema are code
- schema: `guidelines(id, user_id, kind, text, scope, created_at, updated_at, UNIQUE(user_id, kind, text), UNIQUE(id, user_id))` · `guideline_templates` (composite FKs to guideline and a template of its kind) · `guideline_fields` (guideline ↔ 분야) · `guideline_candidates(id, user_id, kind, text, status, occurrences, post_slug NULL, clip_id NULL, first/last_seen_at, UNIQUE(user_id, kind, text))` · the account's switched-off 기본 지침 by kind and name; migrations 0014, 0023, 0078
- placement BE: `backend/internal/guideline` (`ForPrompt` resolving by template and 분야, candidates, store, rpc; consumed by generation and post through consumer-declared ports; template names through the directory port, never a SQL join)
- placement FE: `entities/guideline` (model, api, ui scope control) · `features/create-guideline` `edit-guideline` `delete-guideline` `review-guideline-candidate` · `features/edit-with-ai` (the capture) · `pages/guidelines` · `pages/video-guidelines`
- contract: `proto/postpilot/v1/guideline.proto`

## chg
- r10 260928 GUIDE-41✎ +기억을 통한 감상 추가 (→GEN-73) after 감상은 내가 쓴 것만, whose gist→→GEN-16
