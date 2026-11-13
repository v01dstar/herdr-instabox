package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func identityConfig(t *testing.T, f *fakeEnv, identity Identity) {
	t.Helper()
	dir := filepath.Join(f.dir, "identity-config")
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", dir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(Config{Identity: identity})
	if err := os.WriteFile(configPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestIdentityDefaultsToSignInAndPreservesLoginOnMismatch(t *testing.T) {
	f := newFake(t)
	scope, err := currentIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if scope.UserID != "42" || scope.OrgID != "org_personal" || scope.Server != f.server {
		t.Fatalf("unexpected default: %+v", scope)
	}
	info, err := os.Stat(identityPath())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("automatic binding not saved privately")
	}
	before, _ := os.ReadFile(credentialsPath())
	st := f.instabox()
	st.UserID = 99
	st.OrgID = "org_team"
	f.writeInstabox(st)
	candidate := &Credentials{Server: f.server, AccessToken: "other-user-token", AccessExpiresAt: time.Now().Add(time.Hour)}
	if err := acceptCredentials(candidate); err == nil {
		t.Fatal("wrong sign-in silently replaced the bound user")
	}
	if after, _ := os.ReadFile(credentialsPath()); string(after) != string(before) {
		t.Fatal("rejected candidate overwrote the working sign-in")
	}
	identityConfig(t, f, Identity{UserID: "99"})
	if err := acceptCredentials(candidate); err != nil {
		t.Fatal(err)
	}
	scope, err = currentIdentity()
	if err != nil || scope.UserID != "99" || scope.OrgID != "org_team" {
		t.Fatalf("explicit override failed: %+v %v", scope, err)
	}
	identityConfig(t, f, Identity{})
	if _, err := currentIdentity(); err == nil {
		t.Fatal("removing override must restore the original automatic user binding")
	}
}

func TestWorkspaceOverrideFiltersResourcesAndBlocksForeignActions(t *testing.T) {
	personal := fakeMachine("m_personal", "personal", "running")
	team := fakeMachine("m_team", "team", "running")
	team["orgId"] = "org_team"
	f := newFake(t, personal, team)
	st := f.instabox()
	st.Orgs = []string{"org_team"}
	st.Snapshots = []map[string]any{
		{"id": "sn_personal", "name": "personal", "orgId": "org_personal", "template": map[string]any{"id": "herdr"}},
		{"id": "sn_team", "name": "team", "orgId": "org_team", "template": map[string]any{"id": "herdr"}},
	}
	f.writeInstabox(st)
	initial, err := currentIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ensureCert("m_personal", true); err != nil {
		t.Fatal(err)
	}
	identityConfig(t, f, Identity{OrgID: "org_team"})
	machines, err := listMachines()
	if err != nil || len(machines) != 1 || machines[0].ID != "m_team" {
		t.Fatalf("machine scope: %+v %v", machines, err)
	}
	snapshots, err := listSnapshots()
	if err != nil || len(snapshots) != 1 || snapshots[0].ID != "sn_team" {
		t.Fatalf("snapshot scope: %+v %v", snapshots, err)
	}
	if err := stopMachine("m_personal"); err == nil {
		t.Fatal("foreign stop was allowed")
	}
	if err := execOnMachine("m_personal", "echo no", 1); err == nil {
		t.Fatal("foreign exec was allowed")
	}
	if err := deleteSnapshot("sn_personal"); err == nil {
		t.Fatal("foreign snapshot deletion was allowed")
	}
	if _, err := createMachine("bad-copy", "", "sn_personal"); err == nil {
		t.Fatal("foreign image was accepted")
	}
	job := Job{Kind: jobStop, MachineID: "m_personal", Identity: &initial}
	if _, err := job.run(func(string, ...any) {}); err == nil {
		t.Fatal("stale job survived a workspace override")
	}
	runEnsureCert("m_personal", stateDir(), filepath.Dir(configPath()), activeAccount())
	if _, err := os.Stat(certPath("m_personal")); !os.IsNotExist(err) {
		t.Fatal("old cached certificate survived the scope change")
	}
	if _, err := createMachine("team-new", "herdr", ""); err != nil {
		t.Fatal(err)
	}
	created := f.instabox().Machines
	if len(created) != 3 || created[2]["orgId"] != "org_team" {
		t.Fatal("create did not explicitly target the overridden workspace")
	}
	if created[0]["state"] != "running" || len(f.instabox().Snapshots) != 2 {
		t.Fatal("blocked actions changed foreign resources")
	}
}

func TestIdentityOverridesFailClosed(t *testing.T) {
	f := newFake(t)
	if _, err := currentIdentity(); err != nil {
		t.Fatal(err)
	}
	for _, override := range []Identity{{UserID: "another-user"}, {OrgID: "org_not_a_member"}, {Server: "https://not-the-issuer.invalid"}} {
		identityConfig(t, f, override)
		if _, err := currentIdentity(); err == nil {
			t.Fatalf("accepted unauthorized override: %+v", override)
		}
	}
	if err := os.WriteFile(configPath(), []byte(`{"identity":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := currentIdentity(); err == nil {
		t.Fatal("malformed policy silently fell back")
	}
}

func TestIdentityRejectsChangedUserOnRefresh(t *testing.T) {
	f := newFake(t)
	if _, err := currentIdentity(); err != nil {
		t.Fatal(err)
	}
	c, err := loadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	c.AccessExpiresAt = time.Now().Add(-time.Minute)
	if err := saveCredentials(c); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(credentialsPath())
	st := f.instabox()
	st.UserID = 99
	f.writeInstabox(st)
	if _, err := accessToken(true); err == nil {
		t.Fatal("refresh silently changed the bound user")
	}
	if after, _ := os.ReadFile(credentialsPath()); string(after) != string(before) {
		t.Fatal("rejected refresh replaced the original credentials")
	}
}
