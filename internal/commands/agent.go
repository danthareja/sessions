package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"sessions/internal/attention"
	"sessions/internal/config"
	"sessions/internal/events"
	"sessions/internal/focus"
	"sessions/internal/registry"
)

const agentStateUsage = "running|needs-input|ready|failed|idle"

func Agent(ctx context.Context, env Env, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: sessions agent set|attach|show ...")
	}
	switch args[0] {
	case "set":
		return AgentSet(ctx, env, args[1:])
	case "attach":
		return AgentAttach(ctx, env, args[1:])
	case "show":
		return AgentShow(ctx, env, args[1:])
	default:
		return fmt.Errorf("usage: sessions agent set|attach|show ...")
	}
}

func AgentSet(ctx context.Context, env Env, args []string) error {
	parsed, err := parseArgs(args,
		map[string]bool{"state": true, "message": true, "session": true},
		nil,
	)
	if err != nil {
		return err
	}
	if len(parsed.pos) != 1 {
		return fmt.Errorf("usage: sessions agent set <agent> --state %s [--message <text>] [--session <session>]", agentStateUsage)
	}
	agentName := parsed.pos[0]
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	state := parsed.strings["state"]
	if !registry.ValidAgentState(state) {
		return fmt.Errorf("invalid --state %q; expected running, needs-input, ready, failed, or idle", state)
	}
	message := parsed.strings["message"]

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	store, err := registryStore()
	if err != nil {
		return err
	}
	currentID, currentRoot, _ := currentRepoID(ctx, env.Cwd)
	sessionID := env.getenv("SESSION_ID")
	now := time.Now().UTC()

	var repoForEvent *registry.Repo
	var sessionForEvent *registry.Session
	var agentForEvent *registry.Agent
	if err := store.Update(func(reg *registry.Registry) error {
		repo, sess, err := resolveStateSession(reg, currentID, currentRoot, sessionID, parsed.strings["session"])
		if err != nil {
			return err
		}
		if sess.Agents == nil {
			sess.Agents = map[string]*registry.Agent{}
		}
		agent := sess.Agents[agentName]
		if agent == nil {
			agent = &registry.Agent{Name: agentName}
			sess.Agents[agentName] = agent
		}
		agent.Name = agentName
		agent.State = state
		agent.Message = message
		agent.UpdatedAt = now
		sess.UpdatedAt = now
		repoForEvent = repo
		sessionForEvent = sess
		agentForEvent = agent
		return nil
	}); err != nil {
		return err
	}

	effective, err := effectiveForRepo(globalCfg, repoForEvent)
	if err != nil {
		return err
	}
	policy := attention.New(effective.Attention.States)
	targetStatus := events.TargetStatus{}
	if agentForEvent.FocusTarget != nil {
		status := focus.Status(agentForEvent.FocusTarget, time.Now().UTC())
		targetStatus = events.TargetStatus{Stale: status.Stale, Reason: status.Reason}
	}
	event := events.NewAgent(repoForEvent, sessionForEvent, agentForEvent, policy.Attention(state), targetStatus)
	if err := attention.Command(ctx, effective.Attention, event); err != nil {
		return err
	}
	return nil
}

func AgentAttach(ctx context.Context, env Env, args []string) error {
	parsed, err := parseArgs(args,
		map[string]bool{"provider": true, "target": true, "session": true, "pid": true, "ttl": true, "metadata": true},
		nil,
	)
	if err != nil {
		return err
	}
	if len(parsed.pos) != 1 {
		return fmt.Errorf("usage: sessions agent attach <agent> --provider code|cursor|zed|custom --target <id> [--session <session>] [--pid <pid>] [--ttl <duration>] [--metadata <json>]")
	}
	agentName := parsed.pos[0]
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	provider := parsed.strings["provider"]
	if !focus.ValidProvider(provider) {
		return fmt.Errorf("invalid --provider %q; expected code, cursor, zed, or custom", provider)
	}
	targetID := parsed.strings["target"]
	if targetID == "" {
		return fmt.Errorf("--target is required")
	}
	pid, err := parseOptionalPositiveInt(parsed.strings["pid"], "--pid")
	if err != nil {
		return err
	}
	metadata, err := parseMetadata(parsed.strings["metadata"])
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var expiresAt *time.Time
	if ttlText := parsed.strings["ttl"]; ttlText != "" {
		ttl, err := time.ParseDuration(ttlText)
		if err != nil || ttl <= 0 {
			return fmt.Errorf("invalid --ttl %q; expected a positive duration like 10m", ttlText)
		}
		expires := now.Add(ttl)
		expiresAt = &expires
	}

	store, err := registryStore()
	if err != nil {
		return err
	}
	currentID, currentRoot, _ := currentRepoID(ctx, env.Cwd)
	sessionID := env.getenv("SESSION_ID")
	return store.Update(func(reg *registry.Registry) error {
		_, sess, err := resolveStateSession(reg, currentID, currentRoot, sessionID, parsed.strings["session"])
		if err != nil {
			return err
		}
		if sess.Agents == nil {
			sess.Agents = map[string]*registry.Agent{}
		}
		agent := sess.Agents[agentName]
		if agent == nil {
			agent = &registry.Agent{Name: agentName}
			sess.Agents[agentName] = agent
		}
		agent.Name = agentName
		agent.FocusTarget = &registry.FocusTarget{
			Provider:   provider,
			TargetID:   targetID,
			Metadata:   metadata,
			PID:        pid,
			AttachedAt: now,
			ExpiresAt:  expiresAt,
		}
		agent.UpdatedAt = now
		sess.UpdatedAt = now
		return nil
	})
}

