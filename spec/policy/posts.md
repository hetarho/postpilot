# Policy — Posts and drafts

Canonical rules that are **currently true** in the code. Source: [plan/02](../plan/02.post-drafting-and-list.md),
backend built by job 03. A change to any rule here is a change to shipped behavior — go through `/create-change`.

Photo upload has its own document: [uploads.md](uploads.md).

## Identity

- A post is identified by its **slug**, minted once on the first save and **never changed**. It is the primary key
  *and* part of every object key for the post's photos, so renaming it would orphan the photos.
- Shape: `YYYYMMDD-<sanitized title>`, with a serial suffix `-2`, `-3`, … when that name is taken (PRD §7).
  An empty or fully-stripped title gives `YYYYMMDD-untitled` — never a bare date, which would collide with every
  other untitled post that day.
- Sanitizing: trimmed, lowercased, runs of whitespace and separators collapsed to one `-`, path- and URL-unsafe
  characters dropped (`/ \ ? # % : * " ' < > | & + .` and control characters), truncated to 60 runes, never ending in
  a separator. **Korean and any other letter is kept** — a slug is not required to be ASCII, and stripping Hangul
  would make almost every real title "untitled".
- Slugs are globally unique, not per user: two accounts cannot hold the same slug.
- The mint is a check-then-insert, so the **insert decides**. A slug taken between the two makes the create retry
  with the next serial rather than fail.

## Ownership

- Every read and write is scoped to the acting user, taken from the session by the auth interceptor. No request
  carries a user id.
- A slug that exists but belongs to someone else is **`PermissionDenied` (403)**, not a 404 (PRD §7). At two users
  there is nothing to enumerate.
- A slug that does not exist is `NotFound` (404).
- `ListPosts` returns only the caller's posts, newest first by `updated_at`.

## Drafts

- `SavePostDraft` is create-or-update: an empty slug creates and returns the minted slug; a slug updates `title`,
  `memo` and `updated_at`. It is the autosave endpoint, called about once a second while someone types, so repeated
  saves are plain idempotent updates.
- **There is no save button.** The editor saves 1 s after the last keystroke, and again on every way out of it
  (the tab being hidden or unloaded, and leaving the editor). A save that fails retries with a capped backoff.
- A failed save keeps retrying after the user leaves the editor, but **never after the session ends** — a retry
  landing under the next account's cookie would file one person's draft in someone else's account. The mechanism
  and its full rule list are in [spec/tech/draft-autosave.md](../tech/draft-autosave.md).
- A response that carries no post is not a confirmation, and the client treats it as a failed save. This matters
  most for the create: trusting it would leave the next edit minting a second post for the same draft.
- `status` is `draft` here and only here. Generation (plan 06) is what moves a post to `review`.
- `observations` and `content` belong to the generation pipeline. This context never reads or writes them.
- **There is no post deletion.** The PRD defines photo deletion but not post deletion; flagged as a PRD gap, not an
  oversight.

## Storage of time

- Timestamps are stored as **fixed-width RFC3339 in UTC**. The width matters: `ORDER BY updated_at` and
  `expires_at < ?` are plain string comparisons in SQL, and a trimmed fraction (`…08.5Z`) sorts after a longer one
  (`…08.513110616Z`).
- On the wire, timestamps are RFC3339 strings — the client renders one without needing to know a unit.
