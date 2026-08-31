# Policy — Post purposes (용도)

Canonical backend and frontend rules. Source: [plan/11](../plan/11.post-purpose-presets.md), built by job 22.

A **purpose** is a reusable, account-owned brief: what a kind of post is for and how that kind must be written. The
voice decides how sentences sound; the purpose decides genre, structure and required content. The two are orthogonal
— a post combines exactly one voice and at most one purpose.

## Ownership and persistence

- An account owns zero or more purposes (`purposes`), the user-facing noun 용도. A purpose has exactly three authored
  fields — `name`, `description`, `instructions` — plus its timestamps. Nothing else belongs to it.
- Every store query and RPC is scoped by the authenticated user. No request carries a user id. A foreign purpose id is
  indistinguishable from an unknown one (`NotFound`).
- `purposes_id_user` is the composite target `posts.purpose_id` points at, so a post can never name another account's
  purpose even if a service check is bypassed. `purposes_user_name` keeps display names unique within an account.
- There is no tombstone: a deleted purpose is gone and its name is free again immediately. This is the deliberate
  difference from a voice, which keeps its name so the posts written in it stay readable — a purpose is not something
  a post was written *in*, only something the next machine run is told.
- Migration `0011_post_purposes.sql` creates the table, adds nullable `posts.purpose_id` with the composite account
  foreign key, and adds `model_experiments.purpose_name`. Existing posts get `NULL`; there is no backfill and no seeded
  purpose. Timestamps follow the fixed-width UTC RFC3339 rule.

## Field rules

| Field | Rule |
|---|---|
| `name` | trimmed, non-empty, ≤ `PURPOSE_NAME_MAX_CHARS`, unique among the account's purposes after trim |
| `description` | trimmed, **may be empty**, ≤ `PURPOSE_DESCRIPTION_MAX_CHARS` |
| `instructions` | trimmed, non-empty, ≤ `PURPOSE_INSTRUCTIONS_MAX_CHARS` |

Limits are Unicode scalar-value counts on both sides, so a Hangul syllable is one character to the server and to the
counter. The backend is authoritative; the frontend mirrors the values only for live counters and never decides a save.

## Editing

- `UpdatePurpose` is presence-based, like `UpdateVoiceProfile`: only the fields the request carries change. The store
  runs **one statement per present field** inside one transaction, so a field the request omitted is never named by any
  statement — two fields edited from two tabs cannot overwrite each other, and no read-modify-write can restore a stale
  value. A present empty `description` clears it; a present empty `name` or `instructions` is refused.
- A duplicate name is `AlreadyExists` with reason `PURPOSE_NAME_TAKEN`; a bound violation is `InvalidArgument` with
  `PURPOSE_FIELD_TOO_LONG` and allowlisted `field`, `max`, and `actual` params. The frontend renders these stable
  details in the active locale and never treats raw wire prose as translation input.

## Deletion detaches, never cascades

- `DeletePurpose` removes the purpose and returns `detached_posts`: how many posts lost their assignment. The detach is
  the schema's `purposes_detach_posts_on_delete` trigger, which runs inside the delete's own transaction, so the count
  and the clearing see one snapshot of `posts`.
- It is a trigger rather than `ON DELETE SET NULL` because SQLite sets **every** column of a composite child key to
  NULL, which would try to null `posts.user_id` and fail its NOT NULL constraint. The composite foreign key stays for
  the account guarantee it provides.
- No post, no content, no photo and no history is deleted. Frozen job payloads and experiment snapshots keep the
  brief's text, so work already done stays explainable.

## Assignment to a post

- `SavePostDraftRequest.purpose_id` is presence-aware with three meanings: **absent** preserves the current assignment
  (what ordinary autosave sends), **present and empty** clears it (없음), **present and non-empty** assigns it.
- The purpose is validated **before anything else in the request is applied**, so a request naming an unknown or
  foreign purpose leaves the post exactly as it was — title and memo included — and mints no post on a create.
- Assignment is allowed in **every** status, including `finalized`, and while a job is running. It touches no content,
  revision, machine baseline, finalization field or learn eligibility. This is the deliberate difference from a voice
  reassignment, which withdraws the machine baseline: nothing is ever learned from a purpose, so assigning one costs
  the post nothing.
