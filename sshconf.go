package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// certMargin is how long before expiry a certificate is refreshed.
const certMargin = 10 * time.Minute

func hostsDir() string { return filepath.Join(stateDir(), "ssh") }

func hostFile(id string) string { return filepath.Join(hostsDir(), id+".conf") }

func userSSHConfig() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

func includeLine() string {
	return fmt.Sprintf("Include %q", filepath.Join(hostsDir(), "*.conf"))
}

func sshIncluded() bool {
	data, err := os.ReadFile(userSSHConfig())
	if err != nil {
		return false
	}
	want := includeLine()
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// addInclude prepends the Include line to ~/.ssh/config. It must come first:
// an Include after a Host line would only apply inside that Host block.
func addInclude() error {
	return withRootLock("ssh-config.lock", addIncludeLocked)
}

func addIncludeLocked() error {
	if sshIncluded() {
		return nil
	}
	path := userSSHConfig()
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if len(old) > 0 {
		if err := os.WriteFile(path+".herdr-instabox.bak", old, 0o600); err != nil {
			return err
		}
	}
	return os.WriteFile(path, append([]byte(includeHeader()), old...), 0o600)
}

const includeComment = "# Added by the herdr instabox plugin."

func includeHeader() string { return includeComment + "\n" + includeLine() + "\n\n" }

// removeInclude takes out what addInclude added and leaves the rest untouched.
func removeInclude() error {
	return withRootLock("ssh-config.lock", removeIncludeLocked)
}

func removeIncludeLocked() error {
	path := userSSHConfig()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	text := string(data)
	if rest, ok := strings.CutPrefix(text, includeHeader()); ok {
		text = rest
	} else {
		var kept []string
		for _, line := range strings.Split(text, "\n") {
			if t := strings.TrimSpace(line); t != includeLine() && t != includeComment {
				kept = append(kept, line)
			}
		}
		text = strings.Join(kept, "\n")
	}
	if text == string(data) {
		return nil
	}
	return os.WriteFile(path, []byte(text), 0o600)
}

// writeHostBlock writes the machine's Host block, issuing a certificate for
// the plugin's own SSH key. A Match exec line in front renews the certificate
// before a connection when it is close to expiry.
func writeHostBlock(m Machine) error {
	cm, err := ensureCert(m.ID, true)
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	port := cm.Port
	if port == 0 {
		port = 22
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# instabox machine %s (%s), managed by herdr-instabox\n", m.Name, m.ID)
	fmt.Fprintf(&b, "Match host %s exec \"%s ensure-cert %s %s %s %s\"\n",
		m.Target(), shellQuote(self), m.ID, shellQuote(stateDir()), shellQuote(filepath.Dir(configPath())), shellQuote(activeAccount()))
	fmt.Fprintf(&b, "Host %s\n", m.Target())
	fmt.Fprintf(&b, "  HostName %s\n  Port %d\n  User %s\n", cm.Host, port, cm.Username)
	fmt.Fprintf(&b, "  IdentityFile %s\n  CertificateFile %s\n  IdentitiesOnly yes\n", keyPath(), certPath(m.ID))
	fmt.Fprintf(&b, "  UserKnownHostsFile %s\n  StrictHostKeyChecking yes\n  UpdateHostKeys no\n", knownHostsPath())
	return writeFileAtomic(hostFile(m.ID), []byte(b.String()), 0o600)
}

// hostBlockCurrent reports whether the machine's Host block names the gateway
// its latest certificate was issued for. A renewed certificate can come with a
// new gateway, and then the block is rewritten.
func hostBlockCurrent(id string) bool {
	block, err := os.ReadFile(hostFile(id))
	if err != nil {
		return false
	}
	context := "ensure-cert " + id + " " + shellQuote(stateDir()) + " " + shellQuote(filepath.Dir(configPath())) + " " + shellQuote(activeAccount())
	if !strings.Contains(string(block), context) {
		return false
	}
	data, err := os.ReadFile(certMetaPath(id))
	if err != nil {
		return true // nothing to compare against; the next renewal writes it
	}
	var cm certMeta
	if json.Unmarshal(data, &cm) != nil || cm.Host == "" {
		return true
	}
	port := cm.Port
	if port == 0 {
		port = 22
	}
	return strings.Contains(string(block), fmt.Sprintf("  HostName %s\n  Port %d\n", cm.Host, port))
}

func hasHostBlock(id string) bool {
	_, err := os.Stat(hostFile(id))
	return err == nil
}

func removeHostBlock(id string) {
	_ = os.Remove(hostFile(id))
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// runEnsureCert is `herdr-instabox ensure-cert ID STATEDIR`, which ssh runs
// (Match exec) before each connection. It renews the certificate only near
// expiry and always succeeds: a failed renewal surfaces as an SSH
// authentication error, which herdr already reports.
func runEnsureCert(id, dir string, context ...string) {
	if dir != "" {
		_ = os.Setenv("HERDR_PLUGIN_STATE_DIR", dir)
		name, root := personalAccount, dir
		if filepath.Base(filepath.Dir(dir)) == "accounts" && accountNamePattern.MatchString(filepath.Base(dir)) {
			name, root = filepath.Base(dir), filepath.Dir(filepath.Dir(dir))
		}
		configDir := accountsConfigRoot()
		if name != personalAccount {
			configDir = filepath.Join(configDir, "accounts", name)
		}
		if len(context) > 0 && context[0] != "" {
			configDir = context[0]
		}
		if len(context) > 1 && context[1] != "" {
			name = context[1]
		}
		_ = os.Setenv("HERDR_INSTABOX_ROOT_STATE_DIR", root)
		_ = os.Setenv("HERDR_INSTABOX_ACCOUNT", name)
		_ = os.Setenv("HERDR_PLUGIN_CONFIG_DIR", configDir)
	}
	// The host CLI's selected server/account never retargets a saved SSH host.
	_ = os.Unsetenv("INSTABOX_SERVER")
	_ = os.Unsetenv("INSTABOX_API_URL")
	if _, err := readConfig(configPath()); err != nil {
		_ = os.Remove(certPath(id))
		return
	}
	if _, err := ensureCert(id, false); err != nil {
		// Match exec failure alone would not stop SSH using an old cached cert.
		// Remove only this certificate, leaving other accounts and open sessions alone.
		_ = os.Remove(certPath(id))
	}
}
