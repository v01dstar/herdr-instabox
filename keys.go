package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// keyBinding is a default key for one of the plugin's actions. herdr's manifest
// cannot declare keys, so install-keys writes them into herdr's config.toml.
type keyBinding struct {
	Key, Action, Description string
}

var defaultKeys = []keyBinding{
	{"prefix+m", "open", "instabox settings"},
	{"prefix+shift+m", "new-workspace", "new workspace on the default machine"},
}

func herdrConfigPath() string {
	if path := os.Getenv("HERDR_CONFIG_PATH"); path != "" {
		return path
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "herdr", "config.toml")
}

// tomlString matches `name = "value"` on its own line, with either quote style.
func tomlString(name, value string) *regexp.Regexp {
	v := regexp.QuoteMeta(value)
	return regexp.MustCompile(`(?m)^\s*` + name + `\s*=\s*(?:"` + v + `"|'` + v + `')\s*(?:#.*)?$`)
}

// installKeys adds the default bindings to herdr's config.toml. An action that
// already has a binding, or a key that is already bound, is left alone, so it
// is safe to run again and never overrides the user's own keys.
func installKeys() error {
	path := herdrConfigPath()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	config := string(data)
	var add strings.Builder
	for _, k := range defaultKeys {
		command := pluginID() + "." + k.Action
		switch {
		case tomlString("command", command).MatchString(config):
			fmt.Printf("%s already has a key binding\n", command)
		case tomlString("key", k.Key).MatchString(config):
			fmt.Printf("%s is already bound; bind %s yourself\n", k.Key, command)
		default:
			fmt.Fprintf(&add, "\n[[keys.command]]\nkey = %q\ntype = \"plugin_action\"\ncommand = %q\ndescription = %q\n",
				k.Key, command, k.Description)
			fmt.Printf("bound %s to %s\n", k.Key, command)
		}
	}
	if add.Len() == 0 {
		return nil
	}
	if config != "" && !strings.HasSuffix(config, "\n") {
		config += "\n"
	}
	config += add.String()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp := path + ".herdr-instabox.tmp"
	if err := os.WriteFile(tmp, []byte(config), mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	// A running herdr does not watch its config; without a server this fails
	// and the keys apply when herdr next starts.
	_, _ = herdr("server", "reload-config")
	return nil
}
