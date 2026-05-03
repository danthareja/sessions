package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGlobalDefaultsWhenMissing(t *testing.T) {
	cfg, err := LoadGlobalPath(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultEditor != "code" {
		t.Fatalf("default editor = %q", cfg.DefaultEditor)
	}
	if cfg.WorktreePath != "../{repo}-{name}" {
		t.Fatalf("worktree path = %q", cfg.WorktreePath)
	}
	if len(cfg.Attention.States) != 2 || cfg.Attention.States[0] != "needs-input" || cfg.Attention.States[1] != "failed" {
		t.Fatalf("attention defaults = %+v", cfg.Attention.States)
	}
	if cfg.Attention.Command.Command != "" {
		t.Fatalf("attention command should default empty: %+v", cfg.Attention.Command)
	}
}

func TestLoadGlobalAndMergeRepo(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(`
default_editor = "zed"
worktree_path = "../global-{name}"

[attention]
states = ["ready"]

[attention.command]
command = "attention-default"

[attention.command.states.failed]
command = "attention-failed"

[focus.providers.custom]
command = "focus-custom"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadGlobalPath(globalPath)
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".sessions.toml"), []byte(`
name = "friendly"
worktree_path = "../repo-{name}"
editor = "none"
base_ref = "main"
on_create = "./bin/create"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	repoCfg, err := LoadRepo(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	effective := Merge(cfg, repoCfg, "fallback")
	if effective.RepoName != "friendly" || effective.WorktreePath != "../repo-{name}" || effective.Editor != "none" || effective.BaseRef != "main" {
		t.Fatalf("unexpected effective config: %+v", effective)
	}
	if len(effective.Attention.States) != 1 || effective.Attention.States[0] != "ready" {
		t.Fatalf("attention states did not merge: %+v", effective.Attention.States)
	}
	if effective.Attention.Command.Command != "attention-default" || effective.Attention.Command.States["failed"].Command != "attention-failed" {
		t.Fatalf("attention command config did not merge: %+v", effective.Attention.Command)
	}
	if effective.Focus.Providers["custom"].Command != "focus-custom" {
		t.Fatalf("focus provider config did not merge: %+v", effective.Focus.Providers)
	}
}

func TestRepoAttentionCommandOverridesGlobal(t *testing.T) {
	global := Default()
	global.Attention.States = []string{"needs-input", "failed"}
	global.Attention.Command.Command = "global-attention"
	global.Attention.Command.States = map[string]AttentionCommandRouteConfig{
		"failed": {Command: "global-failed"},
	}
	repo := RepoConfig{
		Attention: RepoAttentionConfig{
			States:    []string{"ready"},
			StatesSet: true,
			Command: AttentionCommandConfig{
				Command: "repo-attention",
				States: map[string]AttentionCommandRouteConfig{
					"needs-input": {Command: "repo-needs-input"},
				},
			},
		},
	}
	effective := Merge(global, repo, "repo")
	if effective.Attention.Command.Command != "repo-attention" {
		t.Fatalf("repo command did not override global: %+v", effective.Attention.Command)
	}
	if len(effective.Attention.States) != 1 || effective.Attention.States[0] != "ready" {
		t.Fatalf("repo states did not override global: %+v", effective.Attention.States)
	}
	if effective.Attention.Command.States["failed"].Command != "global-failed" {
		t.Fatalf("global state override was not preserved: %+v", effective.Attention.Command.States)
	}
	if effective.Attention.Command.States["needs-input"].Command != "repo-needs-input" {
		t.Fatalf("repo state override was not applied: %+v", effective.Attention.Command.States)
	}
}

func TestRepoConfigOverlaysGlobalSections(t *testing.T) {
	global := Default()
	global.DefaultEditor = "zed"
	global.WorktreePath = "../global-{name}"
	global.Attention.States = []string{"needs-input"}
	global.Attention.Command.Command = "global-attention"
	global.Focus.Providers = map[string]FocusProviderConfig{
		"custom": {Command: "global-focus"},
	}
	global.Editors["cursor"] = EditorConfig{
		Command: "cursor-global",
		Args:    []string{"--new-window", "{path}"},
	}

	repo := RepoConfig{
		DefaultEditor: "cursor",
		WorktreePath:  "../repo-{name}",
		Attention: RepoAttentionConfig{
			States:    []string{"ready"},
			StatesSet: true,
			Command: AttentionCommandConfig{
				Command: "repo-attention",
			},
		},
		Focus: FocusConfig{
			Providers: map[string]FocusProviderConfig{
				"custom": {Command: "repo-focus"},
			},
		},
		Editors: map[string]EditorConfig{
			"cursor": {
				Command: "cursor-repo",
				Args:    []string{"--reuse-window", "{path}"},
			},
		},
	}

	effective := Merge(global, repo, "repo")
	if effective.Editor != "cursor" {
		t.Fatalf("repo default editor did not override global: %+v", effective)
	}
	if effective.WorktreePath != "../repo-{name}" {
		t.Fatalf("repo worktree path did not override global: %+v", effective)
	}
	if len(effective.Attention.States) != 1 || effective.Attention.States[0] != "ready" || effective.Attention.Command.Command != "repo-attention" {
		t.Fatalf("repo attention did not override global: %+v", effective.Attention)
	}
	if effective.Focus.Providers["custom"].Command != "repo-focus" {
		t.Fatalf("repo focus provider did not override global: %+v", effective.Focus.Providers)
	}
	editor := effective.Editors["cursor"]
	if editor.Command != "cursor-repo" || len(editor.Args) != 2 || editor.Args[0] != "--reuse-window" {
		t.Fatalf("repo editor config did not override global: %+v", editor)
	}
}

func TestExplicitEmptyAttentionStatesDoNotDefault(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(`
[attention]
states = []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadGlobalPath(globalPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Attention.States) != 0 {
		t.Fatalf("explicit empty global states should be preserved: %+v", cfg.Attention.States)
	}

	global := Default()
	global.Attention.States = []string{"needs-input"}
	repoRoot := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".sessions.toml"), []byte(`
[attention]
states = []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	repo, err := LoadRepo(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	effective := Merge(global, repo, "repo")
	if len(effective.Attention.States) != 0 {
		t.Fatalf("explicit empty repo states should override global: %+v", effective.Attention.States)
	}
}

func TestInvalidTOMLReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("not = [toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGlobalPath(path); err == nil {
		t.Fatal("expected invalid TOML error")
	}
}

func TestEmptyNestedCommandMapsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
[attention.command]
command = "attention"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadGlobalPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Attention.Command.States == nil {
		t.Fatal("attention command states map should be initialized")
	}
	if cfg.Focus.Providers == nil {
		t.Fatal("focus providers map should be initialized")
	}
}