func AgentShow(ctx context.Context, env Env, args []string) error {
	parsed, err := parseArgs(args,
		map[string]bool{"session": true},
		map[string]bool{"json": true},
	)
	if err != nil {
		return err
	}
	if len(parsed.pos) != 1 {
		return fmt.Errorf("usage: sessions agent show <agent> [--session <session>] [--json]")
	}
	agentName := parsed.pos[0]
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	store, err := registryStore()
	if err != nil {
		return err
	}
	reg, err := store.Read()
	if err != nil {
		return err
	}
	currentID, currentRoot, _ := currentRepoID(ctx, env.Cwd)
	repo, sess, err := resolveStateSession(reg, currentID, currentRoot, env.getenv("SESSION_ID"), parsed.strings["session"])
	if err != nil {
		return err
	}
	agent := sess.Agents[agentName]
	if agent == nil {
		return fmt.Errorf("agent %q not found in session %q", agentName, sess.Name)
	}
	status := focus.Status(agent.FocusTarget, time.Now().UTC())
	if parsed.bools["json"] {
		data, err := json.MarshalIndent(agentShowEvent(repo, sess, agent, status), "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(env.Stdout, string(data))
		return nil
	}
	fmt.Fprintf(env.Stdout, "agent: %s\n", agent.Name)
	fmt.Fprintf(env.Stdout, "session: %s/%s\n", repo.Name, sess.Name)
	fmt.Fprintf(env.Stdout, "state: %s\n", emptyDash(agent.State))
	fmt.Fprintf(env.Stdout, "message: %s\n", emptyDash(agent.Message))
	if agent.FocusTarget == nil {
		fmt.Fprintln(env.Stdout, "focus target: none")
		return nil
	}
	fmt.Fprintf(env.Stdout, "focus target: %s %s\n", agent.FocusTarget.Provider, agent.FocusTarget.TargetID)
	if status.Stale {
		fmt.Fprintf(env.Stdout, "stale: yes (%s)\n", status.Reason)
	} else {
		fmt.Fprintln(env.Stdout, "stale: no")
	}
	return nil
}

func resolveStateSession(reg *registry.Registry, currentID, currentRoot, sessionID, explicit string) (*registry.Repo, *registry.Session, error) {
	if explicit != "" {
		return resolveSessionByName(reg, currentID, explicit)
	}
	if repo, sess, ok := resolveSessionByWorktree(reg, currentRoot); ok {
		return repo, sess, nil
	}
	if repo, sess, ok := resolveSessionByID(reg, sessionID); ok {
		return repo, sess, nil
	}
	return nil, nil, fmt.Errorf("could not resolve session from cwd; pass --session <session>")
}

func parseOptionalPositiveInt(value, flag string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid %s %q; expected a positive integer", flag, value)
	}
	return parsed, nil
}

func validateAgentName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("agent name is required")
	}
	return nil
}

func parseMetadata(value string) (map[string]any, error) {
	if value == "" {
		return nil, nil
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(value), &metadata); err != nil {
		return nil, fmt.Errorf("invalid --metadata JSON: %w", err)
	}
	if metadata == nil {
		return nil, fmt.Errorf("invalid --metadata JSON: expected an object")
	}
	return metadata, nil
}

func agentShowEvent(repo *registry.Repo, sess *registry.Session, agent *registry.Agent, status focus.TargetStatus) any {
	return events.NewAgent(repo, sess, agent, false, events.TargetStatus{Stale: status.Stale, Reason: status.Reason})
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
