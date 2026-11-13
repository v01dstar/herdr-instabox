package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var data []byte
		if !entry.IsDir() {
			data, err = os.ReadFile(path)
		}
		files[path] = fmt.Sprintf("%v %v %s", info.Mode(), info.ModTime(), data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestDoctorIsReadOnlyAndRedacted(t *testing.T) {
	f := newFake(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	if err := saveCredentials(&Credentials{Server: server.URL, AccessToken: "NEVER_PRINT_ACCESS", RefreshToken: "NEVER_PRINT_REFRESH", AccessExpiresAt: time.Now().Add(-time.Hour), RefreshExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte(`{"env":{"TOKEN":"NEVER_PRINT_ENV"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, f.dir)
	var out bytes.Buffer
	if err := doctor(&out, []string{"--json"}); err != nil {
		t.Fatal(err)
	}
	var report diagnosticReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.SchemaVersion != 1 || !report.OK || report.Account != "personal" {
		t.Fatalf("report: %s, %v", out.String(), err)
	}
	if strings.Contains(out.String(), "NEVER_PRINT") || calls.Load() != 0 {
		t.Fatal("diagnostics leaked a value or contacted the API")
	}
	if !reflect.DeepEqual(before, snapshotTree(t, f.dir)) {
		t.Fatal("diagnostics modified local files")
	}
}

func TestDoctorReportsBrokenConfigWithoutCredentials(t *testing.T) {
	newFake(t)
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte(`{"typo":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credentialsPath(), []byte(`{"accessToken":"NEVER_PRINT_BROKEN"`), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := doctor(&out, nil); err == nil || !strings.Contains(out.String(), "[error] config") || strings.Contains(out.String(), "NEVER_PRINT") {
		t.Fatalf("unsafe/missing diagnostic: %s, %v", out.String(), err)
	}
	if err := doctor(&out, []string{"--repair"}); err == nil {
		t.Fatal("accepted unsupported diagnostic flag")
	}
}

func TestStarterConfigNeverOverwrites(t *testing.T) {
	newFake(t)
	if err := writeStarterConfig(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(configPath())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions: %v, %v", info, err)
	}
	cfg, err := readConfig(configPath())
	if err != nil || !cfg.DisableVault || len(cfg.Secrets) != 0 {
		t.Fatalf("unsafe starter config: %+v, %v", cfg, err)
	}
	if err := writeStarterConfig(); !os.IsExist(err) {
		t.Fatalf("existing config was not protected: %v", err)
	}
	data, _ := os.ReadFile(configPath())
	if string(data) != starterConfig {
		t.Fatal("existing config changed")
	}
}

func TestSetupBrowserFlowKeepsExistingConfig(t *testing.T) {
	newFake(t)
	if err := writeStarterConfig(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DISPLAY", ":test")
	t.Setenv("SSH_CONNECTION", "")
	old := openBrowser
	openBrowser = func(url string) error {
		go func() {
			if response, err := http.Get(url); err == nil {
				response.Body.Close()
			}
		}()
		return nil
	}
	t.Cleanup(func() { openBrowser = old })
	var out bytes.Buffer
	if err := setup(strings.NewReader(""), &out, "github"); err != nil {
		t.Fatal(err)
	}
	if !sshIncluded() || !strings.Contains(out.String(), "existing config unchanged") {
		t.Fatalf("setup did not preserve config/integrate SSH: %s", out.String())
	}
	data, _ := os.ReadFile(configPath())
	if string(data) != starterConfig {
		t.Fatal("setup replaced existing config")
	}
}

func TestSetupCreatesSafeConfigOnConsent(t *testing.T) {
	newFake(t)
	t.Setenv("DISPLAY", ":test")
	t.Setenv("SSH_CONNECTION", "")
	old := openBrowser
	openBrowser = func(url string) error {
		go func() {
			if response, err := http.Get(url); err == nil {
				response.Body.Close()
			}
		}()
		return nil
	}
	t.Cleanup(func() { openBrowser = old })
	var out bytes.Buffer
	if err := setup(strings.NewReader("yes\n"), &out, "github"); err != nil {
		t.Fatal(err)
	}
	cfg, err := readConfig(configPath())
	if err != nil || !cfg.DisableVault || len(cfg.Secrets) != 0 || !sshIncluded() {
		t.Fatalf("setup did not create safe configuration: %+v, %v", cfg, err)
	}
}

func TestSetupNeedsInputAndValidProvider(t *testing.T) {
	newFake(t)
	var out bytes.Buffer
	if err := setup(strings.NewReader(""), &out, "github"); err == nil {
		t.Fatal("setup silently continued without input")
	}
	if err := setup(strings.NewReader("yes\n"), &out, "invalid"); err == nil {
		t.Fatal("accepted invalid setup provider")
	}
	if err := login("invalid"); err == nil {
		t.Fatal("accepted invalid login provider")
	}
	if _, err := os.Stat(configPath()); !os.IsNotExist(err) {
		t.Fatal("failed setup created configuration")
	}
}

func TestCLIHelpAndDoctorWithBrokenConfig(t *testing.T) {
	newFake(t)
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte(`broken`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--help"}, {"--account", "personal", "help"}} {
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "HERDR_INSTABOX_TEST_CLI=1")
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "doctor [--json]") {
			t.Fatalf("help failed: %s, %v", out, err)
		}
	}
	cmd := exec.Command(os.Args[0], "doctor", "--json")
	cmd.Env = append(os.Environ(), "HERDR_INSTABOX_TEST_CLI=1")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err == nil {
		t.Fatal("doctor did not exit nonzero for broken config")
	}
	var report diagnosticReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil || report.OK {
		t.Fatalf("doctor failed to emit JSON before failure: %s, %v", stdout.String(), err)
	}
}

func TestCreateHintUsesAccountAndSnapshotSelection(t *testing.T) {
	newFake(t)
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte(`{"secrets":["GLOBAL_KEY"],"disableVault":true,"defaultSnapshot":"generic-clean","snapshotProfiles":{"generic-clean":{"secrets":[]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newModel("remotes")
	m.acct = Account{Login: "tester", OrgID: "org_personal"}
	result, _ := m.openAdd("")
	m = result.(model)
	hint := m.addHint(m.dialog.form)
	for _, expected := range []string{"Profile personal", "workspace org_personal", "Secrets: none selected", "Vault: disabled", "generic-clean"} {
		if !strings.Contains(hint, expected) {
			t.Errorf("create hint missing %q: %s", expected, hint)
		}
	}
	if strings.Contains(hint, "GLOBAL_KEY") {
		t.Fatal("hint inherited global Secrets despite empty snapshot selection")
	}
}

func TestOtherProfileSyncSurvivesBrokenPersonalConfig(t *testing.T) {
	f := fakeAccounts(t)
	w := &fakeEnv{t: t, dir: t.TempDir()}
	w.api, w.server = startFakeAPI(t, w.dir)
	w.writeInstabox(fakeInstabox{SignedIn: true, Machines: []map[string]any{fakeMachine("m_work", "work-box", "running")}})
	if err := prepareAccount("work"); err != nil {
		t.Fatal(err)
	}
	if err := saveCredentials(&Credentials{Server: w.server, AccessToken: "work-at", AccessExpiresAt: time.Now().Add(time.Hour), RefreshToken: "work-rt"}); err != nil {
		t.Fatal(err)
	}
	if err := prepareAccount(personalAccount); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte(`broken`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "sync")
	cmd.Env = append(os.Environ(), "HERDR_INSTABOX_TEST_CLI=1")
	if err := cmd.Run(); err == nil {
		t.Fatal("sync hid personal config error")
	}
	if len(f.herdr().Profiles) != 1 {
		t.Fatal("personal config error prevented work sync")
	}
	if err := prepareAccount("work"); err != nil {
		t.Fatal(err)
	}
	workDir, workConfig, workCert := stateDir(), filepath.Dir(configPath()), certPath("m_work")
	if err := os.Remove(workCert); err != nil {
		t.Fatal(err)
	}
	if err := prepareAccount(personalAccount); err != nil {
		t.Fatal(err)
	}
	runTestCLI(t, "ensure-cert", "m_work", workDir, workConfig, "work")
	if _, err := os.Stat(workCert); err != nil {
		t.Fatal("personal config error prevented work certificate renewal")
	}
}
