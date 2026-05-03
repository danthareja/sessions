package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"sessions/internal/registry"
	"sessions/internal/testutil"
)

func TestNewAgentSetListRemoveFlow(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := testutil.TempGitRepo(t)
	testHome(t)

	var stdout, stderr bytes.Buffer
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
		Cwd:    repo,
		Getenv: os.Getenv,
	}
	if err := New(ctx, env, []string{"billing", "--base", "main", "--editor", "none"}); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(repo), "repo-billing")
	if _, err := os.Stat(filepath.Join(worktree, ".env.sessions")); err != nil {
		t.Fatalf(".env.sessions was not written: %v", err)
	}

	env.Cwd = worktree
	if err := AgentSet(ctx, env, []string{"codex", "--state", "needs-input", "--message", "Codex needs input"}); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	env.Cwd = repo
	if err := List(ctx, env, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "codex:needs-input") {
		t.Fatalf("ls output missing agent state:\n%s", stdout.String())
	}

	if err := Remove(ctx, env, []string{"billing"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("worktree still exists or stat failed: %v", err)
	}
}

func TestNewWarnsWhenBranchAlreadyExists(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := testutil.TempGitRepo(t)
	testHome(t)
	testutil.Run(t, repo, "git", "branch", "existing", "main")

	var stderr bytes.Buffer
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
		Cwd:    repo,
		Getenv: os.Getenv,
	}
	err := New(ctx, env, []string{"conflict", "--base", "main", "--branch", "existing", "--editor", "none"})
	if err == nil {
		t.Fatal("expected existing branch to fail")
	}
	if !strings.Contains(err.Error(), `branch "existing" already exists`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "warning: branch existing already exists") {
		t.Fatalf("expected existing branch warning:\n%s", stderr.String())
	}
	store, err := registryStore()
	if err != nil {
		t.Fatal(err)
	}
	reg, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Repos) != 0 {
		t.Fatalf("registry should be empty after existing branch failure: %+v", reg)
	}
}

func TestInitCreatesRepoConfigAndHooks(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := testutil.TempGitRepo(t)

	var stdout bytes.Buffer
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Cwd:    repo,
		Getenv: os.Getenv,
	}
	if err := Init(ctx, env, nil); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(repo, ".sessions.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	configText := string(data)
	if !strings.Contains(configText, `on_create = 'sh "$SESSION_REPO_ROOT/.sessions/create.sh"'`) {
		t.Fatalf("repo config missing create hook:\n%s", configText)
	}
	for _, path := range []string{
		filepath.Join(repo, ".sessions", "create.sh"),
		filepath.Join(repo, ".sessions", "remove.sh"),
	} {
		if info, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		} else if info.Mode().Perm() != 0o755 {
			t.Fatalf("%s mode = %v", path, info.Mode().Perm())
		}
	}
	if !strings.Contains(stdout.String(), "Initialized Sessions") {
		t.Fatalf("init output missing summary:\n%s", stdout.String())
	}

	stdout.Reset()
	if err := Init(ctx, env, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "already initialized") {
		t.Fatalf("idempotent init output missing already initialized:\n%s", stdout.String())
	}
}

