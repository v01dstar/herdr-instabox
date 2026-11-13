# Using InstaBox in Herdr

[Home](../README.md) · [Authentication](authentication.md) · [Troubleshooting](troubleshooting.md)

## The normal loop

1. Select **Local**, open settings with `prefix+m`, and check the account badge.
2. In **remotes**, create a machine with **+ Add remote**, or start/resume an existing one.
3. Select it in Herdr’s sidebar and open a workspace. Work directly on its filesystem.
4. Return to Local for plugin shortcuts/actions. Suspend or stop the machine when appropriate; closing settings alone does not stop it.

An ordinary SSH remote can also be added through **+ Add remote → SSH**. The plugin’s InstaBox account cleanup does not adopt unrelated manual SSH entries.

## Defaults and navigation

Choose **Use as default** on a machine. `prefix+shift+m` creates a workspace there using the last selected account profile; otherwise Local is the default. It never starts a machine, and reports when the default is stopped, hidden, or unavailable. After remote creation, select the workspace in the sidebar yourself.

**Edit…** changes the Herdr-only label, not the cloud machine name. **Hide from sidebar** removes the saved Herdr machine but leaves the cloud machine untouched and visible in settings. **Show in sidebar** allows registration again once it runs.

## Lifecycle

| Cloud state/action | Herdr result |
| --- | --- |
| Running | Registered and enabled when connection checks succeed |
| Stopped, suspended, starting | An already registered machine stays present but disabled |
| Never seen running | Cannot be registered yet; Herdr validates the remote on add |
| Hidden | Removed from Herdr, retained in plugin settings |
| Deleted | Managed connection, Host block, and machine state removed |
| Logout | Profile’s managed connections removed; cloud machines keep running |

Sync runs at Herdr startup, at most once a minute on workspace focus/create events, and every 15 seconds for the open pane’s account. Changes elsewhere can briefly show as reconnecting until the next reconciliation. Use scoped `sync` to reconcile one profile explicitly.

## Copy and snapshots

**Copy machine…** offers a clone or a saved snapshot. The UI checks the machine’s state and explains when copying requires a stop. Review the button/confirmation before continuing: a stop interrupts work on that machine. From **snapshots**, choose **New machine from snapshot…** to start a separate environment.

Only snapshots matching configured templates or prefixes in the effective workspace are offered. Snapshot-specific Secret selection applies when creating the new machine, not retroactively to the snapshot or existing boxes. Audit images for baked-in credentials before reuse.

## Background operations and recovery

Long operations run as detached processes. Closing the pane does not cancel them. Progress/results are stored per account, and completion while the pane is closed produces a Herdr notification.

After an interrupted operation, refresh the cloud state before retrying a create/copy: remote work may have completed even if the local runner died. Do not replay internal `job` commands or edit job files to force success. See [recovery](troubleshooting.md#an-operation-stopped-unexpectedly).

## Custom keys

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

These defaults are installed only if their keys/actions are not already bound. Built-in key conflicts are disabled by Herdr; choose unused chords. Removing custom bindings is manual. Keep the stable plugin ID even when installing this fork.

## Plugin limitations

- Settings is a terminal overlay, not native Settings tabs; it cannot read Herdr’s theme.
- Plugin shortcuts currently execute only from Local, not from a remote server without this plugin.
- A plugin cannot switch the client to a newly created remote workspace.
- The sidebar itself does not contain InstaBox machine actions; use settings.
- There is no automatic local upload, patch-back, agent installation, or provider-login migration.
