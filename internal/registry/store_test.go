package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestReadMissingRegistryReturnsEmpty(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "registry.json"))
	reg, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Version != SupportedVersion || len(reg.Repos) != 0 {
		t.Fatalf("unexpected registry: %+v", reg)
	}
}

func TestUpdatePersistsMutation(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "registry.json"))
	if err := store.Update(func(reg *Registry) error {
		repo := reg.EnsureRepo("repo", "repo", "/repo/.git", "/repo")
		repo.Sessions["s"] = &Session{ID: "s", Name: "billing", RepoID: "repo", Index: 1, UpdatedAt: time.Now().UTC()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reg, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Repos["repo"].Sessions["s"].Name != "billing" {
		t.Fatalf("mutation was not persisted: %+v", reg)
	}
}

func TestHigherVersionRefusesToOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"repos":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewStore(path).Read()
	if err == nil {
		t.Fatal("expected higher version error")
	}
}

func TestCorruptRegistryDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	if err := os.WriteFile(path, []byte(`{bad json`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := NewStore(path).Update(func(reg *Registry) error {
		reg.EnsureRepo("repo", "repo", "/repo/.git", "/repo")
		return nil
	})
	if err == nil {
		t.Fatal("expected corrupt JSON error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{bad json` {
		t.Fatalf("corrupt registry was overwritten: %q", data)
	}
}

func TestConcurrentUpdatesPreserveAgents(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "registry.json"))
	if err := store.Update(func(reg *Registry) error {
		repo := reg.EnsureRepo("repo", "repo", "/repo/.git", "/repo")
		repo.Sessions["s"] = &Session{ID: "s", Name: "billing", RepoID: "repo", Agents: map[string]*Agent{}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, agent := range []string{"claude", "codex"} {
		wg.Add(1)
		go func(agent string) {
			defer wg.Done()
			if err := store.Update(func(reg *Registry) error {
				s := reg.Repos["repo"].Sessions["s"]
				s.Agents[agent] = &Agent{Name: agent, State: AgentReady, UpdatedAt: time.Now().UTC()}
				return nil
			}); err != nil {
				t.Errorf("update %s: %v", agent, err)
			}
		}(agent)
	}
	wg.Wait()
	data, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Registry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("registry JSON corrupt: %v\n%s", err, data)
	}
	agents := decoded.Repos["repo"].Sessions["s"].Agents
	if agents["claude"] == nil || agents["codex"] == nil {
		t.Fatalf("lost concurrent update: %+v", agents)
	}
}

func TestFocusTargetRoundTrip(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "registry.json"))
	expiresAt := time.Now().Add(10 * time.Minute).UTC()
	attachedAt := time.Now().UTC()
	if err := store.Update(func(reg *Registry) error {
		repo := reg.EnsureRepo("repo", "repo", "/repo/.git", "/repo")
		repo.Sessions["s"] = &Session{
			ID:     "s",
			Name:   "billing",
			RepoID: "repo",
			Agents: map[string]*Agent{
				"codex": {
					Name:      "codex",
					State:     AgentRunning,
					UpdatedAt: attachedAt,
					FocusTarget: &FocusTarget{
						Provider:   "custom",
						TargetID:   "pane-1",
						Metadata:   map[string]any{"window": "main"},
						PID:        os.Getpid(),
						AttachedAt: attachedAt,
						ExpiresAt:  &expiresAt,
					},
				},
			},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reg, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	target := reg.Repos["repo"].Sessions["s"].Agents["codex"].FocusTarget
	if target == nil || target.Provider != "custom" || target.TargetID != "pane-1" || target.Metadata["window"] != "main" || target.PID != os.Getpid() || target.ExpiresAt == nil {
		t.Fatalf("focus target did not round-trip: %+v", target)
	}
}
