package attention

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"sessions/internal/config"
	"sessions/internal/events"
	"sessions/internal/registry"
)

func TestCommandWritesStructuredEvent(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "event.json")
	cfg := config.AttentionConfig{
		Command: config.AttentionCommandConfig{
			Command: "cat > " + strconv.Quote(outPath),
		},
	}
	event := attentionEvent(t, registry.AgentNeedsInput, true)
	if err := Command(context.Background(), cfg, event); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded events.Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Attention || decoded.Repo.Name != "repo" || decoded.Session.WorktreePath != "/work" || decoded.Agent.Name != "codex" {
		t.Fatalf("unexpected event: %+v", decoded)
	}
}

func TestCommandUsesPerStateOverride(t *testing.T) {
	dir := t.TempDir()
	defaultPath := filepath.Join(dir, "default.json")
	failedPath := filepath.Join(dir, "failed.json")
	cfg := config.AttentionConfig{
		Command: config.AttentionCommandConfig{
			Command: "cat > " + strconv.Quote(defaultPath),
			States: map[string]config.AttentionCommandRouteConfig{
				registry.AgentFailed: {Command: "cat > " + strconv.Quote(failedPath)},
			},
		},
	}
	if err := Command(context.Background(), cfg, attentionEvent(t, registry.AgentFailed, true)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(failedPath); err != nil {
		t.Fatalf("state override did not run: %v", err)
	}
	if _, err := os.Stat(defaultPath); !os.IsNotExist(err) {
		t.Fatalf("default command should not run, stat err: %v", err)
	}
}

func TestCommandSkipsQuietState(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "event.json")
	cfg := config.AttentionConfig{
		Command: config.AttentionCommandConfig{
			Command: "cat > " + strconv.Quote(outPath),
		},
	}
	if err := Command(context.Background(), cfg, attentionEvent(t, registry.AgentReady, false)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("quiet state should not invoke command, stat err: %v", err)
	}
}

func attentionEvent(t *testing.T, state string, isAttention bool) events.Event {
	t.Helper()
	root := t.TempDir()
	repo := &registry.Repo{ID: "repo", Name: "repo", CommonDir: filepath.Join(root, ".git"), MainWorktree: root}
	sess := &registry.Session{ID: "s", Name: "billing", Index: 1, Branch: "billing", WorktreePath: "/work"}
	agent := &registry.Agent{Name: "codex", State: state, Message: "message", UpdatedAt: time.Now().UTC()}
	return events.NewAgent(repo, sess, agent, isAttention, events.TargetStatus{})
}
