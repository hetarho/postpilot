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

- **Nothing is learned, inferred or auto-generated.** No model writes, suggests, ranks or retires a guideline; there
  is no evidence pipeline and no candidate lifecycle. [I4]'s learning boundary stays entirely with voice.
- No guideline surface calls a provider or enqueues a job ([I5]) — not the list, not a create, edit or delete, not
  the revision capture, not the page mount.
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
- Server refusal messages are shown verbatim through the shared failure catalogue; the client predicts none of them,
  and the per-account cap is deliberately not mirrored client-side.

## Config

| Value | Owner | Default |
|---|---|---|
| `GUIDELINE_TEXT_MAX_CHARS` | BE `platform/config` (typed, env) · FE `shared/config` (`VITE_GUIDELINE_TEXT_MAX_CHARS`) | 300 |
| `GUIDELINE_MAX_PER_ACCOUNT` | BE `platform/config` only | 100 |

Both are counted in Unicode scalar values. A non-positive or malformed backend value is boot-fatal; a malformed
frontend override falls back to the default rather than losing the counter. The section heading, the precedence
sentence, the grounding constraint, the injection ordering and the proto/SQL schema are code, not config.
