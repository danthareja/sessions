package focus

import (
	"testing"
	"time"

	"sessions/internal/attention"
	"sessions/internal/registry"
)

func TestNextSelectsAttentionByPriorityThenOldestUpdate(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	newer := time.Now()
	repo := &registry.Repo{
		ID:   "repo",
		Name: "repo",
		Sessions: map[string]*registry.Session{
			"a": {
				ID:   "a",
				Name: "a",
				Agents: map[string]*registry.Agent{
					"codex": {Name: "codex", State: registry.AgentNeedsInput, UpdatedAt: newer},
				},
			},
			"b": {
				ID:   "b",
				Name: "b",
				Agents: map[string]*registry.Agent{
					"claude": {Name: "claude", State: registry.AgentFailed, UpdatedAt: old},
				},
			},
		},
	}

	candidate, ok := Next([]*registry.Repo{repo}, attention.New([]string{registry.AgentNeedsInput, registry.AgentFailed}))
	if !ok {
		t.Fatal("expected candidate")
	}
	if candidate.Session.Name != "b" || candidate.Agent.Name != "claude" {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
}

func TestNextUsesOldestUpdateWithinPriority(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	newer := time.Now()
	repo := &registry.Repo{
		ID:   "repo",
		Name: "repo",
		Sessions: map[string]*registry.Session{
			"old": {
				ID:   "old",
				Name: "old",
				Agents: map[string]*registry.Agent{
					"codex": {Name: "codex", State: registry.AgentNeedsInput, UpdatedAt: old},
				},
			},
			"new": {
				ID:   "new",
				Name: "new",
				Agents: map[string]*registry.Agent{
					"claude": {Name: "claude", State: registry.AgentNeedsInput, UpdatedAt: newer},
				},
			},
		},
	}

	candidate, ok := Next([]*registry.Repo{repo}, attention.New([]string{registry.AgentNeedsInput}))
	if !ok {
		t.Fatal("expected candidate")
	}
	if candidate.Session.Name != "old" {
		t.Fatalf("expected oldest session, got %s", candidate.Session.Name)
	}
}

func TestNextIgnoresQuietStates(t *testing.T) {
	repo := &registry.Repo{
		ID:   "repo",
		Name: "repo",
		Sessions: map[string]*registry.Session{
			"s": {
				ID:   "s",
				Name: "s",
				Agents: map[string]*registry.Agent{
					"codex": {Name: "codex", State: registry.AgentReady, UpdatedAt: time.Now()},
				},
			},
		},
	}
	if _, ok := Next([]*registry.Repo{repo}, attention.New([]string{registry.AgentNeedsInput})); ok {
		t.Fatal("ready should be quiet")
	}
}
