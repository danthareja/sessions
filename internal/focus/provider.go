package focus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"

	"sessions/internal/attention"
	"sessions/internal/config"
	"sessions/internal/editor"
	"sessions/internal/events"
	"sessions/internal/registry"
)

func Route(ctx context.Context, effective config.Effective, repo *registry.Repo, sess *registry.Session, agent *registry.Agent, stdout, stderr io.Writer) error {
	policy := attention.New(effective.Attention.States)
	isAttention := agent != nil && policy.Attention(agent.State)
	now := time.Now().UTC()

	if agent != nil && agent.FocusTarget != nil {
		target := agent.FocusTarget
		status := Status(target, now)
		if status.Stale {
			fmt.Fprintf(stderr, "warning: ignoring stale focus target %s:%s: %s\n", target.Provider, target.TargetID, status.Reason)
		} else if command := providerCommand(effective.Focus, target.Provider); command != "" {
			event := events.NewAgent(repo, sess, agent, isAttention, events.TargetStatus{Stale: status.Stale, Reason: status.Reason})
			if err := runCommand(ctx, command, event); err != nil {
				fmt.Fprintf(stderr, "warning: exact focus provider %q failed: %v\n", target.Provider, err)
			} else {
				fmt.Fprintf(stdout, "focused %s/%s agent %s via %s target %s\n", repo.Name, sess.Name, agent.Name, target.Provider, target.TargetID)
				return nil
			}
		} else if EditorProvider(target.Provider) {
			if openEditor(effective, target.Provider, sess.WorktreePath, stdout, stderr) {
				fmt.Fprintf(stdout, "focused %s/%s with %s\n", repo.Name, sess.Name, target.Provider)
				return nil
			}
		} else {
			fmt.Fprintf(stderr, "warning: focus target provider %q has no configured command; opening session editor\n", target.Provider)
		}
	}

	if openEditor(effective, effective.Editor, sess.WorktreePath, stdout, stderr) {
		fmt.Fprintf(stdout, "focused %s/%s at %s\n", repo.Name, sess.Name, sess.WorktreePath)
	}
	return nil
}

func providerCommand(cfg config.FocusConfig, provider string) string {
	if cfg.Providers == nil {
		return ""
	}
	return cfg.Providers[provider].Command
}

func openEditor(effective config.Effective, editorName, worktreePath string, stdout, stderr io.Writer) bool {
	resolved, err := editor.Resolve(effective.Editors, editorName, worktreePath)
	if err != nil {
		fmt.Fprintf(stderr, "warning: %v\n", err)
		printManualFallback(stdout, worktreePath, "")
		return false
	}
	if resolved.Command == "" {
		printManualFallback(stdout, worktreePath, "")
		return false
	}
	if err := editor.Open(resolved); err != nil {
		fmt.Fprintf(stderr, "warning: %v\n", err)
		printManualFallback(stdout, worktreePath, resolved.CommandLine())
		return false
	}
	return true
}

func printManualFallback(stdout io.Writer, worktreePath, commandLine string) {
	fmt.Fprintf(stdout, "open worktree manually: %s\n", worktreePath)
	if commandLine != "" {
		fmt.Fprintf(stdout, "editor command: %s\n", commandLine)
	}
}

func runCommand(ctx context.Context, command string, event events.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode focus event: %w", err)
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdin = bytes.NewReader(data)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("provider command failed: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}
