# herdr-instabox

A herdr plugin for instabox cloud machines. Sign in, create, start, suspend, stop,
copy and delete machines from a settings pane modelled on herdr's own
Settings → remotes / snapshots / account; running machines appear in herdr's
sidebar on their own, like any saved SSH machine. herdr itself is unmodified.

It talks to the instabox API (`https://api.instabox.sh`, or
`INSTABOX_SERVER`) directly and drives the `herdr` CLI (saved machines,
workspaces, notifications). The instabox CLI is not needed; the plugin has its
own sign-in, separate from the CLI's. Linux and macOS.

See [CHANGELOG.md](CHANGELOG.md) for release notes.

## Install

```bash
herdr plugin link .                      # from a checkout (run `go build -o bin/herdr-instabox .` first)
herdr plugin install v01dstar/herdr-instabox   # needs Go to build
```

Open it with the `instabox settings` action, or with the keys that installing
adds to herdr's `config.toml`: `prefix+m` for instabox settings and
`prefix+shift+m` for a new workspace on the default machine. A key or action
you have already bound is left alone, and removing the bindings is up to you.
`herdr plugin link` runs no build, so after linking run
`bin/herdr-instabox install-keys` yourself. To bind other keys, edit the config;
pick keys herdr does not already use, because a custom binding that conflicts
with a built-in one is disabled.

```toml
[[keys.command]]
key = "prefix+m"
type = "plugin_action"
command = "v01dstar.instabox.open"
description = "instabox settings"

[[keys.command]]
key = "prefix+shift+m"
type = "plugin_action"
command = "v01dstar.instabox.new-workspace"
description = "new workspace on the default machine"
```

## What it does

- **remotes**: Local, your instabox machines and your own SSH remotes, each with
  its actions: Start / Resume / Suspend… / Stop…, Test connection, Edit… (a
  herdr-only name), Use as default, Hide from / Show in sidebar, Copy machine…
  (clone now, or save as snapshot), Delete machine…. `+ Add remote` creates a
  machine from the herdr template or one of your snapshots, or adds an SSH remote.
- **snapshots**: your snapshots of herdr machines (snapshots of machines on other
  instabox templates are not listed); New machine from snapshot…, Delete snapshot….
- **account**: Add another account profile, sign in with GitHub or Google
  (browser, or a GitHub device code without one), sign out of just this profile,
  and view its storage/machine/snapshot usage. Press **`[` / `]`** to select a
  profile without disconnecting the others.
- **New workspace** opens a workspace on the default machine (Local unless you
  chose another with Use as default). It never starts a machine.

Long operations (create, start, stop, copy, delete) run as background processes:
closing the pane does not interrupt them, and a result that arrives while the
pane is closed shows as a herdr notification.

## How machines get into herdr

The plugin reconciles herdr's saved machines with instabox when herdr starts, at
most once a minute as you switch workspaces, and every 15 seconds while the
pane is open:

| instabox machine | herdr |
|---|---|
| running | saved (`herdr machine add`, which starts its `herdr-remote` session) and enabled |
| stopped, suspended, starting… | disabled (it stays in the sidebar) |
| hidden | removed from herdr, still listed in the pane |
| deleted | removed, with its SSH config and plugin state |

A machine that has not been running since you signed in cannot be saved yet
(herdr checks the remote when it saves one), so it appears once it runs.

SSH: each machine gets a Host block `herdr-instabox-<machine id>` in the plugin's
state directory, pulled in by one `Include` line that signing in adds to the top
of `~/.ssh/config` (a backup is kept as `~/.ssh/config.herdr-instabox.bak`). The
plugin connects with its own SSH key and 24-hour certificates the API issues
for it; a `Match exec` in front of each block renews the certificate before a
connection when less than 10 minutes are left. The gateway checks a
certificate only when a connection opens, so an open connection outlives it.

Signing out revokes this account profile's sign-in and removes its local traces: the herdr
machines, the Host blocks, the SSH key and certificates, and the `Include` line. The machines keep running on instabox, and their herdr-only
names, hidden state and the default are kept for the next sign-in.

## Multiple accounts

Personal and work accounts can stay signed in and connected at the same time.
In the **account** tab choose **Add another account…**, give it a name such as
`work`, then press `]` to select it and sign in. The badge identifies the profile
and actual login. `[` / `]` moves between profiles; the last UI selection is
remembered, including for the New workspace action.

The original sign-in is the **`personal`** profile. Its files and SSH aliases
are not moved or rewritten during upgrade. Named profiles have their own:

- Credentials and refresh lock, stored under `STATE/accounts/<name>/`.
- SSH private key, host trust, certificates, renewal and Include block.
- Jobs, machine labels, hidden state, default workspace target and sync timers.
- `CONFIG/accounts/<name>/config.json`, including snapshot/Secret selections.
  This file starts absent: personal Secrets/env/snapshot defaults are **not**
  inherited into work. Configure it before creating work boxes through the UI.

Names use 1–32 lowercase letters/digits/`_`/`-`, starting with a letter. The
state/config roots are Herdr's `HERDR_PLUGIN_STATE_DIR` and
`HERDR_PLUGIN_CONFIG_DIR`. Account directories are private (0700); credentials,
keys and state files are private (0600). No CLI login tokens are imported.

