# Minimal, explicit starter

Copy [config.json](config.json) to `CONFIG/config.json` for personal, or `CONFIG/accounts/work/config.json` for an existing work profile. Guided `setup` can create this same file without overwriting an existing config.

```bash
bin/herdr-instabox --account personal setup github
bin/herdr-instabox --account personal doctor
bin/herdr-instabox --account personal whoami
```

Expected: no selected InstaBox Secrets, Vault disabled for new machines, and verified personal identity/workspace. The default template is `herdr`. No cloud machine is created by these commands.

Use settings → remotes → **+ Add remote** to create a machine. Cloud resources are not deleted when you close settings or log out; use the machine’s stop/delete action for cleanup. To undo only this example’s config, restore your backup; do not delete the account’s state or identity binding.
