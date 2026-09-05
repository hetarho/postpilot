# Policy — Writing guidelines (작문 지침)

Canonical backend and frontend rules. Source: [plan/16](../plan/16.writing-guidelines.md), built by job 35.

A **guideline** is a reusable, account-owned rule about what a post must avoid or watch out for. It is the third
authored layer beside the two that existed: the voice decides how sentences sound
([policy/voice](voice.md)), the template decides genre and required content ([policy/templates](templates.md)), and a
guideline is a prohibition or a caution that outranks the template on content while leaving register to the voice.

## Ownership and persistence

- An account owns zero or more guidelines (`guidelines`), the user-facing noun 지침. A guideline has exactly one
  authored field — `text` — plus a scope and its timestamps. Nothing else belongs to it.
- Every store query and RPC is scoped by the authenticated user. No request carries a user id. A foreign guideline id
  is indistinguishable from an unknown one (`NotFound`), and so is a foreign template id named in a scope.
- **Nothing references a guideline.** Posts, jobs and experiments carry frozen *texts*, never ids, so deleting one
  detaches nothing and leaves every enqueued run readable.
- Migration `0014_writing_guidelines.sql` creates `guidelines` and `guideline_templates`. It seeds no rows and
  backfills nothing: existing accounts start with zero guidelines, which is what keeps every prompt built before it
  byte-identical to one built after it, apart from the grounding line below.
- `guidelines(user_id, text)` is unique, so an exact duplicate after trim is refused by the database rather than by a
  service check that two concurrent creates could both pass. `guidelines(id, user_id)` is the composite target
  `guideline_templates` points at, so a link can never cross accounts even if a service check is bypassed.
- Migration `0023_guideline_candidates.sql` adds `guideline_candidates`, also seeding and backfilling nothing:
  there is no revision history to reconstruct, so an existing account starts with an empty 후보 list.
  `guideline_candidates(user_id, text)` is unique for the same reason `guidelines(user_id, text)` is, and
  `post_slug` deliberately carries **no** foreign key — see the candidate section below.

## Field rules

| Field | Rule |
|---|---|
| `text` | trimmed, non-empty, at most `GUIDELINE_TEXT_MAX_CHARS` Unicode scalar values (default 300), unique within the account after trim (`AlreadyExists`) |
| `scope` | `global` (must carry no template ids) or `templates` (must carry ≥ 1 distinct owned template id; an unknown or foreign id is `NotFound` and nothing is applied; duplicates in one request collapse to one link) |
| account cap | creating beyond `GUIDELINE_MAX_PER_ACCOUNT` (default 100) is refused with `FailedPrecondition` naming the cap. The cap is checked inside the insert transaction, so two concurrent creates cannot both pass it |

An unset scope on the wire is refused, never defaulted: the one shape that must not be guessed is the one that would
apply a rule to every post of the account.

`UpdateGuideline` uses presence. An update carrying only `text` leaves the scope untouched; a scope patch replaces the
**whole** scope — kind and link set together — atomically in one transaction, because a scope is only meaningful as
both halves at once.

## Candidates

A **candidate** (후보) is one completed revision's instruction, recorded so a correction accrues instead of
vanishing with the tab. It is a receipt for something the user wrote — never a model's opinion about it.

- **Recording** happens inside the `revise` job's own completion path, after the revised content is persisted. It
  creates no job and calls no provider ([I5]). A failed, cancelled or still-running revision records nothing, and
  **a recording failure never fails the revision**: the result the user is looking at is authoritative, and a
  bookkeeping error must not discard it.
- The text is stored **verbatim** — trimmed at the edges and otherwise untouched. Nothing rewrites, summarizes,
  normalizes, generalizes, translates, clusters, scores or ranks it.
- A candidate carries **no scope at all**. Scope is a durable decision about every future post, so it is made at
  approval time; that absence is what makes automatic recording safe.
- **The bound split:** a candidate is stored at the *revision* instruction bound (500 characters), not the guideline
  bound (300). Refusing a long instruction at recording time would lose exactly the most specific corrections, so
  the guideline bound is enforced at approval, where the user shortens it with the live counter. A candidate is
  never silently truncated.

### Statuses and deduplication

`pending` · `approved` · `dismissed`. All three rows are **kept**, because the row itself is what suppresses
re-recording. Deduplication is **exact after trim** and nothing else — the same rule the guideline unique index
uses. There is no similarity, fuzzy or semantic matching.

| Situation | Outcome |
|---|---|
| First sighting, queue has room | a `pending` row with `occurrences = 1` |
| Repeat of a `pending` candidate | `occurrences + 1`, `last_seen_at` advances, no new row. `post_slug` is **not** rewritten — the candidate names where the correction was first seen |
| The text is already a saved guideline | nothing recorded |
| The text is an `approved` or `dismissed` candidate | nothing recorded, nothing revived |
| `GUIDELINE_CANDIDATE_MAX_PENDING` pending rows | nothing recorded — the queue **stops** rather than evicting, because evicting the oldest would discard something the user might have approved |

