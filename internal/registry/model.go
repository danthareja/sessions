package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"time"
)

const SupportedVersion = 1

const (
	AgentRunning    = "running"
	AgentNeedsInput = "needs-input"
	AgentReady      = "ready"
	AgentFailed     = "failed"
	AgentIdle       = "idle"
)

const (
	SetupCreating = "creating"
	SetupOK       = "ok"
	SetupFailed   = "failed"
	SetupSkipped  = "skipped"
)

type Registry struct {
	Version int              `json:"version"`
	Repos   map[string]*Repo `json:"repos"`
}

type Repo struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	CommonDir    string              `json:"commonDir"`
	MainWorktree string              `json:"mainWorktree"`
	Sessions     map[string]*Session `json:"sessions"`
}

type Session struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	RepoID       string            `json:"repoId"`
	Index        int               `json:"index"`
	BaseRef      string            `json:"baseRef"`
	Branch       string            `json:"branch"`
	WorktreePath string            `json:"worktreePath"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	Setup        SetupState        `json:"setup"`
	Agents       map[string]*Agent `json:"agents"`
}

type SetupState struct {
	State   string `json:"state"`
	Message string `json:"message"`
}

type Agent struct {
	Name        string       `json:"name"`
	State       string       `json:"state"`
	Message     string       `json:"message"`
	UpdatedAt   time.Time    `json:"updatedAt"`
	FocusTarget *FocusTarget `json:"focusTarget,omitempty"`
}

type FocusTarget struct {
	Provider   string         `json:"provider"`
	TargetID   string         `json:"targetId"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	PID        int            `json:"pid,omitempty"`
	AttachedAt time.Time      `json:"attachedAt"`
	ExpiresAt  *time.Time     `json:"expiresAt,omitempty"`
}

func Empty() *Registry {
	return &Registry{
		Version: SupportedVersion,
		Repos:   map[string]*Repo{},
	}
}

func RepoID(commonDir string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(commonDir)))
	return hex.EncodeToString(sum[:])[:16]
}

func (r *Registry) Ensure() {
	if r.Version == 0 {
		r.Version = SupportedVersion
	}
	if r.Repos == nil {
		r.Repos = map[string]*Repo{}
	}
}

func (r *Registry) EnsureRepo(id, name, commonDir, mainWorktree string) *Repo {
	r.Ensure()
	repo, ok := r.Repos[id]
	if !ok {
		repo = &Repo{
			ID:       id,
			Sessions: map[string]*Session{},
		}
		r.Repos[id] = repo
	}
	repo.Name = name
	repo.CommonDir = commonDir
	repo.MainWorktree = mainWorktree
	if repo.Sessions == nil {
		repo.Sessions = map[string]*Session{}
	}
	return repo
}

func ValidAgentState(state string) bool {
	switch state {
	case AgentRunning, AgentNeedsInput, AgentReady, AgentFailed, AgentIdle:
		return true
	default:
		return false
	}
}

func AgentPriority(state string) int {
	switch state {
	case AgentFailed:
		return 5
	case AgentNeedsInput:
		return 4
	case AgentReady:
		return 3
	case AgentRunning:
		return 2
	case AgentIdle:
		return 1
	default:
		return 0
	}
}
