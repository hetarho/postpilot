# Postpilot Mac publishing agent

This separate Go runtime pairs a Postpilot account to one isolated Chromium profile and one named Hermes profile. It has no public listener: setup binds `127.0.0.1` on an OS-selected port, and queued work arrives through authenticated outbound Connect polling.

## Install and pair

From the repository root, the normal path is one command:

```sh
./setup-hermes.sh
```

It refreshes the checked-in companion and the current official Hermes install, opens local setup only when no armed
connection exists, runs diagnostics, and installs/reloads the user LaunchAgent. A successful first pairing closes
the temporary loopback setup server automatically, so the script can finish the remaining steps. Later Mac boots do
not require this command: launchd starts the polling agent automatically. Run the same command again to update and
reload the companion. Use `./setup-hermes.sh --setup` only when the local setup page is needed again for another
connection or interactive Naver-login repair.

The underlying commands remain available for diagnostics and development:

```sh
./packaging/install.sh
"$HOME/Library/Application Support/Postpilot Agent/bin/postpilot-agent" setup
"$HOME/Library/Application Support/Postpilot Agent/bin/postpilot-agent" install
```

Create the single-use device code in Postpilot's **발행 Mac** page. In local setup, choose the dedicated browser, sign into Naver, and complete pairing; there is no blog/category ID to copy by hand. The agent opens Naver's generic writer URL and accepts only the blog identity and categories resolved by that signed-in session together with a versioned editor root and the same sole CDP target after navigation. The loopback setup form is protected by a per-run nonce and exact local Host/Origin checks, and never trusts model output. The raw agent token is returned once and saved only in macOS Keychain. Each additional Postpilot account repeats this flow into separate Keychain, browser, Hermes and job directories; an already-running daemon detects the new armed connection within two seconds.

The installer deliberately passes the official Hermes `--skip-setup` option. A global Hermes model wizard would
configure the wrong profile; setup creates `postpilot-<connection-id>` and installs the restricted publisher plugin
there instead. If its capability probe reports missing model authentication, configure that exact named profile with
`hermes -p postpilot-<connection-id> setup --portal` (or `hermes -p ... model`), then submit the same recoverable setup
again.

Postpilot deliberately creates the named profile with `--no-skills` and invokes one-shot publishing with only the
`postpilot-publisher` and `browser` toolsets. Hermes' general `memory`/`skill_manage` learning surface is therefore
not available inside a publish run: browser publication stays reproducible and improves through reviewed plugin and
skill releases rather than silently learning from page content or model guesses.

If a queued job reports `needs_attention`, reopen local setup. Its existing active connections are listed without
local paths or credentials; **이 연결의 네이버 로그인 열기** reopens that connection's same dedicated browser
profile without issuing a new device code. Repair the login/challenge there, then use the retained retry action on
Postpilot's **발행 Mac** page; it remains available even if the source post was deleted.

Before a connection is armed, setup starts or attaches to the selected dedicated browser profile on an ephemeral loopback CDP port, injects its verified WebSocket URL through Hermes' documented `BROWSER_CDP_URL`, and runs `hermes doctor` plus `hermes plugins doctor ... --ci`. The absolute Hermes binary path is stored per connection so launchd does not depend on an interactive shell's `PATH`; `diagnostics` repeats the browser/CDP/plugin checks. The compatibility manifest records what was checked on 2026-08-30, but the installer always consults the official current installer and capability checks remain authoritative.

```sh
postpilot-agent diagnostics
postpilot-agent run
postpilot-agent uninstall
```

`uninstall` removes only the user LaunchAgent. It deliberately retains browser profiles and Keychain credentials unless the user separately confirms those destructive deletions.

## Verify the companion

```sh
go test ./...
go vet ./...
go build ./cmd/postpilot-agent
python3 -m unittest discover -s hermes/postpilot-publisher -p 'test_*.py'
```

These checks use fake browser/CDP, callback, storage, and Hermes boundaries and do not publish. A release is not considered live-verified until an explicitly authorized Naver test-blog smoke run exercises every block type and eight JPEGs across the durable commit fence.

The publisher requires one dedicated CDP page and binds its target id across URL reads, DOM calls, full accessibility
snapshots, refs, the final control, and readback. The editor URL must identify the paired blog before the fence, the
category ID must match its exact DOM selector, category/visibility must be selected or checked, and the readback URL
must be that exact open Naver post. Once the
server acknowledges the fence, exactly one button must match the versioned final accessible name and only that exact
control from the verified snapshot may be activated, once; scheduled/generic/duplicate controls fail closed, and the
authorization is consumed before the click and cannot be retried.
