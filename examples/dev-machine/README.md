# Prepare once, reuse as a development machine

This is a complete manual workflow using the existing UI. It creates billable cloud resources; stop/delete them when finished. No local checkout or host credentials are uploaded by the plugin.

## Create and prepare

1. Follow the [minimal recipe](../minimal/README.md) and sign in to the intended account. Check its login/workspace with `whoami`.
2. In settings → **remotes → + Add remote**, create `docs-demo-base` from the `herdr` template.
3. Wait for it to run, select it in Herdr’s sidebar, and open a shell/workspace on it.
4. Run this **on the remote machine**, not your laptop:

   ```bash
   mkdir -p /data/workspace/docs-demo
   cd /data/workspace/docs-demo
   git init
   printf 'Prepared in InstaBox\n' > README.md
   git status --short
   ```

   Expected: `README.md` is untracked in the remote checkout. A snapshot preserves the disk state, not just committed Git files. This demo requires Git in the image.

## Snapshot and reuse

1. Return to **Local**, open settings for the same account, and choose **Copy machine… → save as snapshot** on `docs-demo-base`.
2. Name the snapshot `docs-demo-ready`. Read the state check and any stop confirmation; stop-sensitive operations interrupt sessions, so save work first.
3. In **snapshots**, select `docs-demo-ready` and choose **New machine from snapshot…**, naming it `docs-demo-copy`.
4. Once running, open a workspace on the copy and run:

   ```bash
   cat /data/workspace/docs-demo/README.md
   ```

   Expected: `Prepared in InstaBox`. Changes on this copy do not become live edits on the source machine or your laptop.

## Add credential policy deliberately

If this becomes a reusable image, configure a [snapshot credential profile](../snapshot-credentials/README.md) by its actual ID and name before creating further machines. Never bake model tokens, SSH private keys, or authenticated Git helpers into a shared snapshot.

## Clean up

From the owning profile, delete `docs-demo-copy` and `docs-demo-base` after reviewing the confirmations, then delete the disposable `docs-demo-ready` snapshot. Verify they disappear from settings and that managed sidebar entries are removed after sync. Closing workspaces or logging out is not cloud cleanup.
