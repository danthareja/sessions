package hooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"sessions/internal/registry"
	"sessions/internal/session"
)

func Run(ctx context.Context, command string, repo *registry.Repo, sess *registry.Session, stdout, stderr io.Writer) error {
	if command == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = sess.WorktreePath
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = os.Environ()
	for key, value := range session.EnvMap(repo, sess) {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %q failed: %w", command, err)
	}
	return nil
}
