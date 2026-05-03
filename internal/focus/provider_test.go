package focus

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"sessions/internal/config"
	"sessions/internal/events"
	"sessions/internal/registry"
)

func TestRouteRunsCustomProviderCommand(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "event.json")
	effective := testEffective()
	effective.Editor = "none"
	effective.Focus.Providers = map[string]config.FocusProviderConfig{
		ProviderCustom: {Command: "cat > " + shellQuote(outPath)},
	}
	repo, sess, agent := routeFixtures()
	agent.FocusTarget = &registry.FocusTarget{Provider: ProviderCustom, TargetID: "pane-1", AttachedAt: time.Now().UTC()}

	var stdout, stderr bytes.Buffer
	if err := Route(context.Background(), effective, repo, sess, agent, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var event events.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	if event.Agent == nil || event.Agent.Name != "codex" || event.FocusTarget == nil || event.FocusTarget.TargetID != "pane-1" {
		t.Fatalf("unexpected event: %+v", event)
	}
	if !strings.Contains(stdout.String(), "via custom target pane-1") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestRouteIgnoresStaleTargetAndFallsBack(t *testing.T) {
	effective := testEffective()
	effective.Editor = "none"
	repo, sess, agent := routeFixtures()
	expired := time.Now().Add(-time.Minute)
	agent.FocusTarget = &registry.FocusTarget{Provider: ProviderCustom, TargetID: "pane-1", AttachedAt: time.Now().Add(-time.Hour), ExpiresAt: &expired}

	var stdout, stderr bytes.Buffer
	if err := Route(context.Background(), effective, repo, sess, agent, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "ignoring stale focus target") {
		t.Fatalf("expected stale warning, got: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), sess.WorktreePath) {
		t.Fatalf("expected manual fallback path, got: %s", stdout.String())
	}
}

func TestRouteProviderFailureFallsBack(t *testing.T) {
	effective := testEffective()
	effective.Editor = "none"
	effective.Focus.Providers = map[string]config.FocusProviderConfig{
		ProviderCustom: {Command: "echo failed >&2; exit 7"},
	}
	repo, sess, agent := routeFixtures()
	agent.FocusTarget = &registry.FocusTarget{Provider: ProviderCustom, TargetID: "pane-1", AttachedAt: time.Now().UTC()}

	var stdout, stderr bytes.Buffer
	if err := Route(context.Background(), effective, repo, sess, agent, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "provider command failed") || !strings.Contains(stderr.String(), "failed") {
		t.Fatalf("expected provider failure warning, got: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), sess.WorktreePath) {
		t.Fatalf("expected fallback path, got: %s", stdout.String())
	}
}

func TestRouteOpensEditorFallback(t *testing.T) {
	effective := testEffective()
	effective.Editor = "code"
	effective.Editors["code"] = config.EditorConfig{
		Command: "sh",
		Args:    []string{"-c", ":"},
	}
	repo, sess, _ := routeFixtures()

	var stdout, stderr bytes.Buffer
	if err := Route(context.Background(), effective, repo, sess, nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "focused repo/billing at /repo-billing") {
		t.Fatalf("expected editor fallback success, got: %s", stdout.String())
	}
}

func TestRouteBuiltInProviderUsesEditorConfig(t *testing.T) {
	effective := testEffective()
	effective.Editor = "none"
	effective.Editors["code"] = config.EditorConfig{
		Command: "sh",
		Args:    []string{"-c", ":"},
	}
	repo, sess, agent := routeFixtures()
	agent.FocusTarget = &registry.FocusTarget{Provider: ProviderCode, TargetID: "window-1", AttachedAt: time.Now().UTC()}

	var stdout, stderr bytes.Buffer
	if err := Route(context.Background(), effective, repo, sess, agent, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "focused repo/billing with code") {
		t.Fatalf("expected built-in provider success, got: %s", stdout.String())
	}
}

func routeFixtures() (*registry.Repo, *registry.Session, *registry.Agent) {
	repo := &registry.Repo{ID: "repo", Name: "repo", CommonDir: "/repo/.git", MainWorktree: "/repo"}
	sess := &registry.Session{ID: "s", Name: "billing", RepoID: "repo", Index: 1, WorktreePath: "/repo-billing"}
	agent := &registry.Agent{Name: "codex", State: registry.AgentNeedsInput, Message: "input", UpdatedAt: time.Now().UTC()}
	return repo, sess, agent
}

func testEffective() config.Effective {
	cfg := config.Default()
	return config.Effective{
		RepoName:  "repo",
		Editor:    "none",
		Attention: cfg.Attention,
		Focus:     cfg.Focus,
		Editors:   cfg.Editors,
	}
}

func shellQuote(s string) string {
	return strconv.Quote(s)
}
