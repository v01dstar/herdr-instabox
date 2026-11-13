# Security and trust

## Reporting a vulnerability

Do not post credentials or exploitable details in a public issue. Use [GitHub private vulnerability reporting](https://github.com/mdp/herdr-instabox/security/advisories/new) if enabled; otherwise contact the repository maintainer privately to arrange a secure channel. Include affected commits, reproduction steps using disposable accounts, and a redacted impact description. No response-time guarantee or external security audit is implied.

## Boundaries

Installing a Herdr plugin runs code as your local OS user. Review the manifest and source before installing an untrusted plugin. InstaBox supplies the remote-machine boundary; this plugin is not itself a sandbox or defense against another process running as your local user.

Profiles isolate sign-ins, SSH credentials, local jobs/state, and configuration to prevent accidental account mixing. Verified identity/workspace guards do not grant permissions or replace server authorization. Shared local Herdr/SSH files are coordinated by locks, not separate OS users.

## Credentials

- The plugin stores its own OAuth sign-in separately from the standalone InstaBox CLI.
- Account directories are created private (`0700`); credentials, keys, binding/state/config files created by the plugin are private (`0600`). Existing user-managed files should be checked with `doctor`.
- SSH uses a plugin-owned key and short-lived certificates; renewal uses the owning profile’s config/credentials.
- Secret names in config select **delivery into a new cloud machine**. Treat those credentials as accessible to processes on that machine. Use scoped, revocable credentials.
- `env` and `envFile` are plain environment delivery, not an encrypted credential store. Do not commit values or publish diagnostics containing them.
- Empty Secret selection and Vault disabling do not remove secrets already present in a snapshot. Audit images before sharing/reuse.
- Open SSH sessions are not killed when certificates expire or a later identity check fails.

`doctor` never prints credential contents or env values and performs no online verification. It does report local paths and configuration error messages; review before sharing. Raw job logs and cloud API errors should also be treated as potentially sensitive.

## Destruction and billing

Use the UI’s confirmations for delete/copy/stop operations. Closing the pane does not cancel background work. After an unknown outcome, inspect actual cloud state before retrying a create/copy. Logout removes the profile’s managed local connections and revokes its sign-in; it does not stop or delete cloud resources, and costs may continue.
