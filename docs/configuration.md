# Configuration reference

[Home](../README.md) · [Examples](../examples/README.md)

Find the root config directory with `herdr plugin config-dir v01dstar.instabox`.

| Profile | File |
| --- | --- |
| `personal` | `CONFIG/config.json` |
| `work` | `CONFIG/accounts/work/config.json` |

`CONFIG` means `HERDR_PLUGIN_CONFIG_DIR` as supplied by Herdr at the root level. Without that environment variable, the personal fallback is `~/.config/herdr/plugins/config/v01dstar.instabox/config.json`. Named profile directories are selected automatically by `--account`.

All fields are optional. A missing file uses the `herdr` template, no explicitly selected Secrets/env, and the platform’s normal Vault behavior. Guided setup offers [this safer starter](../examples/minimal/config.json):

```json
{
  "secrets": [],
  "disableVault": true
}
```

An existing file must contain exactly one JSON object. Unknown fields, incorrect field types, malformed JSON, and unreadable configured `envFile`s are errors, not silent fallback. Restart settings after changing config; create/list operations also re-check it while running. `doctor` and `logout` remain available with broken config.

## Fields

| Field | Default | Meaning |
| --- | --- | --- |
| `templates` | `["herdr"]` | Template IDs whose machines/snapshots are managed; `"*"` accepts all templates |
| `metadata` | `{}` | All configured labels must match to additionally include a machine; also labels every create |
| `snapshotPrefixes` | `[]` | Also offer snapshots whose names start with any listed prefix |
| `secrets` | `[]` | Workspace Secret **names**, not values, selected for each create |
| `disableVault` | `false` | Send `vault: false` on create to prevent unrelated Vault delivery |
| `env` | `{}` | Plain environment variables sent on create; do not commit credentials here |
| `envFile` | absent | Local env file read at config load/create time; explicit `env` wins on collision |
| `defaultTemplate` | first template | Template for creates without a template |
| `defaultSnapshot` | absent | Snapshot ID/name used when the create supplies no snapshot; takes precedence over the template |
| `defaultCwd` | absent | Directory passed to Herdr for new workspaces; falls back to the first configured workspace’s `cwd` |
| `workspaces` | `[]` | Entries with `id`, `title`, `cwd`; documents repo locations and supplies the first default directory, not a repo-picker UI |
| `snapshotProfiles` | `{}` | Snapshot ID/name → `{ "secrets": [...], "metadata": {...} }` |
| `identity` | auto-bound | Optional `userId`, `orgId`, `server`; see [identity policy](authentication.md#identity-binding-and-workspace-overrides) |

Machine inclusion is **template match OR metadata match**, always restricted to the effective workspace. Broadening templates does not broaden account permissions. Empty `templates` currently falls back to `["herdr"]`.

`envFile` supports blank lines, comments, optional `export `, and matching surrounding single/double quotes. It is **not a shell**: no variable interpolation, command substitution, multiline values, or shell evaluation. Use an absolute path; `~` is not expanded. Keep it outside the repository and private.

## Snapshot-specific credential selection

```json
{
  "disableVault": true,
  "secrets": ["DEFAULT_MODEL_KEY"],
  "snapshotProfiles": {
    "work-ready": {
      "secrets": ["WORK_MODEL_KEY", "WORK_GH_TOKEN"],
      "metadata": { "credentialProfile": "work" }
    },
    "generic-clean": { "secrets": [] }
  }
}
```

A matching profile **replaces** the global Secret selection, including an omitted or empty `secrets` array, and merges metadata with snapshot values winning. It also clears `envSecrets` in the create payload. It does **not** clear global plain `env`, remove credentials baked into an image, or rotate an existing machine’s credentials.

UI creates use snapshot IDs; default creates may use a configured name. Add both ID and name entries when you need both routes, and update them when a snapshot is replaced. Credentials belong in InstaBox Secrets—not committed JSON. Vault disabling is separate from explicit Secret selection.

## Workspace directories

```bash
bin/herdr-instabox --account work new-workspace /data/workspace/api
```

An explicit directory overrides `defaultCwd`. The directory must exist on the target machine; config does not clone repositories or create folders. Each account has its own default machine, chosen through **Use as default** in settings.

## Environment

| Variable | Purpose |
| --- | --- |
| `HERDR_PLUGIN_STATE_DIR` | Herdr-supplied state root; keep consistent outside Herdr |
| `HERDR_PLUGIN_CONFIG_DIR` | Herdr-supplied config root |
| `HERDR_BIN_PATH` | Herdr executable override |
| `INSTABOX_SERVER` | Issuer for a new login when not overridden by profile identity settings |

Authenticated requests remain pinned to the issuer saved with the profile. Ambient standalone CLI server/API-key settings do not retarget an authenticated profile. Internal `HERDR_INSTABOX_*` account/root variables are managed by the plugin; do not set them as a substitute for `--account`.
