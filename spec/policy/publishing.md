# Policy — Paired local Naver publishing

Canonical rules under implementation for [plan 12](../plan/12.automated-naver-publishing-through-a-pai.md). Job 25's
server, frontend, and Mac companion paths are in offline verification, and the authorized live-Naver acceptance run
remains a release gate: this document does not claim that SmartEditor compatibility has been proven against a real
account.

## Explicit user action

- Publishing is separate from finalization, learning, autosave, generation, export, reconnect, and polling. Rendering
  `발행하기` issues reads only. `StartPublish` is called only after the user presses `네이버에 발행` and confirms that
  the Mac will activate Naver's final publish control without another prompt.
- Before opening that confirmation, the editor flushes any pending content save. If the saved revision is no longer
  the finalized revision, publication is refused and the user must finalize the new content first.
- Only the exact currently finalized `content_revision` may start. A stale tab, unfinalized post, foreign/revoked or
  unready agent, missing category, second live job, ambiguous outcome, or already-published post is refused.
- `내보내기` remains unchanged and is always the manual fallback. There is no scheduling or notification surface.

## Pairing and credentials

- Human publishing procedures use the existing HttpOnly session. Agent procedures use a separate random bearer
  credential; neither credential authenticates the other's procedures.
- A pairing code carries 48 random bits in three readable groups, is short-lived, hashed at rest, globally capped by
  one atomic conditional insert, and consumed once.
  The raw agent token is returned once, stored only in macOS Keychain, hashed on the server, account-scoped, and invalid
  immediately after revocation.
- Naver credentials, cookies, browser profile paths, CDP URLs, and Hermes provider credentials never enter the VPS,
  frontend state, RPC contract, or logs. Two Postpilot accounts on one Mac use separate Keychain entries, browser
  directories, Hermes profiles, queues, and job directories.
- Setup is loopback-only and work delivery is outbound authenticated polling. Public HTTP API URLs are refused; HTTP
  is allowed only for loopback development.
- Each setup server run uses a cryptographic URL/form nonce, accepts only its exact loopback Host and same-origin POST,
  and limits form size and actions. A constrained Hermes pairing skill follows the signed-in Naver UI to the account's
  own blog and opens its writer without typing, uploading, or publishing. Setup then reads the logged-in blog identity,
  editor readiness, and non-conflicting categories directly through the single local CDP page; Hermes/model stdout is
  never identity evidence. The probe accepts only the versioned editor root and the blog identity Naver selected from
  the signed-in session, then rechecks that the same exact page remains the sole CDP target. The first observed category
  is the initial default and can be changed in Postpilot. Only then may an unarmed
  recoverable local connection be persisted and server profile sync mark it ready. A lost sync response can therefore
  be retried with the same Keychain credential.

## Immutable publication payload

- `StartPublish` reads an ownership-checked, side-effect-free snapshot from the post context. It freezes canonical
  protojson, tags, selected category ID/name and visibility, expected Naver blog identity, and image order from IMAGE
  blocks. A category rename during the atomic start gate rejects the request rather than changing the frozen label.
- A job records the post's immutable creation timestamp with its slug snapshot. Deleting and recreating the same slug
  therefore starts a new publication identity and cannot inherit the old post's URL or uniqueness slot.
- The post hand-off revalidates the complete canonical content even for rows finalized before the current validator.
  Empty or canonically duplicate tags (whitespace collapsed and leading hashes removed) fail before any staging copy;
  the local publisher repeats the duplicate check before both commit-fence and readback acceptance.
- Every image is the already-converted JPEG from the upload pipeline. The storage adapter performs R2/S3 server-side
  copies to `publishing/{job_id}/{ordinal}.jpg`, verifies stored size, and never reads or decodes the bytes in the API.
  Each IMAGE occurrence has a unique ordinal filename plus its canonical source filename, so one attached JPEG may
  safely appear at multiple block positions without rewriting canonical content.
- A cryptographically random job ID is durably reserved before the first R2 copy and is never reused after a job has
  existed. Random-source failure or reservation collision stops before storage mutation. Cleanup releases only an
  unused reservation after every staged object was successfully removed.
- Claims mint short-lived GET URLs for the active lease. The manifest is detached from subsequent post edits and is
  retained only while the job can still run or receive an explicit safe retry.
- The Mac removes those signed GET capabilities from `manifest.json` after verified download, before Hermes can read
  it. Publication history is not foreign-keyed to the source post, so post deletion detaches rather than erases it.
