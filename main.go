// herdr-instabox is a herdr plugin that manages instabox machines and keeps them in
// herdr's saved SSH machines, so they appear in the sidebar like any remote.
//
//	herdr-instabox [ui]                   the remotes/snapshots/account settings pane
//	herdr-instabox open [tab]             open the settings pane (plugin action)
//	herdr-instabox new-workspace [CWD]    new workspace on the default machine
//	herdr-instabox sync [--if-stale]      reconcile herdr with instabox (hooks)
//	herdr-instabox job ID                 run a background job (internal)
//	herdr-instabox login [github|google] sign in (run in the pane's terminal)
//	herdr-instabox ensure-cert ID DIR     renew a certificate (ssh Match exec)
//	herdr-instabox install-keys          add default key bindings to herdr's config
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	args := os.Args[1:]
	explicitAccount := false
	name := activeAccount()
	if len(args) > 0 && args[0] == "--account" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "herdr-instabox: --account needs a profile name")
			os.Exit(2)
		}
		name, explicitAccount, args = args[1], true, args[2:]
	}
	if !explicitAccount && os.Getenv("HERDR_INSTABOX_ACCOUNT") == "" &&
		(len(args) == 0 || args[0] == "ui" || args[0] == "open" || args[0] == "new-workspace") {
		name = selectedAccount()
	}
	if err := prepareAccount(name); err != nil {
		fmt.Fprintln(os.Stderr, "herdr-instabox:", err)
		os.Exit(1)
	}
	os.Args = append([]string{os.Args[0]}, args...)
	cmd := "ui"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	arg := func(i int) string {
		if len(os.Args) > i {
			return os.Args[i]
		}
		return ""
	}
	var err error
	switch cmd {
	case "ui":
		if err = selectAccount(activeAccount()); err != nil {
			break
		}
		var result tea.Model
		result, err = tea.NewProgram(newModel(arg(2)), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
		if m, ok := result.(model); err == nil && ok && m.nextAccount != "" {
			err = restartAccountUI(m.nextAccount, tabNames[m.tab])
		}
	case "account":
		err = accountCommand(os.Args[2:])
	case "whoami":
		a := whoami()
		if a.Err != nil {
			err = a.Err
		} else {
			var scope Identity
			if a.SignedIn() {
				scope, err = currentIdentity()
				if err != nil {
					break
				}
			}
			err = json.NewEncoder(os.Stdout).Encode(map[string]any{"account": activeAccount(), "login": a.Login, "userId": a.UserID, "server": a.Server, "orgId": scope.OrgID, "signedIn": a.SignedIn()})
		}
	case "logout":
		err = errors.Join(cleanupLocal(), signOut())
	case "open":
		if err = selectAccount(activeAccount()); err == nil {
			err = openPane(arg(2))
		}
	case "new-workspace":
		err = newWorkspace(arg(2))
	case "sync":
		if arg(2) == "--if-stale" {
			err = syncIfStale()
		} else {
			err = syncAll().Err
		}
		if !explicitAccount {
			err = errors.Join(err, syncOtherAccounts(arg(2) == "--if-stale"))
		}
	case "job":
		err = runJob(arg(2))
	case "ensure-cert":
		runEnsureCert(arg(2), arg(3), arg(4), arg(5))
	case "login":
		err = login(arg(2))
	case "install-keys":
		err = installKeys()
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "herdr-instabox:", err)
		os.Exit(1)
	}
}

func pluginID() string {
	if id := os.Getenv("HERDR_PLUGIN_ID"); id != "" {
		return id
	}
	return "v01dstar.instabox"
}

func openPane(tab string) error {
	entry := "ui"
	if tab == "account" || tab == "snapshots" {
		entry = tab
	}
	cmd := exec.Command(herdrBin(), "plugin", "pane", "open",
		"--plugin", pluginID(), "--entrypoint", entry, "--focus")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// newWorkspace opens a workspace on the default machine, in cwd when one is
// given (or the configured DefaultCwd). It never starts a machine; when the
// default is not connected it explains why instead.
func newWorkspace(cwd ...string) error {
	st := readState()
	target, label, problem := defaultTarget(st)
	if problem != "" {
		notify("New workspace", problem)
		return nil
	}
	dir := ""
	if len(cwd) > 0 {
		dir = cwd[0]
	}
	if dir == "" {
		dir = loadConfig().DefaultCwd
	}
	args := []string{"workspace", "create"}
	if dir != "" {
		args = append(args, "--cwd", dir)
	}
	args = append(args, "--focus")
	if target != "" {
		args = append([]string{"--machine", target}, args...)
	}
	where := "this computer"
	if label != "" {
		where = label
	}
	if _, err := herdr(args...); err != nil {
		notify("New workspace", fmt.Sprintf("Could not create a workspace on %s: %v", where, err))
		return nil
	}
	if target != "" {
		// Plugins cannot switch the client to another machine.
		notify("New workspace", fmt.Sprintf("Workspace created on %s; select it in the sidebar.", where))
	}
	return nil
}

// defaultTarget resolves the default location to a herdr machine profile ID
// ("" for Local), or explains why it cannot be used now.
func defaultTarget(st State) (target, label, problem string) {
	if st.Default == "" {
		return "", "", ""
	}
	all, err := listProfiles()
	if err != nil {
		return "", "", err.Error()
	}
	managed, ssh := splitProfiles(all)
	if id, ok := strings.CutPrefix(st.Default, "instabox:"); ok {
		label = st.Labels[id]
		p, has := managed[id]
		if label == "" && has {
			label = p.Label
		}
		if label == "" {
			label = id
		}
		switch {
		case st.Hidden[id]:
			return "", label, fmt.Sprintf("Default machine %s is hidden from the sidebar — show it in instabox settings, or choose another default.", label)
		case !has || !p.Enabled:
			return "", label, fmt.Sprintf("Default machine %s isn't connected — start it in instabox settings, or choose another default.", label)
		}
		return p.ID, label, ""
	}
	if id, ok := strings.CutPrefix(st.Default, "ssh:"); ok {
		for _, p := range ssh {
			if p.ID == id {
				if !p.Enabled {
					return "", p.Label, fmt.Sprintf("Default machine %s isn't connected — enable it in instabox settings, or choose another default.", p.Label)
				}
				return p.ID, p.Label, ""
			}
		}
	}
	return "", "", ""
}
