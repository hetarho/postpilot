# Paired local publishing agent

> Technical decision under implementation for [plan 12](../plan/12.automated-naver-publishing-through-a-pai.md).
> The server/frontend/durable-queue and companion foundations remain, but the replacement deterministic publisher is
> not implemented and the agent fails closed. Its offline suite and an explicitly authorized live Naver smoke remain
> release gates.

## Why the executor is local

Naver Blog has no public writing API. The browser that can publish therefore needs a logged-in Naver session and
must survive editor changes and interactive account challenges. Keeping that browser on the owner's Mac gives it the
owner's familiar network and a visible recovery surface while keeping Naver credentials out of the VPS.

The Mac is not made into a public server. A companion daemon dials the postpilot API, claims a durable job and invokes
the deterministic local Naver publisher. Polling is merely delivery; SQLite is the source of truth. Sleep, network
changes and process restarts therefore delay a job instead of losing it, and the Mac exposes no inbound port, tunnel,
CDP endpoint or webhook.

## Two credentials, two trust domains

The normal postpilot HttpOnly cookie proves a human browser session. A publishing-agent bearer token proves one
paired local capability. They are deliberately not interchangeable:

- a human session creates/revokes pairings and starts/cancels publication;
- an agent token can sync only safe labels/categories plus driver name/version, an opaque compatibility-signature id
  and readiness, claim only its owner's jobs and report only a currently leased job; locators or executable actions
  are not profile metadata;
- the single-use pairing code carries 48 random bits, is short-lived and is persisted only as a hash;
- the raw agent token is returned once, stored in macOS Keychain and hashed on the server; and
- Naver credentials, cookies, browser profile paths and CDP endpoints remain local.

Each postpilot account is paired separately. Two connections on one Mac use isolated browser user-data directories
and job directories, so neither profile lock nor cookie jar is shared. The running daemon reloads its owner-only
configuration every two seconds and starts a newly armed account without a reboot or manual restart.

Setup lets the deterministic driver follow only the signed-in Naver UI to the account's own blog and open its writer
in the same visible dedicated page without typing, uploading, or publishing. It then observes the blog id selected by
Naver, a versioned editor-specific root, and conflict-free categories through local CDP before enrollment completes.
It rechecks that the same page is still the sole CDP target; user input, page prose, or a claimed identifier is not
identity evidence. Its per-run cryptographic nonce, exact loopback Host/Origin checks, and bounded action/form parser
protect the local setup mutation surface. It writes the Keychain token reference and a complete but unarmed local
connection before `SyncAgentProfile` may expose the server record as ready; an interrupted/lost response is resumed
with that pending connection instead of consuming another pairing code.

## Immutable payload, mutable delivery

Publication has an external side effect, so the queued payload cannot follow live post state. The server freezes the
exact finalized protojson and settings and copies the referenced, already-converted JPEG objects to job-owned R2
keys. A claim may mint new expiring read URLs repeatedly, but every claim points to the same immutable bytes and
manifest.

This is not a second content canon. The stored manifest is a snapshot of the canonical block array, and terminal
jobs purge it. It is also not new image processing: R2 copies the JPEG objects; the API never receives or decodes
their bytes. Signed claim URLs are used only by the downloader and are stripped from the local manifest before the
publisher reads it. Publication metadata survives source-post deletion because the history row stores the slug
snapshot without a post foreign key. The row also stores the source post's immutable creation timestamp, so a later
post that reuses a deleted slug cannot inherit that history. Repeated references to one canonical image receive
distinct ordinal local filenames while retaining the source filename used by the unchanged content snapshot.

Before storage staging, the server durably reserves the random job ID/prefix. A random-source error or collision stops
before any R2 copy, and a successful job's reservation is retained so a future collision cannot overwrite historical
assets. An unused reservation is released only after complete staging compensation.

## At-most-once after the commit fence

Browser publication cannot offer transactional exactly-once semantics. A process can die after the remote site
commits but before postpilot receives the URL. The design makes that uncertainty explicit instead of pretending a
retry is safe:

1. work before the final click is lease-retryable;
2. the deterministic publisher reports `committing` immediately before that click;
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

## The deterministic publisher is an executor, not the queue

The local publisher owns one browser run. It does not own pairing, retries, leases, content snapshots or product
status. The Mac companion supervises a fixed, versioned state machine whose only input is the validated local manifest
and the enumerated current-job JPEGs. Content is inert data even when it resembles an instruction. There is no model,
prompt, learned behavior, or free-form planner in the publication path.

The cloud protocol remains the typed publishing-agent contract: claim one immutable job, renew its lease, report only
legal progress, and complete or fail it. The server never supplies selectors, JavaScript, coordinates, keystroke
scripts, or a general command language. Browser actions and compatibility signatures ship as reviewed local driver
code. A screen-coordinate recorder is not an implementation option: viewport, focus, scroll, DPI, and incidental
dialogs cannot be product authority.

The driver restricts navigation to the required Naver hosts and staged-file access to the current job. It requires
exactly one dedicated CDP page, verifies it before and after every browser action, and binds URL evidence, page-scoped
CDP calls, accessibility nodes, the final activation, and readback to the same target id. A target switch or leaving
the host allowlist poisons the run before later typing or upload. Editor evidence is accepted only when the exact URL
identifies the paired blog, and post-fence evidence applies only to the exact open readback page.

