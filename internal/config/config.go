package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"sessions/internal/paths"
)

type Config struct {
	DefaultEditor string                  `toml:"default_editor" json:"defaultEditor"`
	WorktreePath  string                  `toml:"worktree_path" json:"worktreePath"`
	Attention     AttentionConfig         `toml:"attention" json:"attention"`
	Focus         FocusConfig             `toml:"focus" json:"focus"`
	Editors       map[string]EditorConfig `toml:"editors" json:"editors"`
}

type AttentionConfig struct {
	States    []string               `toml:"states" json:"states"`
	Command   AttentionCommandConfig `toml:"command" json:"command"`
	StatesSet bool                   `toml:"-" json:"-"`
}

type AttentionCommandConfig struct {
	Command string                                 `toml:"command" json:"command"`
	States  map[string]AttentionCommandRouteConfig `toml:"states" json:"states"`
}

type AttentionCommandRouteConfig struct {
	Command string `toml:"command" json:"command"`
}

type FocusConfig struct {
	Providers map[string]FocusProviderConfig `toml:"providers" json:"providers"`
}

type FocusProviderConfig struct {
	Command string `toml:"command" json:"command"`
}

type EditorConfig struct {
	Command      string   `toml:"command" json:"command"`
	Args         []string `toml:"args" json:"args"`
	CloseCommand string   `toml:"close_command" json:"closeCommand"`
	CloseArgs    []string `toml:"close_args" json:"closeArgs"`
}

type RepoConfig struct {
	Path          string
	Present       bool
	Name          string                  `toml:"name" json:"name"`
	DefaultEditor string                  `toml:"default_editor" json:"defaultEditor"`
	WorktreePath  string                  `toml:"worktree_path" json:"worktreePath"`
	Editor        string                  `toml:"editor" json:"editor"`
	BaseRef       string                  `toml:"base_ref" json:"baseRef"`
	OnCreate      string                  `toml:"on_create" json:"onCreate"`
	OnRemove      string                  `toml:"on_remove" json:"onRemove"`
	Attention     RepoAttentionConfig     `toml:"attention" json:"attention"`
	Focus         FocusConfig             `toml:"focus" json:"focus"`
	Editors       map[string]EditorConfig `toml:"editors" json:"editors"`
}

type RepoAttentionConfig struct {
	States    []string               `toml:"states" json:"states"`
	Command   AttentionCommandConfig `toml:"command" json:"command"`
	StatesSet bool                   `toml:"-" json:"-"`
}

type Effective struct {
	RepoName     string
	WorktreePath string
	Editor       string
	BaseRef      string
	OnCreate     string
	OnRemove     string
	Attention    AttentionConfig
	Focus        FocusConfig
	Editors      map[string]EditorConfig
}

func Default() Config {
	return Config{
		DefaultEditor: "code",
		WorktreePath:  "../{repo}-{name}",
		Attention: AttentionConfig{
			States:  []string{"needs-input", "failed"},
			Command: AttentionCommandConfig{},
		},
		Focus: FocusConfig{
			Providers: map[string]FocusProviderConfig{},
		},
		Editors: map[string]EditorConfig{
			"code": {
				Command: "code",
				Args:    []string{"--new-window", "{path}"},
			},
			"cursor": {
				Command: "cursor",
				Args:    []string{"--new-window", "{path}"},
			},
			"zed": {
				Command: "zed",
				Args:    []string{"{path}"},
			},
		},
	}
}

func LoadGlobal() (Config, error) {
	path, err := paths.ConfigPath()
	if err != nil {
		return Config{}, err
	}
	return LoadGlobalPath(path)
}

func LoadGlobalPath(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.Attention.StatesSet = hasTOMLPath(data, "attention", "states")
	fillConfigDefaults(&cfg)
	return cfg, nil
}

func LoadRepo(root string) (RepoConfig, error) {
	path := filepath.Join(root, ".sessions.toml")
	cfg := RepoConfig{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read repo config %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse repo config %s: %w", path, err)
	}
	cfg.Attention.StatesSet = hasTOMLPath(data, "attention", "states")
	cfg.Present = true
	return cfg, nil
}

