# Credentials belong to the selected image

Copy [config.json](config.json) to the intended account’s config file after backing it up. Replace the two placeholder snapshot IDs with actual IDs and use Secret names from that account’s workspace. The paired name/ID entries cover configured default creates and UI-selected snapshots.

| Source | Explicit Secret selection |
| --- | --- |
| `work-ready` or its ID | `WORK_MODEL_KEY`, `WORK_GH_TOKEN` |
| `generic-clean` or its ID | None, even though a global default exists |
| Any unmatched source | `DEFAULT_MODEL_KEY` |

Open settings → snapshots → select the intended snapshot → **New machine from snapshot…**. Verify the resulting machine’s configuration through InstaBox without printing credential values. The work selection replaces the global list, rather than appending to it. `disableVault` separately prevents unrelated Vault delivery.

**This is not image sanitization.** An empty list does not remove baked-in credentials or global plain `env`. Audit the snapshot before using it as a clean image. Replacing a snapshot requires updating its ID entry. Changing config affects new machines only.

Cleanup: delete test machines explicitly in the owning profile and restore the prior config. Do not delete a shared snapshot without checking who uses it.