Before the fence, every mutation resolves a versioned semantic locator against current DOM/accessibility evidence;
renamed, missing, duplicate, or unexpected controls fail closed. Keyboard submission, native-dialog acceptance, and
unversioned publish-like controls are not alternative commit paths. JPEGs are resolved and uploaded alone in exact
manifest-ordinal order, and the driver counts one only after both a successful one-file upload and a fresh full
accessibility snapshot with exactly one additional editor image.

The fence requires a current full editor snapshot matching every frozen publishable field: title, exact semantic
body/image/caption sequence with no inserted publishable node, exact ordered tags with no extras or duplicates,
category name/id, and visibility. Category and visibility must expose selected or checked state. Any subsequent
pre-fence mutation invalidates that evidence. From photo verification onward all activation is blocked until the
synchronous server commit-fence call succeeds. That acknowledgement authorizes one exact final control on the bound
target; authorization is consumed before one low-level activation is sent, and the driver has no automatic retry for
that action.

Completion requires the reported readback URL to match the active bound page and a post-fence accessibility snapshot
to match the frozen title, block/image/caption order, tags, and category. Unknown editor structure fails before the
fence. The publisher has no general filesystem, shell, search, messaging, delegation, cookie/storage inspection, or
arbitrary-script capability. It runs behind an in-process typed reporter that persists each monotonic transition
synchronously, rejects skipped/duplicate/late progress, closes when the publisher returns, and accepts exactly one
typed terminal result.

## Browser profile rule

The user chooses a supported Chromium-family binary, not an arbitrary live personal profile. The companion launches
that binary with an agent-owned `user-data-dir` and loopback CDP port. The user signs into Naver visibly once. This
avoids profile locking and modern Chromium's restrictions on remote-debugging a default profile, and it keeps
automation state isolated from normal browsing.

Safari is outside the first version because the publisher is designed against Chromium/CDP. Login, CAPTCHA, 2FA and account
identity failures stop for local human repair; automation never attempts to bypass them.
The loopback setup page exposes a safe-label-only list of armed connections and can reopen the selected connection's
same dedicated profile for repair without a new device code or any path/credential disclosure.
The Postpilot agent-management page separately exposes account-scoped retained `needs_attention` jobs and their
explicit retry and confirmed cancel actions, so a deleted source post does not remove the recovery route or retain
its manifest/assets forever after agent revocation or identity drift.

The companion launches the selected binary with `--remote-debugging-address=127.0.0.1`, an ephemeral
`--remote-debugging-port=0`, and the connection-owned `--user-data-dir`. It reads Chrome's `DevToolsActivePort`,
verifies `/json/version` returns a loopback WebSocket on the same port and with the browser id recorded in that
profile's `DevToolsActivePort`, and gives that exact endpoint only to the local publisher. A live setup/login browser
is reused and never killed by the daemon; a browser process launched for one job is closed after that run. Pairing
allows only reversible Naver navigation and read-only identity/category inspection; content entry, uploads, and every
final activation remain unavailable until a real claimed publish run.

An owner-only OS file lock prevents a manual `run` process and the LaunchAgent from operating concurrently. Within
that one daemon, a shared permit serializes every account's claim plus publisher execution. Each claim revalidates the
server's current lease TTL, and setup applies the same heartbeat/lease check before it can sync ready or arm the
connection. Lease-renewal failure cancels the active driver, and abandoned work directories are removed only
after the process lock is held. Publisher diagnostics are reduced to a local redacted marker; only normalized failure
kinds cross to the VPS.

The daemon's 5 s poll (mirrored as `Supervisor.Run`'s fallback) drives the server's agent authentication, which
refreshes `last_seen_at` at most once per `PUBLISH_AGENT_HEARTBEAT_INTERVAL` (`15s`) per agent. Because the refresh lands on the
first poll after the interval elapses, the worst staleness is the interval plus the poll (`20s` at the defaults), and
that sum is what must stay below the frontend's `PUBLISH_AGENT_STALE_MS` (`30000`). Lowering the poll therefore
multiplies requests without multiplying writes, and tightens this budget rather than loosening it.

The Connect client rejects HTTP redirects and its transport adds the agent bearer only to the exact configured API
origin. Lease recovery at startup is database-only; terminal R2 deletion remains retryable sweeper housekeeping, so a
storage outage cannot prevent the rest of the API from serving or turn a durable terminal transition into an
ambiguous RPC result. Human-session calls see revoked-agent state as a failed precondition; only bearer-token
authentication uses unauthenticated.

## Compatibility rule

The driver is versioned together with its Naver editor signature, semantic locator set, supported Chromium/CDP
capabilities, and manifest schema. Setup and diagnostics run a non-publishing probe against the exact dedicated
profile and refuse to arm a connection when the editor root, account identity, category discovery, sole-target rule,
or required CDP/accessibility capability does not match. A detected browser/editor/driver version is safe diagnostic
metadata, not a credential or proof by itself.

Compatibility data selects only code already present in the installed, reviewed agent. Neither profile sync nor a
claimed job may deliver executable JavaScript, CSS/XPath selectors, screen coordinates, or action sequences from the
server. Supporting a changed Naver editor requires a new signed agent/driver release and a fresh probe. Until then the
connection is unready or the job fails `editor_changed` before `committing`; it never guesses or falls back to a
coordinate macro.
