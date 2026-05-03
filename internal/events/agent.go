package events

import (
	"time"

	"sessions/internal/registry"
)

type TargetStatus struct {
	Stale  bool
	Reason string
}

type Event struct {
	Attention   bool         `json:"attention"`
	Repo        EventRepo    `json:"repo"`
	Session     EventSession `json:"session"`
	Agent       *EventAgent  `json:"agent,omitempty"`
	FocusTarget *EventTarget `json:"focusTarget,omitempty"`
}

type EventRepo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CommonDir    string `json:"commonDir"`
	MainWorktree string `json:"mainWorktree"`
}

type EventSession struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Index        int    `json:"index"`
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktreePath"`
}

type EventAgent struct {
	Name      string    `json:"name"`
	State     string    `json:"state"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type EventTarget struct {
	Provider    string         `json:"provider"`
	TargetID    string         `json:"targetId"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	PID         int            `json:"pid,omitempty"`
	AttachedAt  time.Time      `json:"attachedAt"`
	ExpiresAt   *time.Time     `json:"expiresAt,omitempty"`
	Stale       bool           `json:"stale"`
	StaleReason string         `json:"staleReason,omitempty"`
}

func NewAgent(repo *registry.Repo, sess *registry.Session, agent *registry.Agent, attention bool, targetStatus TargetStatus) Event {
	event := Event{
		Attention: attention,
		Repo: EventRepo{
			ID:           repo.ID,
			Name:         repo.Name,
			CommonDir:    repo.CommonDir,
			MainWorktree: repo.MainWorktree,
		},
		Session: EventSession{
			ID:           sess.ID,
			Name:         sess.Name,
			Index:        sess.Index,
			Branch:       sess.Branch,
			WorktreePath: sess.WorktreePath,
		},
	}
	if agent == nil {
		return event
	}
	event.Agent = &EventAgent{
		Name:      agent.Name,
		State:     agent.State,
		Message:   agent.Message,
		UpdatedAt: agent.UpdatedAt,
	}
	if agent.FocusTarget != nil {
		event.FocusTarget = &EventTarget{
			Provider:    agent.FocusTarget.Provider,
			TargetID:    agent.FocusTarget.TargetID,
			Metadata:    agent.FocusTarget.Metadata,
			PID:         agent.FocusTarget.PID,
			AttachedAt:  agent.FocusTarget.AttachedAt,
			ExpiresAt:   agent.FocusTarget.ExpiresAt,
			Stale:       targetStatus.Stale,
			StaleReason: targetStatus.Reason,
		}
	}
	return event
}
