package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Exercise the real CLI dispatch/fanout and certificate helper in subprocesses
// without compiling another binary or touching the developer's live accounts.
func TestMain(m *testing.M) {
	if os.Getenv("HERDR_INSTABOX_TEST_CLI") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runTestCLI(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "HERDR_INSTABOX_TEST_CLI=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("CLI %v: %v\n%s", args, err, out)
	}
}

func fakeAccounts(t *testing.T) *fakeEnv {
	t.Helper()
	f := newFake(t, fakeMachine("m_box", "personal-box", "running"))
	t.Setenv("HERDR_INSTABOX_ACCOUNT", personalAccount)
	t.Setenv("HERDR_INSTABOX_ROOT_STATE_DIR", filepath.Join(f.dir, "state"))
	t.Setenv("HERDR_INSTABOX_ROOT_CONFIG_DIR", filepath.Join(f.dir, "config"))
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", filepath.Join(f.dir, "config"))
	// prepareAccount changes these variables; register all with t.Setenv first.
	t.Setenv("HERDR_PLUGIN_STATE_DIR", filepath.Join(f.dir, "state"))
	if err := prepareAccount(personalAccount); err != nil {
		t.Fatal(err)
	}
	if err := addAccount("work"); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestAccountSyncRenewalAndSignOutAreIsolated(t *testing.T) {
	f := fakeAccounts(t)
	personalCredentials, _ := os.ReadFile(credentialsPath())
	personalKeyPath, personalInclude := keyPath(), includeLine()
	f.sync()
	personalKey, _ := os.ReadFile(personalKeyPath)

	// A different API server, but the same machine ID, must still be isolated.
	w := &fakeEnv{t: t, dir: t.TempDir()}
	w.api, w.server = startFakeAPI(t, w.dir)
	w.writeInstabox(fakeInstabox{SignedIn: true, Machines: []map[string]any{fakeMachine("m_box", "work-box", "running")}})
	if err := prepareAccount("work"); err != nil {
		t.Fatal(err)
	}
	cfg, err := readConfig(configPath())
	if err != nil || cfg.DefaultSnapshot != "" {
		t.Fatal("another account's snapshot config leaked")
	}
	if err := saveCredentials(&Credentials{Server: w.server, AccessToken: "work-at", AccessExpiresAt: time.Now().Add(-time.Minute), RefreshToken: "work-rt"}); err != nil {
		t.Fatal(err)
	}
	// Even a host environment still pointing at personal cannot retarget work.
	if serverURL() != f.server {
		t.Fatal("test requires a conflicting server override")
	}
	f.sync()
	if w.api.refreshes != 1 || f.api.refreshes != 0 {
		t.Fatal("refreshed the wrong sign-in")
	}
	workTarget := (Machine{ID: "m_box"}).Target()
	workKey, _ := os.ReadFile(keyPath())
	if string(workKey) == string(personalKey) || keyPath() == personalKeyPath {
		t.Fatal("SSH keys were shared")
	}
	if len(f.herdr().Profiles) != 2 {
		t.Fatal("sync removed another account's machine")
	}
	if !strings.Contains(f.sshConfig(), personalInclude) || !strings.Contains(f.sshConfig(), includeLine()) {
		t.Fatal("account SSH includes must coexist")
	}
	if err := prepareAccount(personalAccount); err != nil {
		t.Fatal(err)
	}
	f.sync()
	if len(f.herdr().Profiles) != 2 {
		t.Fatal("legacy sync removed a named account's machine")
	}
	if after, _ := os.ReadFile(credentialsPath()); string(after) != string(personalCredentials) {
		t.Fatal("work refresh changed the legacy sign-in")
	}

	if err := prepareAccount("work"); err != nil {
		t.Fatal(err)
	}
	// Expire just work's certificate; only work's API may issue its replacement.
	if err := os.Remove(certPath("m_box")); err != nil {
		t.Fatal(err)
	}
	before := w.api.certs
	if _, err := ensureCert("m_box", false); err != nil {
		t.Fatal(err)
	}
	if w.api.certs != before+1 {
		t.Fatal("work certificate was not independently renewed")
	}
	if err := cleanupLocal(); err != nil {
		t.Fatal(err)
	}
	if err := signOut(); err != nil {
		t.Fatal(err)
	}
	for _, p := range f.herdr().Profiles {
		if p.Target == workTarget {
			t.Fatal("signed-out work profile remains")
		}
	}
	if len(f.herdr().Profiles) != 1 || !strings.Contains(f.sshConfig(), personalInclude) {
		t.Fatal("sign-out damaged personal")
	}
	if err := prepareAccount(personalAccount); err != nil {
		t.Fatal(err)
	}
	if a := whoami(); !a.SignedIn() {
		t.Fatal("personal was signed out with work")
	}
	if _, err := os.Stat(personalKeyPath); err != nil {
		t.Fatal("personal SSH key removed")
	}
}

func TestAccountValidationAndLegacyMigration(t *testing.T) {
	f := fakeAccounts(t)
	root := stateDir()
	for _, name := range []string{"../other", "", "/tmp/work", "Work", "a/b", strings.Repeat("a", 33)} {
		if err := addAccount(name); err == nil {
			t.Errorf("accepted unsafe name %q", name)
		}
	}
	if err := addAccount("work"); err == nil {
		t.Fatal("duplicate account replaced")
	}
	if err := prepareAccount("missing"); err == nil {
		t.Fatal("unknown profile silently created")
	}
	if stateDir() != root {
		t.Fatal("failed switch changed the active profile")
	}
	if err := os.Symlink(root, filepath.Join(root, "accounts", "linked")); err != nil {
		t.Fatal(err)
	}
	if err := prepareAccount("linked"); err == nil {
		t.Fatal("followed a symlink account")
	}
	names, err := accountNames()
	if err != nil || strings.Join(names, ",") != "personal,work" {
		t.Fatalf("bad account inventory: %v %v", names, err)
	}
	if (Machine{ID: "m_box"}).Target() != TargetPrefix+"m_box" {
		t.Fatal("legacy SSH aliases changed")
	}
	if credentialsPath() != filepath.Join(f.dir, "state", "credentials.json") {
		t.Fatal("legacy credentials moved")
	}
	if err := prepareAccount("work"); err != nil {
		t.Fatal(err)
	}
	if c, err := loadCredentials(); err != nil || c != nil {
		t.Fatal("new profile inherited a login")
	}
	if keyPath() == filepath.Join(root, "ssh", "id_ed25519") {
		t.Fatal("new profile inherited a key")
	}
}

func TestSimilarAccountNamesCannotClaimEachOthersTargets(t *testing.T) {
	fakeAccounts(t)
	if err := addAccount("work-team"); err != nil {
		t.Fatal(err)
	}
	if err := prepareAccount("work-team"); err != nil {
		t.Fatal(err)
	}
	other := Profile{ID: "other", Target: (Machine{ID: "m_box"}).Target()}
	if err := prepareAccount("work"); err != nil {
		t.Fatal(err)
	}
	if _, ok := other.MachineID(); ok {
		t.Fatal("prefix overlap claimed another account")
	}
	managed, ssh := splitProfiles([]Profile{other})
	if len(managed) != 0 || len(ssh) != 0 {
		t.Fatal("foreign account must not be managed or shown as an ordinary SSH profile")
	}
}

func TestAccountUISwitchRequestsNewProcess(t *testing.T) {
	fakeAccounts(t)
	m := newModel("account")
	result, cmd := m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	if cmd == nil || result.(model).nextAccount != "work" {
		t.Fatal("account navigation not wired")
	}
	if activeAccount() != personalAccount {
		t.Fatal("UI switched credentials while background requests may be active")
	}
	if err := selectAccount("work"); err != nil {
		t.Fatal(err)
	}
	if selectedAccount() != "work" || activeAccount() != personalAccount {
		t.Fatal("saved UI selection must not change an in-flight process's identity")
	}
	if err := selectAccount("missing"); err == nil {
		t.Fatal("selected an unknown account")
	}
	if selectedAccount() != "work" {
		t.Fatal("failed selection changed the saved profile")
	}
}

func TestCLIAllAccountsAndCertificateHelper(t *testing.T) {
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
	workDir, workCert := stateDir(), certPath("m_work")
	if err := prepareAccount(personalAccount); err != nil {
		t.Fatal(err)
	}
	runTestCLI(t, "sync")
	if len(f.herdr().Profiles) != 2 {
		t.Fatal("unqualified sync did not retain both accounts")
	}
	if err := os.Remove(workCert); err != nil {
		t.Fatal(err)
	}
	w.api.mu.Lock()
	before := w.api.certs
	w.api.mu.Unlock()
	// The helper starts from a different account with an invalid ambient server.
	t.Setenv("INSTABOX_SERVER", "http://127.0.0.1:1")
	runTestCLI(t, "ensure-cert", "m_work", workDir)
	w.api.mu.Lock()
	after := w.api.certs
	w.api.mu.Unlock()
	if after != before+1 {
		t.Fatal("SSH helper did not renew through the owning account")
	}
	runTestCLI(t, "--account", "work", "logout")
	if len(f.herdr().Profiles) != 1 {
		t.Fatal("CLI work logout affected the wrong profiles")
	}
	if a := whoami(); !a.SignedIn() {
		t.Fatal("CLI work logout revoked personal")
	}
}
