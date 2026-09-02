# Tech — Draft autosave

Why the frontend's autosave is a per-post queue outside React, and the rules that keep it
honest. Landed by [job 04](../jobs/archive/04.post-list-and-editor-autosave.md) for
[plan 02](../plan/02.post-drafting-and-list.md). Product basis: PRD F-2 — "폰에서 앱을
닫아도 안 날아간다".

Owner in code: `frontend/src/features/save-draft/model/draft-queue.ts` (the queue) and
`model/useAutosave.ts` (its React end).

## The shape

```
editor (React)              draft-queue (module state, one entry per post)
  text  ──queue()──▶        pending ──debounce──▶ send ──▶ saved
  state ◀──publish──        failed  ──backoff───▶ retry
```

The queue is **not** owned by the component. The editor attaches to it, pushes text into
it, and releases it; the queue keeps going.

## Why it is not inside the hook

Three things went wrong when the save state lived in the component, and all three are
invisible to the user at the moment they happen:

- **A failing save died with its editor.** The flush that carries the last keystrokes out
  of the editor is the one save that most needs a retry, and it was the only one that
  could never get one.
- **The `/posts/new` → `/posts/<slug>` mint swaps routes**, so the editor that started the
  save is not the one that has to finish it. Handing the text across as a snapshot made
  the next editor believe the server already held text it might still be sending.
- **Two editors for one post could each run their own chain of saves**, and the loser was
  decided by which response happened to land last.

One queue per post makes the guarantee statable: *for a given post there is at most one
save in flight, and what it sends is always the newest text queued — and the newest voice
assignment chosen.*

## Rules

- **A queue outlives its editor, never its session.** `discardDraftQueues()` is called on
  logout (`app/routes/AuthenticatedLayout.tsx`) and on a session that died mid-use
  (`app/providers/auth-redirect.ts`). A retry firing after someone else signs in on the
  same device would send the previous account's text under the new account's cookie, and
  the server would file it there.
- **The one exception is an intentional delete.** `discardDraftQueue(slug)` and
  `discardContentQueue(slug)` end the queues for that slug alone, and the editor calls both on a
  successful `DeletePost` before it navigates away. Without them a failing save would keep
  retrying against a slug the server no longer has, and the user would be shown a save failure
  for a post they destroyed on purpose. Their waiters reject with `post deleted` rather than
  `session ended`, so a stray rejection says which discard cancelled it. Every other slug's queue
  is untouched, and the session-wide `discardDraftQueues()` / `discardContentQueues()` keep their
  signatures and their call sites. Like the session-wide discards, it stops the timers and the
  waiters but cannot recall a request already in flight; the queue is marked discarded, so that
  response changes no local state.
- **A draft with no slug yet gets a key of its own** (`new:<n>`), not a shared one. Two
  "새 글" editors are two different drafts. The key becomes the slug when the first save
  mints one.
- **Compare against the request in flight, not against the last saved text.** A request
  cannot be recalled, so an undo made while one is out has to be sent as its own save.
- **A 200 with no post is not a confirmation.** Trusting it would mark text saved, and for
  a draft with no slug yet would leave the next edit creating a second post.
- **Typing during an outage never restarts the backoff.** A keystroke replaces what the
  scheduled retry will send; it does not schedule a request of its own.
- **The text pushes through a layout effect, not a passive one.** A passive effect runs
  after paint and can be deferred past a `pagehide`, and the keystroke it had not recorded
  yet is exactly the one that would be lost.
- **The voice assignment rides in the queue, not beside it** (`assignVoice`, job 18). A
  create always sends `voice_id` — a post cannot exist without one (spec/policy/posts.md) —
  and an ordinary edit never does, so a delayed title save cannot carry a stale voice over a
  newer choice. A choice made for a draft with no post yet is simply remembered; one made
  while the create was already out is followed by an immediate reassignment the moment the
  create lands.
- **A reassignment is an action, not a keystroke.** It is sent at once, without the debounce,
  and its promise reports that one save's outcome. Text typed while it is out waits and goes
  afterwards without the voice. A refused reassignment is *taken back*: the refusal is usually
  an answer (a busy post, a voice deleted meanwhile), and retrying it with every save would
  keep the title from ever landing again — so the retries that follow carry text only, and a
  queue with nothing else to send goes quiet rather than reporting a failure forever.

## What is still best-effort

The flush on `visibilitychange`(hidden) and `pagehide` is a normal request; a browser may
cut it short as it discards the document. `fetch(keepalive)` is not reachable through the
Connect transport, and `sendBeacon` cannot carry the JSON content type across origins
without a preflight it is not allowed to make. The bound on the loss is therefore the
debounce window — one second (`AUTOSAVE_DEBOUNCE_MS`) — which is why it is short.

## Generated-content queue

Editable `PostContent` uses a second per-post queue rather than sharing title/memo state. Its snapshot contains only
the complete protobuf block value; optional generation settings use their own endpoint and never share its
`content_revision`.

- one request is in flight; later edits replace one pending snapshot, so the newest value wins;
- each response supplies the next expected revision; an `Aborted` conflict stops retry and asks for reload;
- transport failures use bounded exponential backoff; logout discards timers, snapshots, and flush waiters;
- page hide/release calls `saveNow()` best-effort, while AI revision/finalization await `flush()`, whose promise
  returns the exact saved revision used by `FinalizePost`;
- manual saves never touch `machine_baseline`; machine winner/revision writes do not use this queue.