func TestInitDoesNotOverwriteExistingConfig(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := testutil.TempGitRepo(t)
	configPath := filepath.Join(repo, ".sessions.toml")
	custom := []byte("on_create = \"./custom-create\"\n")
	if err := os.WriteFile(configPath, custom, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Cwd:    repo,
		Getenv: os.Getenv,
	}
	if err := Init(ctx, env, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(custom) {
		t.Fatalf("init overwrote existing config:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(repo, ".sessions")); !os.IsNotExist(err) {
		t.Fatalf("init created unused default hooks for custom config: %v", err)
	}
}

func TestRemoveRefusesDirtyWorktree(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := testutil.TempGitRepo(t)
	testHome(t)

	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    repo,
		Getenv: os.Getenv,
	}
	if err := New(ctx, env, []string{"dirty", "--base", "main", "--editor", "none"}); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(repo), "repo-dirty")
	if err := os.WriteFile(filepath.Join(worktree, "change.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Remove(ctx, env, []string{"dirty"})
	if err == nil {
		t.Fatal("expected dirty removal to fail")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := Remove(ctx, env, []string{"dirty", "--force"}); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveDeletesReusableSessionBranch(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := testutil.TempGitRepo(t)
	testHome(t)

	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    repo,
		Getenv: os.Getenv,
	}
	if err := New(ctx, env, []string{"reusable", "--base", "main", "--editor", "none"}); err != nil {
		t.Fatal(err)
	}
	if err := Remove(ctx, env, []string{"reusable"}); err != nil {
		t.Fatal(err)
	}
	if branchExists(t, repo, "reusable") {
		t.Fatal("session branch was not deleted")
	}
	if err := New(ctx, env, []string{"reusable", "--base", "main", "--editor", "none"}); err != nil {
		t.Fatalf("expected session name to be reusable after removal: %v", err)
	}
}

func TestRemoveKeepsUnmergedSessionBranch(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := testutil.TempGitRepo(t)
	testHome(t)

	var stderr bytes.Buffer
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
		Cwd:    repo,
		Getenv: os.Getenv,
	}
	if err := New(ctx, env, []string{"unmerged", "--base", "main", "--editor", "none"}); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(repo), "repo-unmerged")
	if err := os.WriteFile(filepath.Join(worktree, "change.txt"), []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.Run(t, worktree, "git", "add", "change.txt")
	testutil.Run(t, worktree, "git", "commit", "-m", "session commit")

	if err := Remove(ctx, env, []string{"unmerged"}); err != nil {
		t.Fatal(err)
	}
	if !branchExists(t, repo, "unmerged") {
		t.Fatal("unmerged session branch should be kept")
	}
	if !strings.Contains(stderr.String(), "branch unmerged was not deleted") {
		t.Fatalf("expected branch preservation warning:\n%s", stderr.String())
	}
}

func TestRemoveResolvesCurrentSessionFromCwd(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := testutil.TempGitRepo(t)
	testHome(t)

	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    repo,
		Getenv: os.Getenv,
	}
	if err := New(ctx, env, []string{"current", "--base", "main", "--editor", "none"}); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(repo), "repo-current")
	env.Cwd = worktree
	if err := Remove(ctx, env, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("worktree still exists or stat failed: %v", err)
	}
}

