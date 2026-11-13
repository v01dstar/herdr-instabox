# Personal GitHub + work Google, connected together

## 1. Establish both profiles

In the [installed plugin environment](../../docs/authentication.md#cli-environment-get-the-roots-right):

```bash
bin/herdr-instabox --account personal setup github
bin/herdr-instabox account add work
bin/herdr-instabox --account work setup google
```

Choose your actual personal/work login in each browser flow. Google requires a local browser. If work must use a headless device code, use `setup github` with the intended work GitHub login instead; provider choice does not determine the local profile name.

## 2. Configure each account independently

Back up existing files, then copy:

| Example | Destination |
| --- | --- |
| [personal.json](personal.json) | `CONFIG/config.json` |
| [work.json](work.json) | `CONFIG/accounts/work/config.json` |

Create the named Secrets in each intended InstaBox workspace first, or replace the lists with `[]`. Replace the workspace directories with remote paths you will actually create. These examples deliver credentials to new machines; use narrowly scoped tokens. Personal config is not inherited by work.

## 3. Verify before creating anything

```bash
bin/herdr-instabox account list
bin/herdr-instabox --account personal whoami
bin/herdr-instabox --account work whoami
bin/herdr-instabox --account work doctor --json
```

Expect `personal` and `work` in the list and the intended user/workspace in each `whoami`. `hasSignIn` alone does not verify authentication. No IDs need to be set manually for ordinary accounts.

## 4. Work on both machines

Open settings, select personal with `[` / `]`, create a personal machine and choose **Use as default**. Select work, create a work machine, and set its own default. Both machines remain connected in the sidebar. Clone the corresponding repositories on the remote machines and create the configured directories.

```bash
bin/herdr-instabox sync
bin/herdr-instabox --account work new-workspace /data/workspace/work-app
```

Select the new workspace in Herdr’s sidebar. The normal shortcut uses the last UI-selected profile; explicit `--account` avoids ambiguity in scripts.

## 5. Prove logout is scoped

```bash
bin/herdr-instabox --account work logout
bin/herdr-instabox --account personal whoami
```

Expected: work’s local connections disappear; personal remains signed in and connected. Work’s cloud machine still exists and may be running. To stop/delete it, sign back into work and use its machine actions. Restore your config backups when finished; logout intentionally retains the profile’s original identity binding.
