# Postpilot Mac publishing agent

This Go runtime is the always-on receiver for mobile publication requests. It pairs each Postpilot account with one
Mac Keychain token and one dedicated Chromium profile. It exposes no public listener: setup binds to `127.0.0.1`, and
the background worker obtains durable jobs through authenticated outbound polling.

The publication path is intentionally deterministic:

```text
mobile StartPublish
  -> Postpilot durable queue and immutable manifest
  -> Mac LaunchAgent outbound claim/lease
  -> versioned local Naver DOM/Accessibility/CDP driver
  -> synchronous commit fence
  -> final click, URL/readback verification, terminal report
```

There is no model or general-purpose agent in the execution path. The server sends typed manifest data, never browser
selectors, JavaScript, shell commands, arbitrary URLs, credentials, cookies, profile paths, or CDP endpoints. Driver
behavior ships as reviewed local code. A changed editor, ambiguous control, missing asset, login challenge, or failed
readback must fail closed before commit or become `outcome_unknown` after the commit fence.

## Transition status

The previous local executor has been removed. Job 25 now tracks the deterministic Naver publisher and its fake-editor
and authorized live-Naver verification. Until that implementation and compatibility probe are complete:

- `postpilot-agent run` refuses to start before it can claim a queued job;
- `postpilot-agent diagnostics` verifies stored credentials and the dedicated browser transport, then reports that the
  publisher probe is unavailable;
- `packaging/install.sh` builds the companion but does not install the LaunchAgent automatically.

This fail-closed transition prevents an incomplete replacement from consuming or failing queued publication jobs.

For a Mac that ran the retired package, deploy backend migration 0015 first. It disarms every legacy connection and
terminates any old lease conservatively (`needs_attention` before the commit fence, `outcome_unknown` after it). Then
run `./packaging/install.sh`: it boots out and removes the old KeepAlive LaunchAgent before replacing the binary. Do
not run `postpilot-agent install` until Job 25's replacement-driver gates pass; the command is disabled in this build.
Existing browser profiles, config, logs, and Keychain credentials are deliberately retained for account recovery.

## Pairing and recovery contract

Local setup first creates an explicit Mac-owned connection draft with a random id and dedicated browser/work paths.
The draft survives setup restarts and device-code replacement, so reopening it always uses the same Naver cookie jar.
After login, setup navigates that same sole page to Naver's generic writer and reads the resolved blog identity and
categories through local CDP. Successful enrollment binds the server agent id and Keychain token to the existing
draft paths; the one-time device code never selects a profile. A per-run nonce plus exact Host/Origin checks protect
the loopback form. The raw agent token is returned once and stored only in macOS Keychain.

Each additional Postpilot account receives separate Keychain, browser-profile, and job directories. Reopening a
connection for login, CAPTCHA, or two-factor repair must reuse the same stable browser profile without issuing a new
device code. Postpilot retains a pre-commit retry action; the driver never attempts to bypass a challenge.

## Development commands

```sh
./packaging/install.sh
"$HOME/Library/Application Support/Postpilot Agent/bin/postpilot-agent" setup

go test ./...
go vet ./...
go build ./cmd/postpilot-agent
sh -n packaging/install.sh packaging/uninstall.sh
```

`uninstall` removes only the user LaunchAgent. Browser profiles, config, logs, and Keychain credentials remain until
the user explicitly removes the account-specific data.

## Release gate

The deterministic publisher must bind one CDP page and target id across all DOM/accessibility operations. It must
verify title, ordered blocks, up to eight JPEGs and captions, ordered unique tags, category, visibility, final-control
uniqueness, and post-publication readback. The server must acknowledge `committing` before exactly one final click.

Automated tests require a fake Naver editor covering normal publication, editor fingerprint drift, login challenges,
target changes, asset mismatch, pre/post-commit process death, duplicate/late progress, and cleanup. A release is not
live-verified until an explicitly authorized Naver test-blog smoke run exercises every canonical block type and eight
JPEGs across the durable commit fence.
