package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const helpText = `herdr-instabox — persistent InstaBox machines in Herdr

Usage: herdr-instabox [--account NAME] COMMAND
       herdr-instabox                         open settings

Getting started:
  setup [github|google]     Offer a safe starter config, then sign in
  doctor [--json]           Read-only local diagnostics (no API calls)
  help, --help, -h          Show this help

Accounts:
  account list             JSON profile list; active and UI-selected are distinct
  account add NAME         Add an isolated profile (does not sign in)
  login [github|google]     Sign in; GitHub defaults to device code without a display
  whoami                   JSON verified login and effective workspace (uses API)
  logout                   Revoke this sign-in and remove its local connections

Workspaces:
  ui [remotes|snapshots|account]   Open settings in this terminal
  open [snapshots|account]        Open a settings pane inside Herdr
  new-workspace [CWD]             Use the selected profile's default; never starts it
  sync [--if-stale]               Reconcile all profiles, or only explicit --account
  install-keys                   Add missing default keybindings

Examples (run in the installed plugin environment):
  herdr-instabox account add work
  herdr-instabox --account work setup google
  herdr-instabox --account work whoami
  herdr-instabox --account work sync

Herdr sets HERDR_PLUGIN_STATE_DIR and HERDR_PLUGIN_CONFIG_DIR. Outside Herdr,
set both to the installed plugin's ROOT directories before using --account.
Plugin sign-ins are separate from the InstaBox CLI. See docs/authentication.md.
Internal commands: job ID, ensure-cert ID DIR [CONFIG_DIR ACCOUNT].
`

type diagnosticCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type diagnosticReport struct {
	SchemaVersion int               `json:"schemaVersion"`
	Account       string            `json:"account"`
	GoVersion     string            `json:"goVersion"`
	Platform      string            `json:"platform"`
	StateDir      string            `json:"stateDir"`
	ConfigPath    string            `json:"configPath"`
	Checks        []diagnosticCheck `json:"checks"`
	OK            bool              `json:"ok"`
}

// Never call whoami, loadCredentials, listJobs or external commands here: those
// may refresh tokens, migrate state, prune jobs, or contact a remote service.
func localDiagnostics() diagnosticReport {
	r := diagnosticReport{SchemaVersion: 1, Account: activeAccount(), GoVersion: runtime.Version(),
		Platform: runtime.GOOS + "/" + runtime.GOARCH, StateDir: stateDir(), ConfigPath: configPath(), OK: true}
	add := func(name, status, message string) {
		r.Checks = append(r.Checks, diagnosticCheck{name, status, message})
		if status == "error" {
			r.OK = false
		}
	}
	for _, tool := range []string{herdrBin(), "ssh", "ssh-keygen"} {
		if _, err := exec.LookPath(tool); err != nil {
			add("executable", "error", tool+" not found; check Herdr's PATH")
		} else {
			add("executable", "ok", tool+" available (version not probed)")
		}
	}
	if _, err := readConfig(configPath()); err != nil {
		add("config", "error", err.Error())
	} else {
		add("config", "ok", "valid; an absent file uses defaults")
	}
	data, err := os.ReadFile(credentialsPath())
	switch {
	case os.IsNotExist(err):
		add("sign-in", "warning", "no saved sign-in; run setup or login")
	case err != nil:
		add("sign-in", "error", "cannot read credentials file; check ownership and permissions")
	default:
		var c Credentials
		if json.Unmarshal(data, &c) != nil || c.AccessToken == "" || c.RefreshToken == "" {
			add("sign-in", "error", "invalid credentials file; sign in again (contents withheld)")
		} else if time.Now().After(c.RefreshExpiresAt) {
			add("sign-in", "warning", "saved refresh token expired; sign in again")
		} else {
			add("sign-in", "ok", "saved credentials present; NOT verified online")
		}
	}
	if binding, err := readIdentity(identityPath(), false); err != nil {
		add("identity", "error", "invalid saved binding; see docs/troubleshooting.md")
	} else if binding.UserID == "" {
		add("identity", "warning", "no saved binding; first verified sign-in establishes it")
	} else {
		add("identity", "ok", "saved user/workspace binding present; use whoami to verify effective scope online")
	}
	if sshIncluded() {
		add("ssh-include", "ok", "this profile's Include line is present")
	} else {
		add("ssh-include", "warning", "this profile's Include line is absent; sign in through settings or run setup")
	}
	for _, path := range []string{stateDir(), credentialsPath(), keyPath(), identityPath(), configPath()} {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			add("permissions", "error", "cannot inspect "+path)
		} else if info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
			add("permissions", "warning", path+": expected a private, non-symlink file/directory (0600/0700)")
		} else {
			add("permissions", "ok", path+": private")
		}
	}
	return r
}

func doctor(out io.Writer, args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "--json") {
		return errors.New("usage: herdr-instabox doctor [--json]")
	}
	r := localDiagnostics()
	if len(args) == 1 {
		if err := json.NewEncoder(out).Encode(r); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(out, "Profile: %s | %s | %s\nState: %s\nConfig: %s\n", r.Account, r.Platform, r.GoVersion, r.StateDir, r.ConfigPath)
		for _, check := range r.Checks {
			fmt.Fprintf(out, "[%s] %s: %s\n", check.Status, check.Name, check.Message)
		}
		fmt.Fprintln(out, "Local checks only. No network requests, token refreshes, repairs or machine changes performed.")
	}
	if !r.OK {
		return errors.New("local checks failed; resolve the reported errors and run doctor again")
	}
	return nil
}

const starterConfig = "{\n  \"secrets\": [],\n  \"disableVault\": true\n}\n"

// setup never replaces an existing config or imports another profile's values.
func setup(in io.Reader, out io.Writer, provider string) error {
	if provider != "" && provider != "github" && provider != "google" {
		return errors.New("usage: herdr-instabox setup [github|google]")
	}
	fmt.Fprintf(out, "Set up profile %s\nConfig: %s\nState: %s\nOther profiles stay connected. The InstaBox CLI login is separate.\n", activeAccount(), configPath(), stateDir())
	if _, err := readConfig(configPath()); err != nil {
		return err
	}
	if _, err := os.Stat(configPath()); os.IsNotExist(err) {
		fmt.Fprint(out, "Create a starter config with no selected Secrets and Vault disabled? [y/N] ")
		answer, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		if err == io.EOF && strings.TrimSpace(answer) == "" {
			return errors.New("setup needs an interactive terminal; use login for non-interactive configuration")
		}
		if strings.EqualFold(strings.TrimSpace(answer), "y") || strings.EqualFold(strings.TrimSpace(answer), "yes") {
			if err := writeStarterConfig(); err != nil {
				return err
			}
		} else {
			fmt.Fprintln(out, "Keeping existing defaults; Vault delivery is not disabled by the plugin.")
		}
	} else if err != nil {
		return err
	} else {
		fmt.Fprintln(out, "Keeping your existing config unchanged.")
	}
	if err := login(provider); err != nil {
		return err
	}
	if err := addInclude(); err != nil {
		return err
	}
	fmt.Fprintln(out, "Ready. Open settings → remotes to create/start a machine, choose Use as default, then open a workspace. No cloud machine was created by setup.")
	return nil
}
