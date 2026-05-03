package attention

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"sessions/internal/config"
	"sessions/internal/events"
)

func Command(ctx context.Context, cfg config.AttentionConfig, event events.Event) error {
	if !event.Attention || event.Agent == nil {
		return nil
	}
	command := cfg.Command.Command
	if override := cfg.Command.States[event.Agent.State]; override.Command != "" {
		command = override.Command
	}
	if command == "" {
		return nil
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode attention event: %w", err)
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if event.Repo.MainWorktree != "" {
		cmd.Dir = event.Repo.MainWorktree
	}
	cmd.Stdin = bytes.NewReader(data)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("attention command failed: %w: %s", err, bytes.TrimSpace(out))
	}
	return nil
}