func Merge(global Config, repo RepoConfig, fallbackRepoName string) Effective {
	fillConfigDefaults(&global)
	name := repo.Name
	if name == "" {
		name = fallbackRepoName
	}
	worktreePath := global.WorktreePath
	if repo.WorktreePath != "" {
		worktreePath = repo.WorktreePath
	}
	editor := global.DefaultEditor
	if repo.DefaultEditor != "" {
		editor = repo.DefaultEditor
	}
	if repo.Editor != "" {
		editor = repo.Editor
	}
	attention := mergeAttention(global.Attention, repo.Attention)
	focus := mergeFocus(global.Focus, repo.Focus)
	editors := mergeEditors(global.Editors, repo.Editors)
	return Effective{
		RepoName:     name,
		WorktreePath: worktreePath,
		Editor:       editor,
		BaseRef:      repo.BaseRef,
		OnCreate:     repo.OnCreate,
		OnRemove:     repo.OnRemove,
		Attention:    attention,
		Focus:        focus,
		Editors:      editors,
	}
}

func fillConfigDefaults(cfg *Config) {
	def := Default()
	if cfg.DefaultEditor == "" {
		cfg.DefaultEditor = def.DefaultEditor
	}
	if cfg.WorktreePath == "" {
		cfg.WorktreePath = def.WorktreePath
	}
	if len(cfg.Attention.States) == 0 && !cfg.Attention.StatesSet {
		cfg.Attention.States = def.Attention.States
	}
	if cfg.Attention.Command.States == nil {
		cfg.Attention.Command.States = map[string]AttentionCommandRouteConfig{}
	}
	if cfg.Focus.Providers == nil {
		cfg.Focus.Providers = map[string]FocusProviderConfig{}
	}
	if cfg.Editors == nil {
		cfg.Editors = map[string]EditorConfig{}
	}
	for name, editor := range def.Editors {
		if _, ok := cfg.Editors[name]; !ok {
			cfg.Editors[name] = editor
		}
	}
}

func mergeAttention(global AttentionConfig, repo RepoAttentionConfig) AttentionConfig {
	merged := AttentionConfig{
		States: append([]string(nil), global.States...),
		Command: AttentionCommandConfig{
			Command: global.Command.Command,
			States:  map[string]AttentionCommandRouteConfig{},
		},
	}
	for state, route := range global.Command.States {
		merged.Command.States[state] = route
	}
	if repo.Command.Command != "" {
		merged.Command.Command = repo.Command.Command
	}
	if repo.StatesSet {
		merged.States = append([]string(nil), repo.States...)
	}
	for state, route := range repo.Command.States {
		merged.Command.States[state] = route
	}
	return merged
}

func hasTOMLPath(data []byte, path ...string) bool {
	var root map[string]any
	if err := toml.Unmarshal(data, &root); err != nil {
		return false
	}
	var current any = root
	for i, key := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return false
		}
		value, ok := m[key]
		if !ok {
			return false
		}
		if i == len(path)-1 {
			return true
		}
		current = value
	}
	return false
}

func mergeFocus(global, repo FocusConfig) FocusConfig {
	merged := FocusConfig{Providers: map[string]FocusProviderConfig{}}
	for name, provider := range global.Providers {
		merged.Providers[name] = provider
	}
	for name, provider := range repo.Providers {
		current := merged.Providers[name]
		if provider.Command != "" {
			current.Command = provider.Command
		}
		merged.Providers[name] = current
	}
	return merged
}

func mergeEditors(global, repo map[string]EditorConfig) map[string]EditorConfig {
	merged := map[string]EditorConfig{}
	for name, editor := range global {
		merged[name] = cloneEditor(editor)
	}
	for name, editor := range repo {
		current := merged[name]
		if editor.Command != "" {
			current.Command = editor.Command
		}
		if len(editor.Args) > 0 {
			current.Args = append([]string(nil), editor.Args...)
		}
		if editor.CloseCommand != "" {
			current.CloseCommand = editor.CloseCommand
		}
		if len(editor.CloseArgs) > 0 {
			current.CloseArgs = append([]string(nil), editor.CloseArgs...)
		}
		merged[name] = current
	}
	return merged
}

func cloneEditor(editor EditorConfig) EditorConfig {
	return EditorConfig{
		Command:      editor.Command,
		Args:         append([]string(nil), editor.Args...),
		CloseCommand: editor.CloseCommand,
		CloseArgs:    append([]string(nil), editor.CloseArgs...),
	}
}
