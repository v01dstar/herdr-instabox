package main

// Each account runs in its own process context. Never switch os.Environ while
// UI commands are in flight: requests, jobs and certificate renewals must keep
// using the identity that admitted them.
import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

const personalAccount = "personal"

var accountNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func activeAccount() string {
	if name := os.Getenv("HERDR_INSTABOX_ACCOUNT"); name != "" {
		return name
	}
	return personalAccount
}

func accountsRoot() string {
	if root := os.Getenv("HERDR_INSTABOX_ROOT_STATE_DIR"); root != "" {
		return root
	}
	return stateDir()
}

func accountsConfigRoot() string {
	if root := os.Getenv("HERDR_INSTABOX_ROOT_CONFIG_DIR"); root != "" {
		return root
	}
	return filepath.Dir(configPath())
}

func accountDir(name string) string {
	if name == personalAccount {
		return accountsRoot() // Keep the existing sign-in, key and SSH targets.
	}
	return filepath.Join(accountsRoot(), "accounts", name)
}

func selectedAccount() string {
	data, err := os.ReadFile(filepath.Join(accountsRoot(), "selected-account"))
	name := strings.TrimSpace(string(data))
	if err == nil && accountNamePattern.MatchString(name) {
		if names, err := accountNames(); err == nil {
			for _, candidate := range names {
				if candidate == name {
					return name
				}
			}
		}
	}
	return personalAccount
}

func selectAccount(name string) error {
	names, err := accountNames()
	if err != nil {
		return err
	}
	for _, candidate := range names {
		if candidate == name {
			return writeFileAtomic(filepath.Join(accountsRoot(), "selected-account"), []byte(name+"\n"), 0o600)
		}
	}
	return fmt.Errorf("unknown account %q", name)
}

func accountNames() ([]string, error) {
	names := []string{personalAccount}
	entries, err := os.ReadDir(filepath.Join(accountsRoot(), "accounts"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() && e.Type()&os.ModeSymlink == 0 && e.Name() != personalAccount && accountNamePattern.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names[1:])
	return names, nil
}

func addAccount(name string) error {
	if !accountNamePattern.MatchString(name) {
		return fmt.Errorf("account name must be 1–32 lowercase letters, digits, _ or -, starting with a letter")
	}
	if name == personalAccount {
		return fmt.Errorf("account %q already exists (the original plugin sign-in)", name)
	}
	dir := accountDir(name)
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return err
	}
	return os.Mkdir(dir, 0o700) // Refuse duplicates; never replace credentials.
}

// prepareAccount is called once, before any asynchronous commands start.
func prepareAccount(name string) error {
	if !accountNamePattern.MatchString(name) {
		return fmt.Errorf("invalid account name %q", name)
	}
	root, configRoot := accountsRoot(), accountsConfigRoot()
	dir, configDir := root, configRoot
	if name != personalAccount {
		dir = filepath.Join(root, "accounts", name)
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unknown account %q; run `herdr-instabox account add %s` first", name, name)
		}
		configDir = filepath.Join(configRoot, "accounts", name)
	}
	for key, value := range map[string]string{
		"HERDR_INSTABOX_ROOT_STATE_DIR":  root,
		"HERDR_INSTABOX_ROOT_CONFIG_DIR": configRoot,
		"HERDR_INSTABOX_ACCOUNT":         name,
		"HERDR_PLUGIN_STATE_DIR":         dir,
		"HERDR_PLUGIN_CONFIG_DIR":        configDir,
	} {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return nil
}

func managedTargetPrefix() string {
	if activeAccount() == personalAccount {
		return TargetPrefix
	}
	name := activeAccount()
	return TargetPrefix + "account-" + strconv.Itoa(len(name)) + "-" + name + "-"
}

func accountCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: herdr-instabox account list | add NAME")
	}
	switch args[0] {
	case "list":
		names, err := accountNames()
		if err != nil {
			return err
		}
		var rows []map[string]any
		for _, name := range names {
			_, err := os.Stat(filepath.Join(accountDir(name), "credentials.json"))
			rows = append(rows, map[string]any{"name": name, "active": name == activeAccount(), "selected": name == selectedAccount(), "hasSignIn": err == nil})
		}
		return json.NewEncoder(os.Stdout).Encode(rows)
	case "add":
		name := ""
		if len(args) > 1 {
			name = args[1]
		} else {
			fmt.Print("New account profile name (e.g. work): ")
			name, _ = bufio.NewReader(os.Stdin).ReadString('\n')
			name = strings.TrimSpace(name)
		}
		if err := addAccount(name); err != nil {
			return err
		}
		fmt.Printf("Added %s. Use [ / ] in settings to select it, then sign in; or run herdr-instabox --account %s login.\n", name, name)
		return nil
	}
	return fmt.Errorf("unknown account command %q", args[0])
}

// Sync other profiles in subprocesses. Environment, refresh locks, credentials,
// jobs and SSH keys are never shared or swapped inside this process.
func syncOtherAccounts(ifStale bool) error {
	names, err := accountNames()
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	var errs []error
	for _, name := range names {
		if name == activeAccount() {
			continue
		}
		args := []string{"--account", name, "sync"}
		if ifStale {
			args = append(args, "--if-stale")
		}
		cmd := exec.Command(self, args...)
		// Global CLI server overrides must not retarget another account's auth.
		cmd.Env = withoutServerOverrides(os.Environ())
		out, err := cmd.CombinedOutput()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %s", name, strings.TrimSpace(string(out))))
		}
	}
	return errors.Join(errs...)
}

func withoutServerOverrides(env []string) []string {
	var out []string
	for _, entry := range env {
		if !strings.HasPrefix(entry, "INSTABOX_SERVER=") && !strings.HasPrefix(entry, "INSTABOX_API_URL=") && !strings.HasPrefix(entry, "INSTABOX_API_KEY=") {
			out = append(out, entry)
		}
	}
	return out
}

func restartAccountUI(name, tab string) error {
	if err := selectAccount(name); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(self, []string{self, "--account", name, "ui", tab}, withoutServerOverrides(os.Environ()))
}
