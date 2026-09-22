# Postpilot Mac companion retirement bridge

Automatic publishing has been retired. This temporary Go module exists only so a Mac that previously installed
`com.postpilot.publishing-agent` can stop it and remove its app-owned local state. The binary does not pair an account,
open Naver, poll for work, install a LaunchAgent, or execute a publication. The old `setup`, `run`, `install`, and
`diagnostics` entry points return a retirement error.

Use the reviewed retirement build pinned by the deployment record. Inspection is the default and changes nothing:

```sh
go run ./cmd/postpilot-agent retire
```

The inspection lists exact known browser profile paths but preserves them. After reviewing the inventory, apply the
ordinary retirement:

```sh
go run ./cmd/postpilot-agent retire --apply
```

Apply first unloads and verifies the current user's `com.postpilot.publishing-agent`, then stops only a manual process
proven to be the installed companion binary. It deletes Keychain entries by the account names stored in `config.json`
without reading their tokens. It removes the installed binary, plist, known per-connection jobs, config, and logs.
Unrelated files, browser binaries, unknown legacy directories, and all browser profiles remain.

Profile deletion is a separate local choice. First inspect the exact paths above, then opt in explicitly:

```sh
go run ./cmd/postpilot-agent retire --apply --delete-profiles
```

The command refuses profile paths that escape the owned profile root, pass through a symlink, or have uncertain
ownership. It never contacts Naver or the Postpilot API.

Every apply writes an owner-only receipt at
`~/Library/Application Support/Postpilot Agent Retirement/shutdown-receipt.json` before deleting anything. The receipt
retains the minimum account and path inventory needed for an interrupted retry. Keep the receipt on that Mac; deployment
records use only its completion status and an operator-managed digest, never its local browser paths.

Validation for this temporary bridge uses fakes and does not stop a developer's daemon or access their Keychain:

```sh
test -z "$(gofmt -l .)"
go vet ./...
go build ./...
go test ./...
sh -n packaging/install.sh packaging/uninstall.sh
```
