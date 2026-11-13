package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// WorkspaceTarget is a repository the plugin can open a workspace in.
type WorkspaceTarget struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Cwd   string `json:"cwd"`
}

// SnapshotProfile keeps credentials scoped to the selected image, never global.
type SnapshotProfile struct {
	Secrets  []string          `json:"secrets"`
	Metadata map[string]string `json:"metadata"`
}

// Config lets a fork or an unusual deployment point the plugin at a different
// machine template, snapshot set, Secrets and workspace directories without
// changing code. It lives in HERDR_PLUGIN_CONFIG_DIR/config.json (Herdr sets
// that variable for plugin commands) and every field is optional; without it
// the plugin behaves exactly as before (the herdr template, no Secrets/env).
type Config struct {
	// Identity optionally overrides the user/workspace learned from sign-in.
	Identity Identity `json:"identity"`
	// SnapshotProfiles is keyed by snapshot ID (UI) or name (default create).
	SnapshotProfiles map[string]SnapshotProfile `json:"snapshotProfiles"`
	// DisableVault prevents unrelated personal Vault items reaching these boxes.
	DisableVault bool `json:"disableVault"`
	// Templates are InstaBox template ids the plugin manages. "*" matches all.
	Templates []string `json:"templates"`
	// SnapshotPrefixes additionally accept snapshots whose names start with one
	// of these, regardless of template. Useful for a prepared base snapshot.
	SnapshotPrefixes []string `json:"snapshotPrefixes"`
	// Metadata selects machines by label in addition to Templates.
	Metadata map[string]string `json:"metadata"`
	// Secrets are org Secret names selected on every created machine.
	Secrets []string `json:"secrets"`
	// Env are plain environment variables set on every created machine.
	Env map[string]string `json:"env"`
	// EnvFile is a .env file read at create time; Env wins on collisions.
	EnvFile string `json:"envFile"`
	// DefaultTemplate is used when a create names no template. Defaults to the
	// first entry in Templates.
	DefaultTemplate string `json:"defaultTemplate"`
	// DefaultSnapshot is used when a create names neither a template nor a
	// snapshot. Empty keeps the template default.
	DefaultSnapshot string `json:"defaultSnapshot"`
	// DefaultCwd is where New workspace opens. Empty keeps Herdr's default.
	DefaultCwd string `json:"defaultCwd"`
	// Workspaces documents per-repo directories (the UI and docs can show them;
	// Herdr keybindings can call `new-workspace <cwd>` directly).
	Workspaces []WorkspaceTarget `json:"workspaces"`
}

func configPath() string {
	if dir := os.Getenv("HERDR_PLUGIN_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "config.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "herdr", "plugins", "config", "v01dstar.instabox", "config.json")
}

// readConfig rejects mistakes before defaults or credential selections are used.
// Missing configuration is valid; an existing file must contain one JSON object.
func readConfig(path string) (Config, error) {
	var c Config
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return c, fmt.Errorf("read config %s: %w", path, err)
	}
	if err == nil {
		if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
			return c, fmt.Errorf("config %s must contain a JSON object", path)
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&c); err != nil {
			return Config{}, fmt.Errorf("invalid config %s: %w", path, err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return Config{}, fmt.Errorf("config %s must contain exactly one JSON object", path)
		}
	}
	if c.Identity.Server != "" {
		u, err := url.Parse(c.Identity.Server)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return Config{}, fmt.Errorf("config %s: identity.server must be an HTTP(S) issuer URL without credentials, query or fragment", path)
		}
	}
	if len(c.Templates) == 0 {
		c.Templates = []string{HerdrTemplate}
	}
	if c.DefaultTemplate == "" {
		c.DefaultTemplate = c.Templates[0]
	}
	if c.Env == nil {
		c.Env = map[string]string{}
	}
	if c.EnvFile != "" {
		values, err := readEnvFile(c.EnvFile)
		if err != nil {
			return Config{}, fmt.Errorf("config %s: envFile cannot be read: %w", path, err)
		}
		for k, v := range values {
			if _, ok := c.Env[k]; !ok {
				c.Env[k] = v
			}
		}
	}
	if c.DefaultCwd == "" && len(c.Workspaces) > 0 {
		c.DefaultCwd = c.Workspaces[0].Cwd
	}
	return c, nil
}

func readEnvFile(path string) (map[string]string, error) {
	out := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		out[k] = v
	}
	return out, nil
}

// matchesMachine reports whether a machine belongs to this plugin's fleet.
func (c Config) matchesMachine(m Machine) bool {
	if len(c.Metadata) > 0 {
		ok := true
		for k, v := range c.Metadata {
			if m.Metadata[k] != v {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	for _, t := range c.Templates {
		if t == "*" || (m.Template != nil && m.Template.ID == t) {
			return true
		}
	}
	return false
}

// matchesSnapshot reports whether a snapshot is a valid base for this fleet.
func (c Config) matchesSnapshot(s Snapshot) bool {
	for _, p := range c.SnapshotPrefixes {
		if p != "" && strings.HasPrefix(s.Name, p) {
			return true
		}
	}
	if s.Template != nil {
		for _, t := range c.Templates {
			if t == "*" || s.Template.ID == t {
				return true
			}
		}
	}
	return false
}
