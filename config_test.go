package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStrictConfig(t *testing.T) {
	for _, input := range []string{
		``, `null`, `[]`, `{"secrets":`, `{"secret": ["typo"]}`,
		`{"disableVault": "true"}`, `{"snapshotProfiles":{"clean":{"secret":[]}}}`,
		`{"identity":{"workspace":"typo"}}`, `{} {}`, `{} trailing`,
		`{"identity":{"server":"ftp://example.com"}}`,
		`{"identity":{"server":"https://user:password@example.com"}}`,
	} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := readConfig(path); err == nil {
				t.Fatalf("accepted invalid config %q", input)
			}
		})
	}
}

func TestMissingConfigAndEnvFilePrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg, err := readConfig(path)
	if err != nil || cfg.DefaultTemplate != "herdr" || len(cfg.Secrets) != 0 {
		t.Fatalf("missing config defaults: %+v, %v", cfg, err)
	}
	newFake(t)
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte(`{"envFile":"`+filepath.Join(dir, "missing.env")+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfig(configPath()); err == nil || !strings.Contains(err.Error(), "envFile") {
		t.Fatalf("unreadable envFile accepted: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "missing.env"), []byte("FROM_FILE=ok\nCOLLISION=file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte(`{"envFile":"`+filepath.Join(dir, "missing.env")+`","env":{"COLLISION":"explicit"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = readConfig(configPath())
	if err != nil || cfg.Env["FROM_FILE"] != "ok" || cfg.Env["COLLISION"] != "explicit" {
		t.Fatalf("env precedence: %+v, %v", cfg.Env, err)
	}
}

func TestShippedExampleConfigs(t *testing.T) {
	count := 0
	err := filepath.WalkDir("examples", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".json") {
			count++
			cfg, err := readConfig(path)
			if err != nil {
				t.Errorf("%s: %v", path, err)
			}
			if !cfg.DisableVault {
				t.Errorf("%s: examples must explicitly disable Vault", path)
			}
		}
		return nil
	})
	if err != nil || count < 5 {
		t.Fatalf("example coverage: %d files, %v", count, err)
	}
}

func TestConfigEditBlocksCreateAndLists(t *testing.T) {
	newFake(t)
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte(`{"secret":["bad"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := createMachine("must-not-create", "", ""); err == nil {
		t.Fatal("create ignored broken configuration")
	}
	if _, err := listMachines(); err == nil {
		t.Fatal("machine list ignored broken configuration")
	}
	if _, err := listSnapshots(); err == nil {
		t.Fatal("snapshot list ignored broken configuration")
	}
}
