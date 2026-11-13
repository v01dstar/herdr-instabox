# Authentication and multiple accounts

[Home](../README.md) · [Two-account recipe](../examples/multiple-accounts/README.md)

## Three different kinds of “profile”

| Term | Meaning |
| --- | --- |
| Login provider | GitHub or Google, used to authenticate to InstaBox |
| Account profile | A local name such as `personal` or `work`, with independent sign-in and configuration |
| Snapshot credential profile | A `snapshotProfiles` entry selecting Secrets/metadata for an image; not a login |

An InstaBox workspace (`orgId`) is the API scope within an authenticated account. Profile names do not grant access or select a workspace by themselves.

## Recommended: settings

Select **Local** in Herdr, press `prefix+m`, then open **account**. Choose a login provider. Setup offers a private starter config only if no config exists. This starter selects no Secrets and sets `disableVault: true`. Declining keeps the default behavior, which does not disable Vault delivery.

GitHub and Google normally open a local browser using a loopback callback and PKCE. GitHub falls back to a device code if a browser cannot be used. Google requires a browser on the machine running the plugin; an SSH/headless session cannot use the GitHub device flow for Google.

For another account, choose **Add another account…**, enter `work`, select it with `]`, then sign in. The badge shows the local profile and login. `[` / `]` changes profiles by restarting the settings process, so pending requests never swap credentials underneath themselves.

Names are 1–32 lowercase letters, digits, `_`, or `-`, beginning with a letter. `personal` always exists and preserves the original installation’s paths and aliases.

## CLI environment: get the roots right

The examples use `bin/herdr-instabox` from a built checkout. Herdr supplies the installed plugin’s environment automatically. Outside Herdr, point to that same installation before running commands:

```bash
# Ask Herdr for its config directory; use your actual installed state directory.
herdr plugin config-dir v01dstar.instabox
export HERDR_PLUGIN_CONFIG_DIR="/absolute/path/printed/by/herdr"
export HERDR_PLUGIN_STATE_DIR="/absolute/path/to/installed/plugin/state"
```

Find the state directory in the running plugin’s environment or Herdr’s plugin diagnostics. Do not guess or use a different directory for each command. The standalone fallback is `~/.local/state/herdr-instabox`, which may not be the directory Herdr uses.

Set both variables to **root directories**, not `accounts/work`. A fresh shell is simplest; avoid carrying internal `HERDR_INSTABOX_ACCOUNT`, `HERDR_INSTABOX_ROOT_STATE_DIR`, or `HERDR_INSTABOX_ROOT_CONFIG_DIR` overrides from a profile subprocess.

```bash
bin/herdr-instabox --account personal setup github
bin/herdr-instabox account add work
bin/herdr-instabox --account work setup google
bin/herdr-instabox --account personal whoami
bin/herdr-instabox --account work whoami
bin/herdr-instabox sync                       # all profiles
bin/herdr-instabox --account work sync        # only work
bin/herdr-instabox --account work ui account  # remembers UI selection
```

`setup` offers initial config, signs in, and adds this profile’s SSH Include. It creates no cloud machine. `login` only performs sign-in; it does not perform setup’s SSH integration step. Existing config is never overwritten by setup.

`account list` prints a JSON array with `name`, `active`, `selected`, and `hasSignIn`. `active` describes this command’s process; `selected` is the remembered UI choice. A credentials file’s presence is not proof of a valid sign-in. `whoami` verifies online and prints `account`, `login`, `userId`, `server`, `orgId`, and `signedIn`.

Unqualified `ui`, `open`, and `new-workspace` use the remembered selection unless the account environment is set. Other unqualified commands normally use `personal`; unqualified `sync` also reconciles the other profiles. Use explicit `--account NAME` for scripts.

## What is isolated?

Named profiles use `STATE/accounts/<name>/` and `CONFIG/accounts/<name>/config.json`. Each owns credentials/refresh locks, SSH keys/trust/certificates, machine labels/hidden state/default, jobs, identity binding, and sync timing. Configuration starts absent—personal Secrets, env, and snapshot selections are not copied to work.

The Herdr machine registry and `~/.ssh/config` are shared, with serialized writes and namespaced targets. Signing out of work does not remove personal’s entries. Automatic certificate renewal uses the host block’s owning profile, not the standalone CLI’s active account or the UI selection.

## Identity binding and workspace overrides

On first verified use, the profile records its own user and default workspace in `identity.json`. Existing sign-ins migrate automatically. Later logins and refreshes must match the binding. Choosing the wrong browser account cannot silently replace the profile.

Normally **no IDs are needed**. To target another accessible workspace, set only the needed fields in that profile’s config:

```json
{
  "identity": {
    "orgId": "org_REPLACE_WITH_ACCESSIBLE_WORKSPACE"
  }
}
```

Advanced overrides also accept string `userId` and `server`. An alternate user requires authentication as that actual user. An alternate workspace requires membership; an override never grants permission. Removing an override restores the original binding, which persists across logout.

Machine/snapshot lists are filtered to the effective workspace. Mutations, exec, SSH issuance, and background jobs check scope. Jobs fail if identity/workspace changes before execution. Existing open SSH sessions are not killed when a later check fails.

## Signing out and migration

```bash
bin/herdr-instabox --account work logout
```

Logout revokes this sign-in and removes its local managed connections, key, certificates, and Include line. Names, hidden state, default, and identity binding remain for the next sign-in. Other profiles and the InstaBox CLI are unaffected. **Cloud machines keep running and may incur charges.** Stop/delete them separately if desired.

For an old manual SSH workaround, first sign in to the new plugin profile and verify it reaches the intended box/session. Only then remove the old manual Herdr entry/helper. Do not stop the machine or its sessions just to migrate; the plugin does not silently adopt or delete ordinary SSH remotes.