- `Post` and `PostSummary` carry a transport-only `PurposeRef {id, name}`, unset when the post has none. The post
  context never reads the `purposes` table; the name comes from the purpose context's published directory through a
  consumer-owned `PurposeDirectory` port.

## Prompts, freezing, and what is never learned

- The frozen brief enters the write and revise prompts at one fixed position: after the **complete** voice profile
  (styleguide → active rules → excerpts → user rules → ending constraint) and before the per-post material, as
  `[글의 용도: {name}]`, the `이 글의 용도` line when the description is non-empty, `작성 지침:` + the instructions
  verbatim, then a fixed precedence sentence. Absent a purpose the builder adds no purpose bytes and remains
  **byte-identical** to the current fixed no-purpose golden under `internal/generation/testdata`.
- The position is load-bearing twice: the voice-profile prefix stays byte-stable across posts of different purposes
  (PRD §5's caching note), and the brief stays in the stable half, so every revision of one post re-injects the
  identical block.
- The brief is resolved **once, at enqueue**, and written into the job payload beside `target_length`; handlers read it
  from the payload and never from the live row. Editing or deleting the purpose after `StartGeneration` /
  `StartRevision` returns — including across a restart-resume or an explicit retry — cannot change the prompt. A
  purpose deleted between the save and the start is simply absent: a post with no purpose, not a failed start.
- A write A/B experiment freezes the brief into the shared prepared snapshot, so both candidates receive byte-identical
  system prompts and a different purpose is a different input hash. `ModelExperiment.purpose_name` records it by name,
  stored rather than derived, so the comparison detail keeps reading correctly after the purpose is renamed or deleted
  and after the retention sweep clears the snapshot.
- **Nothing about a purpose is learned, inferred, or written by a model** ([I4] stays entirely with voice). No purpose
  is selected from a title, memo, tags, photos or content; there is no default and no seeded library. The observe stage
  never sees a brief — observation stays facts-only about photos ([I3]).
- No purpose surface makes a provider call or enqueues a job ([I5]): listing, selecting, creating, editing and deleting
  are plain CRUD round trips.

## Frontend

- `/purposes` is a top-level destination (용도, after 말투). It lists the account's purposes with their post counts,
  edits each field read-first and one at a time, and deletes behind a confirmation that states the detach count. The
  empty state shows one worked example **as copy** and creates no row.
- The directory query is re-read on every mount (`staleTime: 0`, `refetchOnMount: 'always'`). `post_count` is a
  projection over *posts*, so assigning a purpose in the editor changes it without touching any purpose and no purpose
  mutation can invalidate it — and it is the number the user confirms a destructive detach against.
- A rename invalidates the post caches as well as the directory: the name is projected onto every assigned post and
  rendered on the list, so those rows change what they display without changing at all.
- The editor and `/posts/new` show a `용도` select defaulting to 없음, beside the voice select, with the selected
  purpose's description under it and a link to `/purposes`. Selecting writes a presence-aware patch through the
  per-post draft queue, so a delayed title save cannot revert a newer selection. On a create, 없음 sends **no** field at
  all rather than an empty one.
- The select stays enabled while a job runs and says that the running job keeps the purpose it started with. A failed
  directory read is shown as a failure with a retry — never as an empty directory, which would leave clearing as the
  only available action.
- The post list shows the purpose name beside the voice for an assigned post and nothing for an unassigned one.

## Configuration

| Value | Owner | Default |
|---|---|---|
| `PURPOSE_NAME_MAX_CHARS` | BE `platform/config` · FE `shared/config` (`VITE_PURPOSE_NAME_MAX_CHARS`) | `40` |
| `PURPOSE_DESCRIPTION_MAX_CHARS` | same pair | `200` |
| `PURPOSE_INSTRUCTIONS_MAX_CHARS` | same pair | `2000` |

A non-positive or malformed backend value is boot-fatal. The frontend falls back to the default rather than disabling
its counter, so a build-time typo cannot silently remove the client-side bound. Section headings, the precedence
sentence, and the proto/SQL schema are code, not configuration (ARCHITECTURE §4).
