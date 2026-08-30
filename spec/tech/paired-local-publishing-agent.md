# Paired local publishing agent

> Technical decision under implementation for [plan 12](../plan/12.automated-naver-publishing-through-a-pai.md).
> Job 25's server/frontend/companion paths are in offline verification; an explicitly authorized live Naver smoke
> remains the release gate.

## Why the executor is local

Naver Blog has no public writing API. The browser that can publish therefore needs a logged-in Naver session and
must survive editor changes and interactive account challenges. Keeping that browser on the owner's Mac gives it the
owner's familiar network and a visible recovery surface while keeping Naver credentials out of the VPS.

The Mac is not made into a public server. A companion daemon dials the postpilot API, claims a durable job and invokes
Hermes locally. Polling is merely delivery; SQLite is the source of truth. Sleep, network changes and process restarts
therefore delay a job instead of losing it, and the Mac exposes no inbound port, tunnel, CDP endpoint or webhook.

## Two credentials, two trust domains

The normal postpilot HttpOnly cookie proves a human browser session. A publishing-agent bearer token proves one
paired local capability. They are deliberately not interchangeable:

- a human session creates/revokes pairings and starts/cancels publication;
- an agent token can sync only its safe profile metadata, claim only its owner's jobs and report only a currently
  leased job;
- the single-use pairing code carries 48 random bits, is short-lived and is persisted only as a hash;
- the raw agent token is returned once, stored in macOS Keychain and hashed on the server; and
- Naver credentials, cookies, browser profile paths, CDP endpoints and Hermes provider credentials remain local.

Each postpilot account is paired separately. Two connections on one Mac use isolated browser user-data directories
and Hermes profiles, so neither profile lock, cookie jar nor agent memory is shared. The running daemon reloads its
owner-only configuration every two seconds and starts a newly armed account without a reboot or manual restart.

Setup first lets a constrained Hermes pairing skill follow the signed-in Naver UI to the account's own blog and open
its writer in the same visible dedicated page. Deterministic local CDP then observes the blog id selected by Naver, a
versioned editor-specific root, and conflict-free categories before enrollment completes. It rechecks that the same
page is still the sole CDP target; Hermes/model output is not identity evidence. Its per-run cryptographic nonce,
exact loopback Host/Origin checks, and bounded action/form parser protect the local setup mutation surface. It writes the Keychain token
reference and a complete but unarmed local connection before `SyncAgentProfile` may expose the server record as ready;
an interrupted/lost response is resumed with that pending connection instead of consuming another pairing code.

## Immutable payload, mutable delivery

Publication has an external side effect, so the queued payload cannot follow live post state. The server freezes the
exact finalized protojson and settings and copies the referenced, already-converted JPEG objects to job-owned R2
keys. A claim may mint new expiring read URLs repeatedly, but every claim points to the same immutable bytes and
manifest.

This is not a second content canon. The stored manifest is a snapshot of the canonical block array, and terminal
jobs purge it. It is also not new image processing: R2 copies the JPEG objects; the API never receives or decodes
their bytes. Signed claim URLs are used only by the downloader and are stripped from the local manifest before Hermes
reads it. Publication metadata survives source-post deletion because the history row stores the slug snapshot without
a post foreign key. The row also stores the source post's immutable creation timestamp, so a later post that reuses a
deleted slug cannot inherit that history. Repeated references to one canonical image receive distinct ordinal local
filenames while retaining the source filename used by the unchanged content snapshot.

Before storage staging, the server durably reserves the random job ID/prefix. A random-source error or collision stops
before any R2 copy, and a successful job's reservation is retained so a future collision cannot overwrite historical
assets. An unused reservation is released only after complete staging compensation.

## At-most-once after the commit fence

Browser publication cannot offer transactional exactly-once semantics. A process can die after the remote site
commits but before postpilot receives the URL. The design makes that uncertainty explicit instead of pretending a
retry is safe:

1. work before the final click is lease-retryable;
2. the Hermes publisher plugin reports `committing` immediately before that click;
3. any loss after the persisted fence becomes `outcome_unknown`; and
4. only a readback URL for the expected blog that is still open in the paired browser makes the job `published`.

An unknown outcome is never auto-retried. This is at-most-once behavior at the point where duplication matters,
with an honest ambiguous state when the remote platform cannot prove the answer.
The API checks expired leases every half lease TTL; this recovery cadence is deliberately separate from the slower
R2 orphan-object sweep. SQL progress and failure mutations compare the current stage/sequence and classify the result
from the durable commit timestamp atomically, closing delayed-report races at the fence.
A correlated active-agent check is part of every leased mutation, so revocation fences renewal, progress, completion,
and failure even when an RPC authenticated just before the revoke.
A transient heartbeat failure cancels the local run without racing lease recovery with a `FailPublish` call. The
server later classifies the expired lease from its durable pre- or post-fence state.

## Hermes is an executor, not the queue

Hermes owns one browser run. It does not own pairing, retries, leases, content snapshots or product status. The Mac
companion supervises a programmatic one-shot using a dedicated profile, versioned skill and a narrow publisher
plugin. The plugin is the structured boundary for manifest access and progress/completion; model prose is not parsed
as product state. `postpilot_finish` POSTs one validated terminal record to the per-run bearer-protected loopback
callback; the supervisor reads only that record, while Hermes stdout and stderr are discarded.

