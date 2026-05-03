package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sessions/internal/config"
	gitx "sessions/internal/git"
)

const (
	defaultCreateHookCommand = `sh "$SESSION_REPO_ROOT/.sessions/create.sh"`
	defaultRemoveHookCommand = `sh "$SESSION_REPO_ROOT/.sessions/remove.sh"`
	defaultRepoConfig        = "on_create = '" + defaultCreateHookCommand + "'\n" +
		"on_remove = '" + defaultRemoveHookCommand + "'\n"
	defaultCreateHook = `#!/usr/bin/env sh
set -eu

# Add repo-specific setup for a new session worktree here.
`
	defaultRemoveHook = `#!/usr/bin/env sh
set -eu

# Add repo-specific cleanup before a session worktree is removed here.
`
)

func Init(ctx context.Context, env Env, args []string) error {
	parsed, err := parseArgs(args, nil, nil)
	if err != nil {
		return err
	}
	if len(parsed.pos) != 0 {
		return fmt.Errorf("usage: sessions init")
	}

	repo, err := gitx.Discover(ctx, env.Cwd)
	if err != nil {
		return err
	}

	configPath := filepath.Join(repo.Root, ".sessions.toml")
	sessionsDir := filepath.Join(repo.Root, ".sessions")
	createPath := filepath.Join(sessionsDir, "create.sh")
	removePath := filepath.Join(sessionsDir, "remove.sh")

	var created []string
	configCreated, err := writeFileIfMissing(configPath, []byte(defaultRepoConfig), 0o644)
	if err != nil {
		return err
	}
	if configCreated {
		created = append(created, rel(repo.Root, configPath))
	}

	needCreateHook := configCreated
	needRemoveHook := configCreated
	if !configCreated {
		repoCfg, err := config.LoadRepo(repo.Root)
		if err != nil {
			return err
		}
		needCreateHook = referencesDefaultHook(repoCfg.OnCreate, "create.sh")
		needRemoveHook = referencesDefaultHook(repoCfg.OnRemove, "remove.sh")
	}
	if needCreateHook || needRemoveHook {
		dirCreated, err := mkdirIfMissing(sessionsDir)
		if err != nil {
			return err
		}
		if dirCreated {
			created = append(created, rel(repo.Root, sessionsDir)+string(os.PathSeparator))
		}
	}
	if needCreateHook {
		createCreated, err := writeFileIfMissing(createPath, []byte(defaultCreateHook), 0o755)
		if err != nil {
			return err
		}
		if createCreated {
			created = append(created, rel(repo.Root, createPath))
		}
	}
	if needRemoveHook {
		removeCreated, err := writeFileIfMissing(removePath, []byte(defaultRemoveHook), 0o755)
		if err != nil {
			return err
		}
		if removeCreated {
			created = append(created, rel(repo.Root, removePath))
		}
	}

	if len(created) == 0 {
		fmt.Fprintf(env.Stdout, "Sessions already initialized in %s\n", repo.Root)
		return nil
	}
	fmt.Fprintf(env.Stdout, "Initialized Sessions in %s\n", repo.Root)
	for _, path := range created {
		fmt.Fprintf(env.Stdout, "  created %s\n", path)
	}
	return nil
}

func referencesDefaultHook(command, script string) bool {
	return strings.Contains(command, ".sessions/"+script)
}

func mkdirIfMissing(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("%s exists but is not a directory", path)
		}
		return false, nil
	}
	if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return false, err
	}
	return true, nil
}

func writeFileIfMissing(path string, data []byte, perm os.FileMode) (bool, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if os.IsExist(err) {
			info, statErr := os.Stat(path)
			if statErr == nil && info.IsDir() {
				return false, fmt.Errorf("%s exists but is a directory", path)
			}
			return false, nil
		}
		return false, err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return false, err
	}
	return true, nil
}

func rel(root, path string) string {
	value, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return value
}