- After staging, Start reserves the serialized writer, re-reads the exact post snapshot through post behavior and the
  current connection through publishing behavior, then inserts the job/assets in that transaction. Concurrent post
  edits, photo changes, profile sync, or revocation therefore either happen before the guard and reject Start or after
  the frozen job exists; publishing persistence never queries the post table directly.

## Lease and commit fence

- Legal progress is `queued → claimed → preparing → opening_editor → filling_content → uploading_photos → committing
  → verifying → published`; sequence numbers strictly increase and a current, unexpired lease token is required.
- A lease lost before `committing` requeues safely. The agent must receive a synchronous server acknowledgement that
  `committing` is durable before it may activate Naver's final control.
- A transient renewal failure cancels the local run and sends no competing terminal failure report. The server-owned
  lease expiry then performs the same safe pre-commit requeue or post-commit ambiguous classification.
- Progress writes compare the persisted stage and sequence atomically. Failure classification also reads the durable
  commit timestamp/stage in the same SQL mutation, so a delayed pre-commit failure cannot overwrite a concurrent
  commit fence.
- Every lease renewal, progress, completion, and failure SQL mutation also rechecks that the owning agent is not
  revoked. Revocation therefore fences an already-authenticated in-flight request atomically, not only its next RPC.
- Any failure, timeout, browser loss, or lease expiry at or after `committing` becomes `outcome_unknown`. It is never
  automatically retried, canceled, or treated as failed. Only a verified HTTPS URL under the paired
  `blog.naver.com/<blog-id>/...` path that is still open in the active paired browser completes a job. Its post-fence
  accessibility snapshot must match the frozen title, block/image/caption order, tags, and category; visibility is
  bound by the fully verified pre-fence editor snapshot.
- `postpilot_finish` validates locally and records its result once through the per-run authenticated loopback callback.
  Hermes stdout/stderr is discarded as non-authoritative model prose; a process exit without that callback becomes a
  normalized failure (or `outcome_unknown` after the fence).
- Editor/readback boundaries identify category, visibility, and tags by accessibility role/state, not label text
  alone. Body paragraphs may equal setting labels without being truncated. Terminal cleanup attempts all assets and
  all terminal jobs before returning an aggregated error; only fully cleaned jobs lose their asset rows, so one
  persistent object error cannot starve unrelated cleanup and remains retryable on the next sweep.
- Login expiry, CAPTCHA, 2FA, and account mismatch before commit become `needs_attention`. After local repair, the
  user may explicitly requeue the same job only with the same agent, revision, category, and visibility; no new post
  snapshot is read. This frozen retry does not depend on the post's current revision/finalization or continued
  existence. The account-scoped agent-management page lists retained retryable jobs, so source-post deletion cannot
  hide the retry action. The same page offers a confirmed cancel that purges the retained manifest/assets even after
  source deletion, agent revocation, or identity/category drift. Any pre-commit `needs_attention` job remains
  cancelable even when its Mac is unavailable. The loopback
  setup page lists safe labels for already-armed connections and can reopen the selected connection's same dedicated
  browser profile for repair without consuming another pairing code.

## Local executor boundary

- The companion launches the chosen supported Chromium binary with an agent-owned `user-data-dir` and an ephemeral
  loopback CDP port, verifies Chrome's WebSocket endpoint, and supplies only that local URL through Hermes'
  `BROWSER_CDP_URL` environment variable.
- Hermes receives a constant prompt containing one random local handle. The plugin checks that handle before reading
  the owner-only manifest directory. Manifest strings remain tool data; only enumerated current-job JPEG paths may be
  resolved or passed to the restricted CDP upload method.
