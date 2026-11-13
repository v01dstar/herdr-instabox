# Troubleshooting and recovery

[Home](../README.md) · [Authentication](authentication.md)

## Start with local diagnostics

From a built checkout in the [installed plugin environment](authentication.md#cli-environment-get-the-roots-right):

```bash
bin/herdr-instabox --account work doctor
bin/herdr-instabox --account work doctor --json
```

`doctor` checks executable availability, config, saved credentials/binding, Include presence, and local permissions. It does not execute Herdr/SSH, contact a server, migrate saved credentials, refresh tokens, or repair files. Exit code is nonzero for local errors; warnings such as signed-out status do not fail the report. JSON has `schemaVersion: 1`, `ok`, account/platform/paths, and named checks with `ok`, `warning`, or `error` status.

It does not prove remote access. Use scoped `whoami` for verified user/workspace, then `sync` for reconciliation. Review paths and error messages before sharing diagnostics; never attach raw credentials, SSH private keys, certificates, or env files.

You can also run diagnostics in Herdr’s managed environment without finding state paths:

```bash
herdr plugin action invoke doctor --plugin v01dstar.instabox
herdr plugin log list --plugin v01dstar.instabox --limit 1
```

Action invocation is asynchronous: find the matching log record and wait for it to finish before reading its JSON stdout. This action normally diagnoses personal; for another profile use the explicit CLI `--account` form with the correct roots. The `setup` action opens the account tab for the remembered UI profile:

```bash
herdr plugin action invoke setup --plugin v01dstar.instabox
```

## Shortcut does nothing

Select **Local** first. Herdr sends custom bindings to the active machine’s server; a remote server generally does not have the local plugin. Check whether another custom or built-in command owns the chord. Linking a checkout does not build or install keys: run the build and `install-keys` commands from the README.

## Machine missing from the sidebar

- Check the account badge and `--account NAME whoami`; the profile may target a different workspace.
- Start the machine. A machine never seen running cannot be registered because Herdr validates the connection on add.
- Use **Show in sidebar** if it is hidden.
- Check `templates` / `metadata` in this profile’s config. Both are still workspace-scoped.
- Run scoped `sync`; use **Test connection** in settings and inspect the reported error.
- If stopped/suspended, an already registered entry is intentionally disabled.

## New workspace stays local or refuses to open

Each account has its own default; choose **Use as default** on the intended machine and verify the selected profile. Start the machine explicitly. A hidden/offline default is not automatically started. After successful remote workspace creation, select it in the sidebar manually. Verify the configured directory exists on the remote machine.

## Wrong browser account / identity mismatch

Use the intended account in the browser, or add a new profile for a different person/login. Rejected candidate logins must not overwrite an existing profile’s credentials. Logout does not erase the original binding. Do not delete `identity.json` as a troubleshooting shortcut: first review the expected user/workspace and any explicit overrides. Overrides require real authentication/membership.

## Google cannot sign in on SSH/headless Linux

Use Google from a local graphical session, or authenticate the intended user through GitHub’s device flow if that login is available. Google does not support this plugin’s GitHub device-code fallback.

## Invalid configuration after upgrading

Config is now strict: unknown keys and invalid types are errors. Fix the reported field rather than relying on silent defaults. A configured `envFile` must exist and be readable. Examples contain placeholders that must be replaced with your resources. `doctor`, `logout`, account management, and help remain available even with a malformed config.

## SSH certificate renewal fails

Run `whoami` for the owning profile, not just the last selected profile or standalone CLI account. Check that the machine belongs to its effective workspace and config has not changed scope. Renewal refuses to reuse a cached certificate after a failed identity/scope check. Existing open sessions may continue because the gateway checks certificates when connections open.

Setup/sign-in through settings adds this profile’s Include to the top of `~/.ssh/config`, retaining a backup as `config.herdr-instabox.bak` when replacing existing content. Use `doctor` to check Include presence. Do not restore an old whole-file backup over unrelated SSH changes; inspect and merge deliberately.

## An operation stopped unexpectedly

The local runner may have died after the remote operation succeeded. Reopen settings and refresh/sync the owning profile. Inspect actual cloud machines and snapshots **before retrying a create or copy**, to avoid duplicate resources. Job files/logs are under that profile’s `STATE/jobs/`; review locally and redact before sharing. Do not replay internal runner commands or alter job identity to bypass checks.

For start/stop/delete, inspect the current resource state before retrying its UI action. A changed workspace/identity intentionally rejects an admitted job. Restore the intended configuration or perform a new operation under the verified scope; do not force the old job through.

## Signed out, but machines still running

Expected: logout disconnects and revokes only the profile’s plugin sign-in. It does not stop/delete cloud machines. Sign back into the owning profile and stop/delete them explicitly. The standalone InstaBox CLI has a separate login and may show a different account.