The dedupe read, the guideline check and the pending count all run inside **one transaction**, so two concurrent
recordings can neither both insert the same text nor both pass a full queue.

### Approval and dismissal

- **Approval is `CreateGuideline`.** There is no Approve procedure: the create already owns every field rule, the
  text bound, the account cap and every refusal an approval needs. It gained an optional `from_candidate_id`.
- The create marks the candidate approved **in the same transaction as the insert** — by `from_candidate_id` when
  present, and by the saved text either way. The by-text half is what marks the candidate an on-the-spot
  `지침으로 저장` recorded, without the client having to learn its id.
- A guideline **text edit** runs the same approval, so no way of saving a text can leave that text sitting as a
  pending candidate.
- Only a `pending` candidate may be moved. A terminal one reads as `NotFound` — which rolls the create back — so a
  stale tab can neither approve one candidate twice nor dismiss one already approved. A named
  `from_candidate_id` that matches no pending candidate of the account is `GUIDELINE_CANDIDATE_NOT_FOUND`.
- A refused create (text bound, account cap, duplicate text) approves nothing: the candidate stays pending.
- **무시** marks the row `dismissed`. Nothing is deleted, there is no confirmation dialog, and there is no undo
  beyond writing the guideline by hand — the same instruction from a later revision does not reappear.

### Lifecycle

- Deleting the source post drops the candidate's `post_slug` and keeps its text, so the row is listed without a
  link. The detach runs **after** the post row is actually gone, so a delete that fails partway never leaves a
  candidate claiming its post is deleted; a detach failure is logged and does not fail the delete. `post_slug` is a
  plain nullable column with no foreign key: an FK with `SET NULL` would be the only way to keep the text, and that
  is more machinery than a column the reader treats as optional.
- Deleting a saved guideline **neither revives nor resets** its approved candidate. The approved row keeps
  suppressing re-recording; re-creating the rule is an explicit create.

## Scope resolution and injection

- A post with template P receives the account's `global` guidelines plus those linked to P. A post with no template
  receives the global ones only. A `templates` guideline whose every template was deleted (**적용 대상 없음**) reaches
  no prompt at all until it is rescoped.
- Injection order is the global group first, then the scoped group, each by `created_at, id` ascending. The
  management screen lists them in exactly that order, so what the user sees is what the writer is given.
- The write and revise prompts render **one** `[작문 지침]` section at **one** position: after the `[글 템플릿]`
  section when the post has a template, otherwise directly after the voice profile's `[종결어미 제약]`, and always
  before `[이번 글]`. The texts are hyphen-bulleted lines, verbatim, closed by a fixed precedence sentence.
- The section heading and the precedence sentence stay Korean for every target language, as the template section's do:
  they frame user-authored text and are not part of the output-language contract. Plan 13's rule stands — the target
  language outranks a conflicting language instruction inside a guideline's text.
- With no applicable guidelines there is no section at all, and the prompt is byte-identical to the baseline. The
  voice profile prefix and the template section are byte-identical with and without guidelines, so the cache-stable
  prefix of PRD §5 and plan 11's acceptance criteria both keep holding.

## The built-in grounding constraint

The fixed `WritePrompt` / `RevisePrompt` text — and their English forms — carries one universal constraint: state no
concrete fact the memo and the photo observations do not carry, and invent no interaction, facility, service,
conversation or price.

Each pass then adds the clause its own material can support, and only that one. The **write** pass holds the memo and
the observations, so it is also told to omit or confine to the observed range whatever it cannot confirm. The
**revise** pass receives neither, so the same instruction there would license stripping real facts out of blocks the
request never mentioned, against the byte-for-byte preservation rule beside it; it is told instead to apply the
constraint only to sentences the request makes it write or touch, and to leave everything outside the request alone
without re-checking its facts.

All three strings are Go constants, not configuration and not database rows, and they apply with zero setup on every
account. The constraint is
deliberately disjoint from change 10's Korean naturalness baseline, which owns style and nothing else, and the observe
prompt never sees it ([I3]). Its arrival regenerated the prompt goldens exactly once; every other byte-identity rule
is stated relative to that new baseline.

## Freezing

- `StartGeneration` and `StartRevision` resolve the post's **current** `template_id` once — the same value the template
  brief is resolved from, so both come from one consistent view — and write the applicable ordered texts into the job
  payload beside `target_length` and `template`.
- `StartWriteExperiment` freezes the same texts into the shared prepared snapshot. Both candidates therefore receive
  byte-identical system prompts, and because the texts are part of the snapshot, a different applicable set produces
  a different input hash: a different rule set is a different experiment.
- Handlers read only the payload or the snapshot. Editing, rescoping or deleting a guideline after a start returns
  changes nothing in flight, including across a restart-resume and an explicit retry.
- Payloads written before this feature decode as "no guidelines" rather than failing.

## What a guideline is not