- The profile exposes only the Postpilot publisher tools and a narrow browser allowlist. Direct shell/filesystem,
  search, messaging, delegation, arbitrary JavaScript evaluation, cookie/storage CDP methods, non-Naver navigation,
  and arbitrary local paths are blocked. Exactly one dedicated CDP page must exist, and its target id is bound across
  URL reads, DOM calls, full snapshots, refs, the final activation, and readback. The page is rechecked before and
  after every browser action; a target switch or leaving the allowlist poisons the run before later data entry. A full accessibility snapshot is bound to an immediately preceding exact
  `window.location.href` read through the only permitted read-only console expression. The editor URL's query/path
  identity must equal the paired blog before verification, and readback evidence applies only to that exact page URL.
  Before the fence, a click must reference the current real accessibility snapshot; any DOM mutation invalidates
  those refs. Snapshot-labelled final controls, renamed/unknown buttons, keyboard submission,
  and native-dialog acceptance are blocked regardless of model-reported stage. JPEGs must be uploaded one at a time in
  exact manifest-ordinal order. Each needs a successful one-file `DOM.setFileInputFiles` result followed by a fresh
  full accessibility snapshot with exactly one additional editor image; a compact snapshot or merely resolving a path
  is not upload evidence. The fence additionally requires one current full snapshot matching the title and exact
  semantic body sequence with no inserted text or image, block/image/caption order, the exact ordered tag collection
  with no extra or duplicate tag,
  frozen category name, and visibility. The frozen category ID must also resolve through its exact manifest-derived
  DOM selector, and both category and visibility require selected/checked accessibility state. Any subsequent
  pre-fence browser interaction invalidates that verification.
  From photo verification onward every activation is blocked. The fence accepts exactly one button whose accessible
  name is in the versioned final-control allowlist; scheduled, generic, renamed, or duplicate publish-like controls
  cannot arm it. After synchronous acknowledgement, only that exact ref may be activated, exactly once. Its authorization is
  consumed before the click, and every other or repeated mutation is blocked. Missing assets/evidence, mistaken tool
  ordering, stale refs, or a rejected durable progress call therefore fail closed.
- One owner-only OS file lock permits only one companion daemon on a Mac; a process-local permit additionally
  serializes all paired accounts. Startup removes abandoned job directories only after acquiring that lock. The
  daemon reloads the owner-only config every two seconds and starts newly armed account connections without restart.
- Agent HTTP clients reject redirects and inject the bearer only when the request origin exactly matches the configured
  API origin, so a redirect cannot carry the raw Keychain credential elsewhere.
- Setup and diagnostics run the current official Hermes install/profile/plugin doctor flow and a real loopback CDP
  connection probe. The Postpilot profile disables Hermes' Browser Use CLI backend so the guarded built-in
  `browser_*` tools attach to the exact injected CDP endpoint. A recorded checked release is evidence, not a forever
  pin or substitute for capability checks.
- Reusing a live Chromium debugging port requires the `/json/version` WebSocket browser path to equal the browser id
  recorded in that connection's `DevToolsActivePort`; a stale file whose port now belongs to another profile is refused.

## Retention and user-visible state

- Published, failed, canceled, and ambiguous jobs purge manifest content and staged objects while retaining metadata,
  normalized safe error text, settings, timestamps, and the verified URL. `needs_attention` retains its exact frozen
  manifest/assets only for explicit safe retry.
- Recovery checks leases at half the configured lease TTL, marks expired post-commit work ambiguous, and requeues
  only pre-commit work. Lease recovery is a database-only startup requirement; terminal object cleanup is retryable
  sweeper work, so an R2 outage cannot take down login or unrelated API reads. The independent publishing orphan sweep
  removes old unreferenced `publishing/` objects and protects every key referenced by a live/retryable job.
- Once a terminal transition is durable, its RPC returns that terminal job even if immediate staged-object deletion
  fails. Cleanup is idempotent sweeper work; an object-store outage must not make a successful publish/cancel/failure
  transition look ambiguous to either the Mac or the human client.
- Raw browser/model failure detail never enters an agent RPC. The Mac records only a redacted diagnostic marker, and
  the VPS derives normalized Korean-safe messages solely from the failure kind. Transient agent-auth storage failures
  remain retryable; only a revoked token permanently stops that connection. A revoked Mac encountered by a human
  session is `FailedPrecondition`, not `Unauthenticated`, and therefore never logs out the human user.
  The in-app panel is the only completion/status channel in this version.

## Configuration

| Value | Owner | Default |
|---|---|---:|
| `PUBLISH_PAIRING_TTL` | BE env/config | `10m` |
| `PUBLISH_MAX_PENDING_PAIRINGS` | BE env/config | `8` |
| `PUBLISH_LEASE_TTL` | BE env/config | `45s` |
| `PUBLISH_ASSET_URL_TTL` | BE env/config | `10m` |
| `PUBLISH_ORPHAN_SWEEP_INTERVAL` | BE env/config | `24h` |
| `PUBLISH_ORPHAN_MIN_AGE` | BE env/config | `1h` |
| `PUBLISH_JOB_POLL_MS` | FE shared config | `2000` |
| `PUBLISH_AGENT_STALE_MS` | FE shared config | `30000` |

The Mac companion uses typed defaults of 5 s polling with backoff/jitter, 10 s lease heartbeats, a 15 minute job
timeout, and 60 Hermes turns. It validates the enrollment lease before ready sync/activation and every claim's current
advertised lease TTL against the heartbeat cadence;
a renewal failure cancels the active Hermes run immediately.
