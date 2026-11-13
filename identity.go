package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Identity defaults to the first verified sign-in in this profile. Config may
// override individual fields; an override never grants access or impersonates a user.
type Identity struct {
	Server string `json:"server,omitempty"`
	UserID string `json:"userId,omitempty"`
	OrgID  string `json:"orgId,omitempty"`
}

var identityCache = struct {
	sync.Mutex
	values map[string]Identity
}{values: map[string]Identity{}}

func identityPath() string { return filepath.Join(stateDir(), "identity.json") }

func readIdentity(path string, config bool) (Identity, error) {
	var identity Identity
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return identity, nil
	}
	if err != nil {
		return identity, err
	}
	if config {
		var cfg struct {
			Identity Identity `json:"identity"`
		}
		err = json.Unmarshal(data, &cfg)
		identity = cfg.Identity
	} else {
		err = json.Unmarshal(data, &identity)
		if err == nil && (identity.Server == "" || identity.UserID == "" || identity.OrgID == "") {
			err = errors.New("incomplete saved identity")
		}
	}
	if err != nil {
		return Identity{}, fmt.Errorf("invalid identity settings in %s: %w", path, err)
	}
	return identity, nil
}

func normalizedIssuer(server string) string {
	server = strings.TrimRight(server, "/")
	if replacement, ok := movedServers[server]; ok {
		return strings.TrimRight(replacement, "/")
	}
	return server
}

// verifyIdentity uses /me with the candidate token directly, not accessToken,
// so it also works before a new sign-in is saved and cannot recurse.
func verifyIdentity(c *Credentials) (Identity, error) {
	override, err := readIdentity(configPath(), true)
	if err != nil {
		return Identity{}, err
	}
	bound, err := readIdentity(identityPath(), false)
	if err != nil {
		return Identity{}, err
	}
	policy, _ := json.Marshal([]Identity{bound, override})
	key := stateDir() + "\x00" + c.Server + "\x00" + c.AccessToken + "\x00" + string(policy)
	identityCache.Lock()
	defer identityCache.Unlock()
	if identity, ok := identityCache.values[key]; ok {
		return identity, nil
	}
	var me struct {
		UserID json.Number `json:"userId"`
		Org    struct {
			ID string `json:"id"`
		} `json:"org"`
	}
	if err := call(request{method: http.MethodGet, path: "/v1/me", server: c.Server,
		token: c.AccessToken, out: &me}); err != nil {
		return Identity{}, err
	}
	if me.UserID == "" || me.Org.ID == "" {
		return Identity{}, errors.New("IBX sign-in did not return a user and default workspace")
	}
	actual := Identity{Server: normalizedIssuer(c.Server), UserID: string(me.UserID), OrgID: me.Org.ID}
	var scope Identity
	err = withLock("identity.lock", func() error {
		// Another process may have bound this profile while /me was in flight.
		bound, err = readIdentity(identityPath(), false)
		if err != nil {
			return err
		}
		expected := bound
		if expected.UserID == "" {
			expected = actual
		}
		if override.Server != "" {
			expected.Server = override.Server
		}
		if override.UserID != "" {
			if override.UserID != expected.UserID {
				expected.OrgID = actual.OrgID
			}
			expected.UserID = override.UserID
		}
		if override.OrgID != "" {
			expected.OrgID = override.OrgID
		}
		if normalizedIssuer(expected.Server) != actual.Server || expected.UserID != actual.UserID {
			return fmt.Errorf("profile %s identity mismatch: sign in as user %s on %s, or explicitly update this profile's identity settings", activeAccount(), expected.UserID, expected.Server)
		}
		if expected.OrgID != actual.OrgID {
			var out struct {
				Orgs []struct {
					ID string `json:"id"`
				} `json:"orgs"`
			}
			if err := call(request{method: http.MethodGet, path: "/v1/orgs", server: c.Server, token: c.AccessToken, out: &out}); err != nil {
				return err
			}
			allowed := false
			for _, org := range out.Orgs {
				if org.ID == expected.OrgID {
					allowed = true
				}
			}
			if !allowed {
				return fmt.Errorf("signed-in user does not have access to workspace %s", expected.OrgID)
			}
		}
		scope = Identity{Server: actual.Server, UserID: actual.UserID, OrgID: expected.OrgID}
		if bound.UserID == "" {
			data, _ := json.MarshalIndent(actual, "", "  ")
			return writeFileAtomic(identityPath(), data, 0o600)
		}
		return nil
	})
	if err != nil {
		return Identity{}, err
	}
	if len(identityCache.values) >= 32 {
		clear(identityCache.values)
	}
	identityCache.values[key] = scope
	return scope, nil
}

func currentIdentity() (Identity, error) {
	if _, err := accessToken(false); err != nil {
		return Identity{}, err
	}
	c, err := loadCredentials()
	if err != nil {
		return Identity{}, err
	}
	if c == nil {
		return Identity{}, errSignedOut
	}
	return verifyIdentity(c)
}

func requireOrg(actual string, scope Identity) error {
	if actual == "" || actual != scope.OrgID {
		return fmt.Errorf("resource is outside profile %s workspace %s", activeAccount(), scope.OrgID)
	}
	return nil
}

// Guard machine actions (including exec and certificate issuance) before they
// reach the server. Read-only list filters alone would not protect stale jobs.
func guardScopedRequest(r *request, token, server string, scope Identity) error {
	snapshotReference := ""
	if strings.HasPrefix(r.path, "/v1/snapshots/") && r.method == http.MethodDelete {
		snapshotReference = strings.TrimPrefix(r.path, "/v1/snapshots/")
	}
	if strings.HasPrefix(r.path, "/v1/machines/") && r.method != http.MethodGet {
		id := strings.SplitN(strings.TrimPrefix(r.path, "/v1/machines/"), "/", 2)[0]
		var machine Machine
		if err := call(request{method: http.MethodGet, path: "/v1/machines/" + id, token: token, server: server, out: &machine}); err != nil {
			return err
		}
		if err := requireOrg(machine.OrgID, scope); err != nil {
			return err
		}
	}
	if r.path == "/v1/machines" && r.method == http.MethodPost {
		in, ok := r.in.(map[string]any)
		if !ok {
			return errors.New("invalid machine create payload")
		}
		copy := map[string]any{}
		for k, v := range in {
			copy[k] = v
		}
		if org, present := copy["orgId"]; present && org != scope.OrgID {
			return errors.New("machine create workspace conflicts with profile identity")
		}
		copy["orgId"] = scope.OrgID
		if reference, ok := copy["snapshotId"].(string); ok {
			snapshotReference = reference
		}
		r.in = copy
	}
	if snapshotReference != "" {
		var out struct {
			Snapshots []Snapshot `json:"snapshots"`
		}
		if err := call(request{method: http.MethodGet, path: "/v1/snapshots", server: server, token: token, out: &out}); err != nil {
			return err
		}
		found := false
		for _, snapshot := range out.Snapshots {
			if snapshot.ID == snapshotReference || snapshot.Name == snapshotReference {
				if snapshot.OrgID == scope.OrgID {
					found = true
					break
				}
			}
		}
		if !found {
			return fmt.Errorf("snapshot is not available in workspace %s", scope.OrgID)
		}
	}
	return nil
}
