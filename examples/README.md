# Examples

These recipes are for this fork and its stable plugin ID `v01dstar.instabox`. JSON files contain **Secret names**, never tokens. Replace example names/paths with resources in your own InstaBox workspace.

| Recipe | Files | Outcome |
| --- | --- | --- |
| [Minimal](minimal/README.md) | Starter config | No selected Secrets; Vault disabled |
| [Dev machine](dev-machine/README.md) | End-to-end walkthrough | Prepare, snapshot, reuse, and clean up |
| [Workspaces](workspaces/README.md) | Custom fleet config | Create from prepared images and open the right directory |
| [Snapshot credentials](snapshot-credentials/README.md) | Snapshot profiles | Work image gets work Secrets; generic image selects none |
| [Multiple accounts](multiple-accounts/README.md) | Personal and work configs | Both connected, separate defaults and credentials |

`CONFIG` means the root printed by `herdr plugin config-dir v01dstar.instabox`. `STATE` means the actual installed state root supplied by Herdr. For CLI commands from a built checkout, first [set the environment](../docs/authentication.md#cli-environment-get-the-roots-right).

Back up existing config files before copying examples. Examples are not merged automatically. Make config directories private (`0700`) and files private (`0600`). Tests decode every shipped example using the production config reader; they do not prove your named snapshots or Secrets exist online.
