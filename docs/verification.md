# Verification

[Home](../README.md) · [Contributing](../CONTRIBUTING.md)

## What automated checks establish

```bash
go test ./...
go test -race ./...
go vet ./...
go build -o bin/herdr-instabox .
python3 scripts/check-docs.py
```

Tests drive the real Go code and CLI subprocesses against a local fake API and fake Herdr executable. They cover account namespaces, cleanup isolation, identity/workspace guards, refresh and certificate context, jobs, config errors, setup safety, diagnostic immutability/redaction, and shipped example decoding. The documentation checker checks local links/anchors and basic Markdown hygiene without network access.

CI runs those checks on Linux and macOS using the Go version in `go.mod`. It does **not** authenticate to InstaBox, create real machines, prove Herdr UI behavior, or verify live browser-provider compatibility. A manifest minimum version is a requirement, not evidence that every version above it was tested.

## Disposable live checklist

Use test accounts/workspaces and narrowly scoped Secrets. Obtain approval for billable resources before running this checklist. Avoid production snapshots and never include tokens in evidence.

Record date, OS/architecture, Herdr version, plugin commit, Go version, and which checks passed/failed. Keep user IDs, account logins, paths, and resource names redacted as appropriate.

- [ ] Install this fork from a clean environment; verify the stable plugin ID and nonconflicting default keys.
- [ ] Open settings from Local and sign in through GitHub browser flow.
- [ ] With no config, accept the safe starter; check `0600` file and no overwrite on a second setup.
- [ ] Add work, sign in with Google, verify intended account/workspace; check personal remains connected.
- [ ] Separately verify GitHub device login from a headless environment; Google should explain its browser requirement.
- [ ] Attempt a wrong-account re-login; verify old credentials/connections remain intact.
- [ ] Configure different Secrets per profile. Inspect create selections and effective workspace before creating.
- [ ] Create one disposable machine per profile; confirm both appear and connect in Herdr.
- [ ] Clone/create a test repository on the remote; choose default and open a workspace with an explicit cwd.
- [ ] Switch UI account selection; confirm its independent default is used and existing sessions survive.
- [ ] Close settings during a long operation; confirm completion notification and correct account label.
- [ ] Save a disposable snapshot (include `/data` for the [dev-machine recipe](../examples/dev-machine/README.md)); create a copy and verify the marker file.
- [ ] Test per-snapshot nonempty and empty Secret selection with Vault disabled; do not print values.
- [ ] Suspend/resume and stop/start a disposable machine; inspect session behavior and sidebar state.
- [ ] Verify SSH certificate renewal uses its owner even when another profile is selected.
- [ ] Run `doctor --json`; verify it makes no cloud changes and exposes no tokens or env values.
- [ ] Restart Herdr; verify all-account startup reconciliation.
- [ ] Sign out work; verify only its managed local connections disappear and cloud machines remain.
- [ ] Sign back in and explicitly delete every disposable machine and snapshot. Verify cloud and local cleanup.

For failure recovery, interrupt only a disposable runner, inspect remote state before retrying, and check that no duplicate machine is created inadvertently. Do not use destructive tests against shared resources.

## Visual evidence

The README’s SVG is an **architecture illustration**, not a screenshot or proof of live verification. No live screenshots are bundled until captured from a real session with consent.

For a release, capture settings/remotes with both account namespaces connected, the account view showing intended workspace, and the create dialog’s Secret-name/Vault summary. Use disposable names, hide personal login/workspace identifiers, remove terminal scrollback containing auth URLs/device codes, and review images for secrets before committing. Include descriptive alt text and record the tested commit/version here.

## Release checklist

- [ ] All automated checks pass on Linux and macOS.
- [ ] Live checks above have a dated outcome, including unverified items.
- [ ] README install target, manifest version, changelog, and examples agree.
- [ ] Existing personal state/aliases remain compatible.
- [ ] Upstream attribution is retained; licensing is explicitly resolved before redistribution under a new license.
- [ ] Release notes identify configuration validation changes and any migration needed.
