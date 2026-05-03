package session

import (
	"strings"
	"testing"
	"time"

	"sessions/internal/registry"
)

func TestValidateName(t *testing.T) {
	for _, name := range []string{"billing", "a1", "one-two"} {
		if err := ValidateName(name); err != nil {
			t.Fatalf("%s should be valid: %v", name, err)
		}
	}
	for _, name := range []string{"Billing", "-bad", "bad_underscore"} {
		if err := ValidateName(name); err == nil {
			t.Fatalf("%s should be invalid", name)
		}
	}
}

func TestAllocateIndexReusesLowestAvailable(t *testing.T) {
	repo := &registry.Repo{Sessions: map[string]*registry.Session{
		"a": {Index: 2},
		"b": {Index: 3},
	}}
	if got := AllocateIndex(repo); got != 1 {
		t.Fatalf("AllocateIndex() = %d, want 1", got)
	}
	repo.Sessions["c"] = &registry.Session{Index: 1}
	delete(repo.Sessions, "a")
	if got := AllocateIndex(repo); got != 2 {
		t.Fatalf("AllocateIndex() = %d, want 2", got)
	}
}

func TestEnvFileIncludesRequiredValues(t *testing.T) {
	repo := &registry.Repo{Name: "sampleapp", MainWorktree: "/tmp/sampleapp"}
	sess := &registry.Session{
		ID:           "abc",
		Name:         "billing",
		Index:        3,
		WorktreePath: "/tmp/sampleapp-billing",
		Branch:       "billing",
		BaseRef:      "main",
		UpdatedAt:    time.Now(),
	}
	env := EnvFile(repo, sess)
	for _, want := range []string{
		`SESSION_ID="abc"`,
		`SESSION_NAME="billing"`,
		`SESSION_INDEX="3"`,
		`SESSION_REPO_NAME="sampleapp"`,
		`SESSION_WORKTREE="/tmp/sampleapp-billing"`,
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("env file missing %q:\n%s", want, env)
		}
	}
	if strings.Contains(env, "export ") {
		t.Fatalf("env file should be pure dotenv without export:\n%s", env)
	}
}