func TestRemovePrintsCompletionAfterHook(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := testutil.TempGitRepo(t)
	testHome(t)
	if err := os.WriteFile(filepath.Join(repo, ".sessions.toml"), []byte(`on_remove = "printf 'hook complete\n'"`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	env := Env{
		Stdin:  strings.NewReader("y\n"),
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Cwd:    repo,
		Getenv: os.Getenv,
	}
	if err := New(ctx, env, []string{"hooked", "--base", "main", "--editor", "none"}); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := Remove(ctx, env, []string{"hooked"}); err != nil {
		t.Fatal(err)
	}

	output := stdout.String()
	hookIndex := strings.Index(output, "hook complete\n")
	removedIndex := strings.Index(output, "removed session hooked\n")
	if hookIndex < 0 || removedIndex < 0 {
		t.Fatalf("remove output missing hook or completion message:\n%s", output)
	}
	if removedIndex < hookIndex {
		t.Fatalf("completion was printed before hook output:\n%s", output)
	}
	if !strings.HasSuffix(output, "removed session hooked\n") {
		t.Fatalf("completion should be final remove output:\n%s", output)
	}
}

func TestRemoveRunsConfiguredEditorClose(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := testutil.TempGitRepo(t)
	home := testHome(t)
	closeLog := filepath.Join(t.TempDir(), "close.log")
	writeGlobalConfig(t, home, `
default_editor = "cursor"

[editors.cursor]
command = "sh"
args = ["-c", ":"]
close_command = "sh"
close_args = ["-c", "printf '%s\n' \"$1\" > \"$2\"", "close", "{path}", "`+closeLog+`"]
`)

	var stdout bytes.Buffer
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Cwd:    repo,
		Getenv: os.Getenv,
	}
	if err := New(ctx, env, []string{"closeme", "--base", "main", "--editor", "cursor"}); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(repo), "repo-closeme")
	wantPath, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := Remove(ctx, env, []string{"closeme"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(closeLog)
	if err != nil {
		t.Fatal(err)
	}
	gotPath := strings.TrimSpace(string(data))
	if gotPath != wantPath {
		t.Fatalf("close command got %q, want %q", gotPath, wantPath)
	}
	if got := stdout.String(); got != "removed session closeme\n" {
		t.Fatalf("remove output = %q, want %q", got, "removed session closeme\n")
	}
}

func TestRemoveResolvesSessionIDFallback(t *testing.T) {
	ctx := context.Background()
	home := testHome(t)
	configDir := filepath.Join(home, ".sessions")
	store := registry.NewStore(filepath.Join(configDir, "registry.json"))
	worktree := filepath.Join(t.TempDir(), "missing-worktree")
	if err := store.Update(func(reg *registry.Registry) error {
		repo := reg.EnsureRepo("repo", "repo", "/tmp/repo/.git", "/tmp/repo")
		repo.Sessions["s"] = &registry.Session{ID: "s", Name: "work", RepoID: "repo", WorktreePath: worktree, Agents: map[string]*registry.Agent{}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    t.TempDir(),
		Getenv: func(key string) string {
			if key == "SESSION_ID" {
				return "s"
			}
			return os.Getenv(key)
		},
	}
	if err := Remove(ctx, env, []string{"--force"}); err != nil {
		t.Fatal(err)
	}
	reg, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Repos) != 0 {
		t.Fatalf("registry should be empty after remove: %+v", reg)
	}
}

func TestNewHookTrustDecline(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := testutil.TempGitRepo(t)
	testHome(t)
	if err := os.WriteFile(filepath.Join(repo, ".sessions.toml"), []byte(`on_create = "echo create"`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	env := Env{
		Stdin:  strings.NewReader("n\n"),
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
		Cwd:    repo,
		Getenv: os.Getenv,
	}
	err := New(ctx, env, []string{"blocked", "--base", "main", "--editor", "none"})
	if err == nil {
		t.Fatal("expected trust decline to abort")
	}
	if !strings.Contains(stderr.String(), "Trust this repo") {
		t.Fatalf("trust prompt not shown:\n%s", stderr.String())
	}
	store, err := registryStore()
	if err != nil {
		t.Fatal(err)
	}
	reg, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Repos) != 0 {
		t.Fatalf("registry should be empty after trust decline: %+v", reg)
	}
}

func TestAgentSetSessionIDFallback(t *testing.T) {
	ctx := context.Background()
	home := testHome(t)
	configDir := filepath.Join(home, ".sessions")
	store := registry.NewStore(filepath.Join(configDir, "registry.json"))
	if err := store.Update(func(reg *registry.Registry) error {
		repo := reg.EnsureRepo("repo", "repo", "/tmp/repo/.git", "/tmp/repo")
		repo.Sessions["s"] = &registry.Session{ID: "s", Name: "work", RepoID: "repo", WorktreePath: "/tmp/work", Agents: map[string]*registry.Agent{}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    t.TempDir(),
		Getenv: func(key string) string {
			if key == "SESSION_ID" {
				return "s"
			}
			return os.Getenv(key)
		},
	}
	if err := AgentSet(ctx, env, []string{"aider", "--state", "ready"}); err != nil {
		t.Fatal(err)
	}
	reg, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Repos["repo"].Sessions["s"].Agents["aider"].State != "ready" {
		t.Fatalf("agent set did not update session: %+v", reg.Repos["repo"].Sessions["s"].Agents)
	}
}

func TestAgentSetPreservesFocusTargetAndInvokesAttentionCommand(t *testing.T) {
	ctx := context.Background()
	home := testHome(t)
	outPath := filepath.Join(t.TempDir(), "event.json")
	repoRoot := t.TempDir()
	writeGlobalConfig(t, home, `
[attention.command]
command = 'cat > `+strconvQuote(outPath)+`'
`)
	store := registry.NewStore(filepath.Join(home, ".sessions", "registry.json"))
	now := time.Now().UTC()
	if err := store.Update(func(reg *registry.Registry) error {
		repo := reg.EnsureRepo("repo", "repo", filepath.Join(repoRoot, ".git"), repoRoot)
		repo.Sessions["s"] = &registry.Session{
			ID:           "s",
			Name:         "work",
			RepoID:       "repo",
			WorktreePath: "/tmp/work",
			Agents: map[string]*registry.Agent{
				"codex": {
					Name:      "codex",
					State:     registry.AgentRunning,
					UpdatedAt: now,
					FocusTarget: &registry.FocusTarget{
						Provider:   "custom",
						TargetID:   "pane-1",
						AttachedAt: now,
					},
				},
			},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    t.TempDir(),
		Getenv: os.Getenv,
	}
	if err := AgentSet(ctx, env, []string{"codex", "--session", "work", "--state", "needs-input", "--message", "Codex needs input"}); err != nil {
		t.Fatal(err)
	}
	reg, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	agent := reg.Repos["repo"].Sessions["s"].Agents["codex"]
	if agent.State != registry.AgentNeedsInput || agent.FocusTarget == nil || agent.FocusTarget.TargetID != "pane-1" {
		t.Fatalf("agent state update dropped data: %+v", agent)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		Attention   bool `json:"attention"`
		FocusTarget *struct {
			TargetID string `json:"targetId"`
		} `json:"focusTarget"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	if !event.Attention || event.FocusTarget == nil || event.FocusTarget.TargetID != "pane-1" {
		t.Fatalf("unexpected attention event: %+v", event)
	}
}

func TestAgentSetUsesRepoAttentionCommand(t *testing.T) {
	ctx := context.Background()
	home := testHome(t)
	outPath := filepath.Join(t.TempDir(), "event.json")
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".sessions", "attention.sh"), []byte("#!/bin/sh\ncat > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".sessions.toml"), []byte(`
[attention.command]
command = 'sh .sessions/attention.sh `+strconvQuote(outPath)+`'
`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := registry.NewStore(filepath.Join(home, ".sessions", "registry.json"))
	if err := store.Update(func(reg *registry.Registry) error {
		repo := reg.EnsureRepo("repo", "repo", filepath.Join(repoRoot, ".git"), repoRoot)
		repo.Sessions["s"] = &registry.Session{
			ID:           "s",
			Name:         "work",
			RepoID:       "repo",
			WorktreePath: "/tmp/work",
			Agents:       map[string]*registry.Agent{},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    t.TempDir(),
		Getenv: os.Getenv,
	}
	if err := AgentSet(ctx, env, []string{"codex", "--session", "work", "--state", "needs-input"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("repo attention command did not run: %v", err)
	}
}

func TestAgentSetAttentionCommandFailureStillPersistsState(t *testing.T) {
	ctx := context.Background()
	home := testHome(t)
	repoRoot := t.TempDir()
	writeGlobalConfig(t, home, `
[attention.command]
command = "echo boom >&2; exit 7"
`)
	store := registry.NewStore(filepath.Join(home, ".sessions", "registry.json"))
	now := time.Now().UTC()
	if err := store.Update(func(reg *registry.Registry) error {
		repo := reg.EnsureRepo("repo", "repo", filepath.Join(repoRoot, ".git"), repoRoot)
		repo.Sessions["s"] = &registry.Session{
			ID:           "s",
			Name:         "work",
			RepoID:       "repo",
			WorktreePath: "/tmp/work",
			Agents: map[string]*registry.Agent{
				"codex": {
					Name:      "codex",
					State:     registry.AgentRunning,
					UpdatedAt: now,
					FocusTarget: &registry.FocusTarget{
						Provider:   "custom",
						TargetID:   "pane-1",
						AttachedAt: now,
					},
				},
			},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    t.TempDir(),
		Getenv: os.Getenv,
	}
	err := AgentSet(ctx, env, []string{"codex", "--session", "work", "--state", "needs-input", "--message", "still persisted"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected attention command failure with output, got %v", err)
	}
	reg, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	agent := reg.Repos["repo"].Sessions["s"].Agents["codex"]
	if agent.State != registry.AgentNeedsInput || agent.Message != "still persisted" || agent.FocusTarget == nil || agent.FocusTarget.TargetID != "pane-1" {
		t.Fatalf("state update was not persisted before attention command failure: %+v", agent)
	}
}

func TestAgentAttachAndShowJSON(t *testing.T) {
	ctx := context.Background()
	home := testHome(t)
	store := registry.NewStore(filepath.Join(home, ".sessions", "registry.json"))
	if err := store.Update(func(reg *registry.Registry) error {
		repo := reg.EnsureRepo("repo", "repo", "/tmp/repo/.git", "/tmp/repo")
		repo.Sessions["s"] = &registry.Session{ID: "s", Name: "work", RepoID: "repo", WorktreePath: "/tmp/work", Agents: map[string]*registry.Agent{
			"codex": {Name: "codex", State: registry.AgentRunning, Message: "working", UpdatedAt: time.Now().UTC()},
		}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    t.TempDir(),
		Getenv: os.Getenv,
	}
	if err := AgentAttach(ctx, env, []string{"codex", "--session", "work", "--provider", "custom", "--target", "pane-1", "--ttl", "10m", "--pid", strconv.Itoa(os.Getpid()), "--metadata", `{"window":"main"}`}); err != nil {
		t.Fatal(err)
	}
	reg, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	agent := reg.Repos["repo"].Sessions["s"].Agents["codex"]
	if agent.State != registry.AgentRunning || agent.FocusTarget == nil || agent.FocusTarget.Metadata["window"] != "main" {
		t.Fatalf("attach did not preserve state or metadata: %+v", agent)
	}
	var stdout bytes.Buffer
	env.Stdout = &stdout
	if err := AgentShow(ctx, env, []string{"codex", "--session", "work", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"targetId": "pane-1"`) || !strings.Contains(stdout.String(), `"stale": false`) {
		t.Fatalf("show JSON missing focus target details:\n%s", stdout.String())
	}
}

func TestAgentAttachCreatesUnknownAgentAndRejectsInvalidInput(t *testing.T) {
	ctx := context.Background()
	home := testHome(t)
	store := registry.NewStore(filepath.Join(home, ".sessions", "registry.json"))
	if err := store.Update(func(reg *registry.Registry) error {
		repo := reg.EnsureRepo("repo", "repo", "/tmp/repo/.git", "/tmp/repo")
		repo.Sessions["s"] = &registry.Session{ID: "s", Name: "work", RepoID: "repo", WorktreePath: "/tmp/work", Agents: map[string]*registry.Agent{}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    t.TempDir(),
		Getenv: os.Getenv,
	}
	if err := AgentAttach(ctx, env, []string{"new-agent", "--session", "work", "--provider", "custom", "--target", "pane-1"}); err != nil {
		t.Fatal(err)
	}
	reg, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if target := reg.Repos["repo"].Sessions["s"].Agents["new-agent"].FocusTarget; target == nil || target.TargetID != "pane-1" {
		t.Fatalf("unknown agent attach did not create focus target: %+v", reg.Repos["repo"].Sessions["s"].Agents)
	}

	invalidCases := [][]string{
		{"bad-provider", "--session", "work", "--provider", "tmux", "--target", "pane-2"},
		{"missing-target", "--session", "work", "--provider", "custom"},
		{"bad-pid", "--session", "work", "--provider", "custom", "--target", "pane-2", "--pid", "abc"},
		{"bad-ttl", "--session", "work", "--provider", "custom", "--target", "pane-2", "--ttl", "0s"},
		{"bad-json", "--session", "work", "--provider", "custom", "--target", "pane-2", "--metadata", "[]"},
	}
	for _, args := range invalidCases {
		if err := AgentAttach(ctx, env, args); err == nil {
			t.Fatalf("expected AgentAttach(%v) to fail", args)
		}
	}
	reg, err = store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Repos["repo"].Sessions["s"].Agents) != 1 {
		t.Fatalf("invalid attaches mutated registry: %+v", reg.Repos["repo"].Sessions["s"].Agents)
	}
}

func TestAgentCommandsRejectEmptyAgentNames(t *testing.T) {
	ctx := context.Background()
	home := testHome(t)
	store := registry.NewStore(filepath.Join(home, ".sessions", "registry.json"))
	if err := store.Update(func(reg *registry.Registry) error {
		repo := reg.EnsureRepo("repo", "repo", "/tmp/repo/.git", "/tmp/repo")
		repo.Sessions["s"] = &registry.Session{ID: "s", Name: "work", RepoID: "repo", WorktreePath: "/tmp/work", Agents: map[string]*registry.Agent{}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    t.TempDir(),
		Getenv: os.Getenv,
	}
	if err := AgentSet(ctx, env, []string{"", "--session", "work", "--state", "ready"}); err == nil {
		t.Fatal("expected empty agent set name to fail")
	}
	if err := AgentAttach(ctx, env, []string{" ", "--session", "work", "--provider", "custom", "--target", "pane-1"}); err == nil {
		t.Fatal("expected empty agent attach name to fail")
	}
	if err := AgentShow(ctx, env, []string{"", "--session", "work"}); err == nil {
		t.Fatal("expected empty agent show name to fail")
	}
	if err := Focus(ctx, env, []string{"work", "--agent", ""}); err == nil {
		t.Fatal("expected empty focus --agent value to fail")
	}
}

func TestFocusNextUsesRepoScopingAndAllOverride(t *testing.T) {
	if !testutil.IsGitAvailable() {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repoOne := testutil.TempGitRepo(t)
	repoTwo := testutil.TempGitRepo(t)
	home := testHome(t)
	writeGlobalConfig(t, home, `default_editor = "none"`)
	store := registry.NewStore(filepath.Join(home, ".sessions", "registry.json"))
	repoOneID, _, ok := currentRepoID(ctx, repoOne)
	if !ok {
		t.Fatal("could not resolve repo one")
	}
	repoTwoID, _, ok := currentRepoID(ctx, repoTwo)
	if !ok {
		t.Fatal("could not resolve repo two")
	}
	old := time.Now().Add(-time.Hour).UTC()
	if err := store.Update(func(reg *registry.Registry) error {
		r1 := reg.EnsureRepo(repoOneID, "one", filepath.Join(repoOne, ".git"), repoOne)
		r1.Sessions["one-work"] = &registry.Session{ID: "one-work", Name: "work", RepoID: repoOneID, WorktreePath: repoOne, Agents: map[string]*registry.Agent{
			"codex": {Name: "codex", State: registry.AgentNeedsInput, UpdatedAt: old},
		}}
		r2 := reg.EnsureRepo(repoTwoID, "two", filepath.Join(repoTwo, ".git"), repoTwo)
		r2.Sessions["two-work"] = &registry.Session{ID: "two-work", Name: "work", RepoID: repoTwoID, WorktreePath: repoTwo, Agents: map[string]*registry.Agent{
			"claude": {Name: "claude", State: registry.AgentFailed, UpdatedAt: old},
		}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Cwd:    repoOne,
		Getenv: os.Getenv,
	}
	if err := Focus(ctx, env, []string{"--next"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), repoOne) || strings.Contains(stdout.String(), repoTwo) {
		t.Fatalf("repo-scoped focus selected wrong session:\n%s", stdout.String())
	}
	stdout.Reset()
	if err := Focus(ctx, env, []string{"--next", "--all"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), repoTwo) {
		t.Fatalf("--all should select higher-priority repo two session:\n%s", stdout.String())
	}
}

func TestFocusExplicitSessionAndAgent(t *testing.T) {
	ctx := context.Background()
	home := testHome(t)
	writeGlobalConfig(t, home, `default_editor = "none"`)
	store := registry.NewStore(filepath.Join(home, ".sessions", "registry.json"))
	if err := store.Update(func(reg *registry.Registry) error {
		repo := reg.EnsureRepo("repo", "repo", "/tmp/repo/.git", "/tmp/repo")
		repo.Sessions["s"] = &registry.Session{ID: "s", Name: "work", RepoID: "repo", WorktreePath: "/tmp/work", Agents: map[string]*registry.Agent{
			"codex": {Name: "codex", State: registry.AgentNeedsInput, UpdatedAt: time.Now().UTC()},
		}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Cwd:    t.TempDir(),
		Getenv: os.Getenv,
	}
	if err := Focus(ctx, env, []string{"work"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "/tmp/work") {
		t.Fatalf("explicit focus missing worktree path:\n%s", stdout.String())
	}
	stdout.Reset()
	if err := Focus(ctx, env, []string{"repo/work", "--agent", "codex"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "/tmp/work") {
		t.Fatalf("agent focus missing worktree path:\n%s", stdout.String())
	}
	if err := Focus(ctx, env, []string{"repo/work", "--agent", "missing"}); err == nil {
		t.Fatal("expected missing agent error")
	}
}

func TestFocusExplicitSessionAmbiguity(t *testing.T) {
	ctx := context.Background()
	home := testHome(t)
	writeGlobalConfig(t, home, `default_editor = "none"`)
	store := registry.NewStore(filepath.Join(home, ".sessions", "registry.json"))
	if err := store.Update(func(reg *registry.Registry) error {
		one := reg.EnsureRepo("one", "one", "/tmp/one/.git", "/tmp/one")
		one.Sessions["s1"] = &registry.Session{ID: "s1", Name: "work", RepoID: "one", WorktreePath: "/tmp/one-work", Agents: map[string]*registry.Agent{}}
		two := reg.EnsureRepo("two", "two", "/tmp/two/.git", "/tmp/two")
		two.Sessions["s2"] = &registry.Session{ID: "s2", Name: "work", RepoID: "two", WorktreePath: "/tmp/two-work", Agents: map[string]*registry.Agent{}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	env := Env{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Cwd:    t.TempDir(),
		Getenv: os.Getenv,
	}
	err := Focus(ctx, env, []string{"work"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous session error, got %v", err)
	}
}

func testHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func writeGlobalConfig(t *testing.T, home, text string) {
	t.Helper()
	configDir := filepath.Join(home, ".sessions")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func strconvQuote(value string) string {
	return strconv.Quote(value)
}

func branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	err := exec.Command("git", "-C", repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch).Run()
	if err == nil {
		return true
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false
	}
	t.Fatalf("check branch %s: %v", branch, err)
	return false
}
