# Prepared images and multiple repositories

Copy [config.json](config.json) into the intended profile’s config after backing up the existing file. Replace `agent-browser` and `team-ready` with a template/snapshot available in that profile’s workspace, or remove `defaultSnapshot` to create from a template. A configured default snapshot is used ahead of the template; a missing snapshot produces an API error, not an automatic fallback.

This widens machine discovery to either listed template **or** matching metadata. It also offers `team-` snapshots. It does not gain access to other accounts or workspaces.

On the remote machine, create the directories and clone the projects there. Choose **Use as default** for the running machine, then from the installed plugin environment:

```bash
bin/herdr-instabox --account work new-workspace
bin/herdr-instabox --account work new-workspace /data/workspace/web
```

Expected: the first workspace starts in `/data/workspace/api`; the second uses `/data/workspace/web`. Select them in the sidebar. `workspaces` documents directories and supplies the first default if `defaultCwd` is absent—it is not an implemented repository chooser.

Cleanup: close the test workspaces, stop/delete any test cloud machine you created, and restore the configuration backup. Removing config alone does not stop a machine.
