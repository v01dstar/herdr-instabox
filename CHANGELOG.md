# Changelog

## 0.5.1 — 2026-11-13

### Added

- Independent account profiles for simultaneous personal and work connections.
  Each profile has its own sign-in, refresh lock, SSH keys, certificates, jobs,
  workspace defaults, and snapshot/Secret configuration.
- Account management commands (`account list`, `account add`, `--account NAME`),
  scoped `whoami` and `logout`, and `[` / `]` account navigation in settings.
- Remembered settings-account selection and all-account startup/event sync.
- Automatic user/workspace binding from each profile's verified sign-in, with
  optional `identity.userId`, `identity.orgId`, and `identity.server` overrides.
- Per-snapshot credential profiles, including explicit empty selections and
  optional Vault disabling, so generic images do not inherit unrelated Secrets.
- Regression tests for account isolation, real CLI subprocess sync/renewal,
  identity overrides, rejected logins/refreshes, workspace scoping, and stale jobs.

### Changed

- Existing single-account sign-ins remain the `personal` profile without moving
  credentials or changing their SSH aliases.
- Settings restart in a fresh process when switching profiles rather than
  changing credentials underneath in-flight commands.
- Authenticated requests use their saved issuer, not ambient CLI server/account
  overrides. Certificate renewal carries its owning profile's configuration.
- Machine and snapshot lists are workspace-scoped. Creates explicitly target
  that workspace; actions, exec, snapshots, jobs, and SSH issuance validate scope.

### Fixed

- Syncing or signing out of one profile no longer removes another profile's
  connections or SSH configuration. Shared registry/config writes are serialized.
- Wrong-account login and refresh attempts cannot silently overwrite an existing
  profile's credentials. Malformed identity settings fail closed.
- Failed identity/workspace checks cannot fall back to an old cached SSH
  certificate; existing open sessions are left running.

### Notes

- Overrides require matching authentication and workspace membership; they do not
  grant server-side permissions or impersonate another user.
- Plugin sign-ins remain separate from the standalone InstaBox CLI. No CLI login
  or refresh tokens are imported.
- These are account-mixing safeguards, not isolation from other processes running
  as the same host OS user.