- **Nothing is learned, inferred or auto-generated.** No model writes, suggests, ranks or retires a guideline **or a
  candidate**; there is no evidence pipeline, no similarity or semantic dedupe, no auto-approval, and no threshold
  at which a candidate becomes a rule by itself. [I4]'s learning boundary stays entirely with voice.

  **Recording user text is not learning about it.** That is the line the candidate lifecycle sits on: a candidate is
  the user's own sentence, copied verbatim, deduplicated by exact string equality, counted, and inert until the user
  approves it with a scope they chose. No model reads or writes one anywhere in the flow.
- **No candidate is ever applied.** A candidate in any state — `pending`, `approved` or `dismissed` — reaches no
  prompt, no post and no job. Only a saved guideline is injected. An account with candidates but no new guidelines
  produces prompts byte-identical to the same account before candidates existed.
- No guideline surface calls a provider or enqueues a job ([I5]) — not the list, not a create, edit or delete, not
  the revision capture, not the page mount, and not the candidate list, approval or dismissal. Recording rides the
  revision job that is already running.
- There is no revision history. A candidate is one recorded instruction, not a log of what a revision did, and
  there is no manual candidate creation, no editing of an occurrence count, no bulk approve and no import/export.
- There are no seeded or shipped guidelines. The empty state shows one worked example as **copy**, never as a row.
- There is no per-post selection, opt-out, enable/disable toggle, manual ordering, version history or import/export.
  Applicability derives from scope alone, and delete is the off switch.
- Nothing validates output against a guideline: guidelines are prompt material, and plan 06's block validator is
  unchanged.

## Frontend

- `/guidelines` (nav 지침, after 템플릿, lazily split like its siblings) lists the account's guidelines in injection
  order with a scope badge — `전역`, the template-name chips, or `적용 대상 없음` — a read-first text edit, a
  whole-scope edit, and a delete whose confirmation states that already-enqueued work keeps its frozen text and
  nothing else is affected.
- The create form is a textarea with a live remaining-character count from `shared/config` plus the scope control:
  a 전역 / 특정 템플릿 switch and, for the second, a checkbox list of the account's templates.
- Template names on every chip are a projection, so the list query is keyed `(accountId, 'guidelines')` with
  `staleTime: 0` and `refetchOnMount: 'always'`. Renaming or deleting a template additionally marks it stale.
- After a **completed** revision, `지침으로 저장` sits beside `규칙으로 저장` and opens a dialog seeded with the
  revision instruction, editable before saving, offering 전역 (default) and the post's current template when it has one
  (read from the already-loaded post; no new query). It calls the standard create RPC. `AlreadyExists` renders as
  already-saved information, not a failure. `규칙으로 저장` stays a pre-flight checkbox because the voice learns from
  the run itself; a guideline is a plain create, so it can wait for the result.
- A **후보** section sits **above** the saved list — it is what has changed since the user last looked, and the saved
  list is reference. Rows appear in the server's review order (occurrences descending, then last-seen descending),
  each showing its text, its occurrence count when above one, and its source post as a link (plain text when the
  post is gone). The section renders nothing when nothing is waiting and the queue has room; a failed candidate read
  renders nothing either, because the saved list already owns the page's error state.
- When the pending queue is full the section **says so**. That is the one thing an empty result cannot tell the
  user, and the client owns no copy of the bound — the server returns `queue_full` beside the list.
- **승인** opens the entity's own scope control (전역 preselected) over an editable text with the live 300-character
  count, and calls the standard create with `from_candidate_id`. A duplicate-text refusal keeps the dialog open,
  says the rule already exists, and re-reads the candidate list — the usual cause is another tab having saved it,
  which also approved this row. **무시** has no dialog.
- The candidate list is keyed `(accountId, 'guideline-candidates')` with `staleTime: 0` and
  `refetchOnMount: 'always'`, because a candidate arrives from a revision the user ran in another tab. A create
  invalidates **both** lists, since an approval moves a row from one to the other; a dismissal invalidates only the
  candidate list.
- Server refusal messages are shown verbatim through the shared failure catalogue; the client predicts none of them,
  and neither the per-account cap nor the pending-candidate bound is mirrored client-side.

## Config

| Value | Owner | Default |
|---|---|---|
| `GUIDELINE_TEXT_MAX_CHARS` | BE `platform/config` (typed, env) · FE `shared/config` (`VITE_GUIDELINE_TEXT_MAX_CHARS`) | 300 |
| `GUIDELINE_MAX_PER_ACCOUNT` | BE `platform/config` only | 100 |
| `GUIDELINE_CANDIDATE_MAX_PENDING` | BE `platform/config` only | 50 |

Both are counted in Unicode scalar values. A non-positive or malformed backend value is boot-fatal; a malformed
frontend override falls back to the default rather than losing the counter. The section heading, the precedence
sentence, the grounding constraint, the injection ordering, the candidate statuses, the candidate review ordering,
the candidate storage bound (the revision instruction bound, which must not be able to drift from it) and the
proto/SQL schema are code, not config.
