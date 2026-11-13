package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSnapshotProfileCredentials(t *testing.T) {
	for _, snapshot := range []string{"1984-ready", "mdp-ready", "generic-ready"} {
		t.Run(snapshot, func(t *testing.T) {
			f := newFake(t)
			var got map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" && r.URL.Path == "/v1/me" {
					writeJSON(w, 200, map[string]any{"userId": 42, "org": map[string]string{"id": "org_personal"}})
					return
				}
				if r.Method == "GET" && r.URL.Path == "/v1/snapshots" {
					writeJSON(w, 200, map[string]any{"snapshots": []map[string]string{{"id": snapshot, "orgId": "org_personal"}}})
					return
				}
				if r.Method != "POST" || r.URL.Path != "/v1/machines" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				_ = json.NewDecoder(r.Body).Decode(&got)
				writeJSON(w, 202, succeeded("m_test"))
			}))
			defer srv.Close()
			t.Setenv("INSTABOX_SERVER", srv.URL)
			if err := saveCredentials(&Credentials{Server: srv.URL, AccessToken: "test", AccessExpiresAt: time.Now().Add(time.Hour)}); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HERDR_PLUGIN_CONFIG_DIR", f.dir)
			config := `{"disableVault":true,"secrets":["WRONG_GLOBAL_TOKEN"],"metadata":{"herdr":"true"},"snapshotProfiles":{
				"1984-ready":{"secrets":["GH_TOKEN","OPENROUTER_API_KEY"],"metadata":{"purpose":"1984"}},
				"mdp-ready":{"secrets":["GH_TOKEN","PERSONAL_OPENROUTER_KEY"],"metadata":{"purpose":"mdp"}},
				"generic-ready":{"secrets":["PERSONAL_OPENROUTER_KEY"],"metadata":{"purpose":"generic"}}
			}}`
			if err := os.WriteFile(filepath.Join(f.dir, "config.json"), []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := createMachine("test", "", snapshot); err != nil {
				t.Fatal(err)
			}
			want := map[string][]any{
				"1984-ready":    {"GH_TOKEN", "OPENROUTER_API_KEY"},
				"mdp-ready":     {"GH_TOKEN", "PERSONAL_OPENROUTER_KEY"},
				"generic-ready": {"PERSONAL_OPENROUTER_KEY"},
			}[snapshot]
			if !reflect.DeepEqual(got["secrets"], want) || got["vault"] != false {
				t.Fatalf("incorrect credential selection: %#v", got)
			}
			metadata := got["metadata"].(map[string]any)
			if metadata["herdr"] != "true" || metadata["purpose"] == nil {
				t.Fatalf("incorrect metadata: %#v", metadata)
			}
		})
	}
}