CLI equivalents, from a plugin environment (outside Herdr, set the state/config
roots to the installed plugin's directories first):

```bash
bin/herdr-instabox account list
bin/herdr-instabox account add work
bin/herdr-instabox --account work login google
bin/herdr-instabox --account work whoami
bin/herdr-instabox --account work sync
bin/herdr-instabox --account work ui
bin/herdr-instabox --account work logout
```

The unqualified startup/event `sync` reconciles **all** accounts; an explicit
`--account NAME sync` limits it to that account. A profile can only manage its
own namespaced Herdr SSH targets. Syncing, deleting, or signing out cannot
remove another profile's machines, keys or certificates. Shared Herdr/SSH config
writes are serialized across profiles. Selecting another UI profile restarts
the UI process instead of swapping credentials underneath pending requests.

Certificate renewal uses the credentials alongside the SSH host block, not the
CLI's active account. Authenticated API calls stay with the issuer saved in that
profile, regardless of an ambient CLI server override. No CLI account switching
is needed once both plugin profiles have their own sign-ins.

**Migrating a manual SSH workaround:** sign into the new plugin profile first
and verify the managed connection reaches the same box and remote session. Only
then remove the old manual Herdr profile/SSH helper; do not stop the box or its
sessions. The plugin does not silently adopt or delete ordinary SSH profiles.

### Default identity and optional overrides

No IDs need to be configured by default. On first use, each profile verifies its
**own signed-in IBX user and default workspace** and records that non-secret
binding in `identity.json` alongside its state (0600). Existing sign-ins migrate
automatically. Later sign-ins and token refreshes must match, preventing a wrong
browser account from silently taking over a profile. This does not follow the
standalone CLI's login or the sandbox's Linux username.

Override only the fields you need in that profile's `config.json`:

```json
{
  "identity": {
    "userId": "-63427",
    "orgId": "org_8f3f687fdd824693ba9126ce9924b1ba"
  }
}
```

`server` may also be specified when using a different IBX issuer. User IDs are
strings. An alternate user requires signing in as that actual user; an alternate
workspace must be accessible to the signed-in user. Overrides do **not** grant
permissions. Omitting fields uses the automatic binding; removing an override
restores it. The original binding stays in place across sign-out.

Machines and snapshots are filtered to the effective workspace. Creates send
its `orgId` explicitly, while machine actions/exec, snapshot actions and SSH
certificate issuance reject out-of-scope targets. Background jobs record their
identity/workspace and fail if that changes before execution. SSH renewal carries
the profile's config as well as its credentials; a failed identity/scope check
cannot reuse an old cached certificate. Existing open SSH sessions are not killed.

`--account NAME whoami` reports the effective workspace. Malformed identity
settings fail closed. These checks prevent accidental account mixing, not
malicious access by another process running under the same host OS user; IBX
server permissions remain the actual authorization boundary.

## Differences from built-in support

Things a plugin cannot do with herdr's current plugin API:

- The pane is a terminal overlay (a popup once herdr ships popup placement),
  not tabs inside herdr's Settings, and it cannot read herdr's theme.
- New workspace cannot switch the client to another machine; after creating
  a remote workspace it points you to the sidebar.
- Between reconciliations herdr may briefly show a machine that changed state
  elsewhere (for example, auto-suspended) as reconnecting.
- herdr's sidebar has no instabox actions; use the pane.
- Key bindings for the plugin's actions work only while Local is the active
  machine. herdr sends a custom key binding to the active machine's server,
  and a remote machine's server has neither the binding nor the plugin.
  Switch to Local in the sidebar first.



## Configuration (fork)

The fork reads `HERDR_PLUGIN_CONFIG_DIR/config.json` (normally
`~/.config/herdr/plugins/config/v01dstar.instabox/config.json`) so a deployment
can point the plugin at a prepared base snapshot and wider machine set without
changing code. Every field is optional; without the file the plugin behaves like
upstream (the `herdr` template, no Secrets or env).

```json
{
  "templates": ["agent-browser", "herdr"],
  "snapshotPrefixes": ["1984-"],
  "metadata": { "purpose": "1984", "herdr": "true" },
  "secrets": ["GH_TOKEN"],
  "env": { "APP_ENV": "development" },
  "envFile": "/home/me/instabox/box.env",
  "defaultTemplate": "agent-browser",
  "defaultSnapshot": "1984-ready",
  "defaultCwd": "/data/workspace/bigbrother",
  "workspaces": [
    { "id": "bigbrother", "title": "BigBrother", "cwd": "/data/workspace/bigbrother" }
  ]
}
```

- `templates` / `metadata`: which machines are synced into Herdr. A machine
  matching either is included.
- `snapshotPrefixes`: snapshots that are offered as create sources in addition
  to snapshots whose template is listed.
- `secrets`, `env`, `envFile`: sent on every create (`envFile` is read at create
  time; `env` wins on collisions).
- `defaultTemplate`, `defaultSnapshot`: used when a create names neither.
  `defaultSnapshot` wins when a snapshot exists.
- `defaultCwd` / `workspaces`: `new-workspace` passes `--cwd`; a keybinding can
  call `bin/herdr-instabox new-workspace /data/workspace/mcps` for another repo.

## Development

```bash
go test ./...        # runs against a fake instabox API and the fake herdr CLI in testdata/fake
go build -o bin/herdr-instabox .
```
# Local snapshot credential profiles

This checkout adds `snapshotProfiles` to `config.json`, keyed by snapshot ID
(used by the UI) or name. Each entry has `secrets` and `metadata`. A matching
profile **replaces** the global Secret selection and merges metadata. This keeps
personal and work OpenRouter credentials separate and lets generic images omit
GitHub auth. `disableVault: true` prevents unrelated personal Vault delivery on
new machines. Configure both name and ID entries when changing a snapshot.

The active local configuration is at
`~/.config/herdr/plugins/config/v01dstar.instabox/config.json`.
There are no credentials in that file. Run `go test ./...` before rebuilding.