Content is data, including content that looks like an instruction. The one-shot prompt contains only a local job
handle. A pre-tool guard restricts navigation to the required Naver hosts and staged-file access to the current job;
a result transform requires exactly one dedicated CDP page, verifies it before and after each browser action, and
records only ref/label pairs from a real accessibility snapshot. URL evidence, page-scoped CDP calls, snapshots,
refs, the final activation, and readback all carry the same target id; a target switch fails closed before later
typing or upload. Every full snapshot is paired with an immediately preceding exact current-page URL read
through the only permitted read-only location expression. Editor evidence is accepted only when that URL identifies
the paired blog, and post-fence evidence is bound only to the exact open readback page. Before the fence,
unknown/stale refs, final-labelled or renamed/unknown buttons, keyboard
submission, and native-dialog acceptance are blocked independently of progress order; DOM-changing actions invalidate
every cached ref. JPEGs are resolved and uploaded alone in exact manifest-ordinal order, and the plugin counts one only
after both a successful `DOM.setFileInputFiles` result and a subsequent full accessibility snapshot with exactly one
additional editor image. Compact snapshots cannot supply upload or readback evidence. The fence also requires a current
full editor snapshot matching every frozen publishable field: title, exact semantic body/image/caption sequence with
no inserted publishable node, exact ordered tags with no extras/duplicates, category name, and visibility. The category ID must resolve through the exact
manifest-derived DOM selector, and category/visibility evidence must be selected or checked. From photo verification onward all activation is
blocked until the synchronous commit-fence call succeeds. That call requires exactly one button matching the
versioned final-control accessible-name allowlist and authorizes its exact ref in the verified snapshot; scheduled,
generic, or duplicate publish-like controls cannot arm the fence. Its one activation is consumed before the click,
and no other or repeated mutation is permitted.
Completion likewise requires the reported readback URL to
match a live page target from the paired CDP browser and a post-fence snapshot of that page to match the frozen title,
block/image/caption order, tags, and category. Web search, messaging,
delegation and general filesystem/terminal capabilities are absent. Unknown editor structure fails before the fence.

Hermes is intentionally behind a local executor interface. Plan 12 builds only the Hermes adapter, but a later
deterministic browser driver may replace it without changing the server job or pairing contracts.

## Browser profile rule

The user chooses a supported Chromium-family binary, not an arbitrary live personal profile. The companion launches
that binary with an agent-owned `user-data-dir` and loopback CDP port. The user signs into Naver visibly once. This
avoids profile locking and modern Chromium's restrictions on remote-debugging a default profile, and it keeps
automation state isolated from normal browsing.

Safari is outside the first version because the chosen Hermes path is Chromium/CDP. Login, CAPTCHA, 2FA and account
identity failures stop for local human repair; automation never attempts to bypass them.
The loopback setup page exposes a safe-label-only list of armed connections and can reopen the selected connection's
same dedicated profile for repair without a new device code or any path/credential disclosure.
The Postpilot agent-management page separately exposes account-scoped retained `needs_attention` jobs and their
explicit retry and confirmed cancel actions, so a deleted source post does not remove the recovery route or retain
its manifest/assets forever after agent revocation or identity drift.

As built, the companion launches the selected binary with `--remote-debugging-address=127.0.0.1`, an ephemeral
`--remote-debugging-port=0`, and the connection-owned `--user-data-dir`. It reads Chrome's `DevToolsActivePort`,
verifies `/json/version` returns a loopback WebSocket on the same port and with the browser id recorded in that
profile's `DevToolsActivePort`, and injects that exact URL as Hermes'
documented `BROWSER_CDP_URL`. A live setup/login browser is reused and never killed by the daemon; a browser process
launched for one job is closed after that run. The absolute Hermes executable is stored with the connection because
launchd must not depend on the interactive shell's `PATH`.

The named Postpilot Hermes profile sets `browser.backend` to `off`, disabling Hermes' default Browser Use CLI mode so
the narrower built-in `browser_*` toolset is available and bound to that exact CDP endpoint. Pairing additionally
allows only reversible Naver navigation and read-only screen inspection; publication tools, content entry, uploads,
and every final activation remain unavailable until a real claimed publish run.

An owner-only OS file lock prevents a manual `run` process and the LaunchAgent from operating concurrently. Within
that one daemon, a shared permit serializes every account's claim plus Hermes execution. Each claim revalidates the
server's current lease TTL, and setup applies the same heartbeat/lease check before it can sync ready or arm the
connection. Lease-renewal failure cancels Hermes, and abandoned work directories are removed only
after the process lock is held. Publisher diagnostics are reduced to a local redacted marker; only normalized failure
kinds cross to the VPS.

The Connect client rejects HTTP redirects and its transport adds the agent bearer only to the exact configured API
origin. Lease recovery at startup is database-only; terminal R2 deletion remains retryable sweeper housekeeping, so a
storage outage cannot prevent the rest of the API from serving or turn a durable terminal transition into an
ambiguous RPC result. Human-session calls see revoked-agent state as a failed precondition; only bearer-token
authentication uses unauthenticated.

## Compatibility rule

Hermes evolves independently from postpilot. Implementation must read the then-current official installation,
programmatic CLI, profile, plugin and browser/CDP documentation, install or verify the latest compatible release,
and probe capabilities before arming a connection. A detected version is diagnostic metadata, not a server-side
credential or a forever-pinned product constant.
