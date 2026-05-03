package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sessions/internal/config"
	"sessions/internal/editor"
	gitx "sessions/internal/git"
	"sessions/internal/hooks"
	"sessions/internal/paths"
	"sessions/internal/registry"
	sessionpkg "sessions/internal/session"
	"sessions/internal/trust"
)

func New(ctx context.Context, env Env, args []string) error {
	parsed, err := parseArgs(args,
		map[string]bool{"base": true, "branch": true, "editor": true},
		map[string]bool{"no-setup": true},
	)
	if err != nil {
		return err
	}
	if len(parsed.pos) != 1 {
		return fmt.Errorf("usage: sessions new <name> [--base <ref>] [--branch <branch>] [--editor <name>] [--no-setup]")
	}
	name := parsed.pos[0]
	if err := sessionpkg.ValidateName(name); err != nil {
		return err
	}

	repo, err := gitx.Discover(ctx, env.Cwd)
	if err != nil {
		return err
	}
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return err
	}
	repoCfg, err := config.LoadRepo(repo.Root)
	if err != nil {
		return err
	}
	effective := config.Merge(globalCfg, repoCfg, filepath.Base(repo.MainWorktree))

	baseRef := firstNonEmpty(parsed.strings["base"], effective.BaseRef)
	if baseRef == "" {
		baseRef, err = gitx.DefaultBranch(ctx, repo.Root)
		if err != nil {
			return err
		}
	}
	branch := firstNonEmpty(parsed.strings["branch"], name)
	if err := gitx.ValidateBranchName(ctx, repo.Root, branch); err != nil {
		return err
	}
	if gitx.BranchExists(ctx, repo.Root, branch) {
		fmt.Fprintf(env.Stderr, "warning: branch %s already exists; choose a different session name or pass --branch\n", branch)
		return fmt.Errorf("branch %q already exists", branch)
	}

	editorName := effective.Editor
	if override, ok := parsed.strings["editor"]; ok {
		editorName = override
	}

	if effective.OnCreate != "" && !parsed.bools["no-setup"] {
		trustPath, err := paths.TrustedPath()
		if err != nil {
			return err
		}
		if err := trust.NewStore(trustPath).Require(registry.RepoID(repo.CommonDir), repo.CommonDir, repoCfg.Path, effective.OnCreate, effective.OnRemove, env.Stdin, env.Stderr); err != nil {
			return err
		}
	}

	store, err := registryStore()
	if err != nil {
		return err
	}

	repoID := registry.RepoID(repo.CommonDir)
	now := time.Now().UTC()
	var createdRepo *registry.Repo
	var createdSession *registry.Session
	if err := store.Update(func(reg *registry.Registry) error {
		repoEntry := reg.EnsureRepo(repoID, effective.RepoName, repo.CommonDir, repo.MainWorktree)
		for _, existing := range repoEntry.Sessions {
			if existing.Name == name {
				return fmt.Errorf("session %q already exists in repo %q", name, repoEntry.Name)
			}
		}
		idx := sessionpkg.AllocateIndex(repoEntry)
		worktreePath := renderWorktreePath(effective.WorktreePath, repo.MainWorktree, effective.RepoName, name, branch, idx)
		if _, err := os.Stat(worktreePath); err == nil {
			return fmt.Errorf("worktree path already exists: %s", worktreePath)
		}
		id, err := sessionpkg.NewID()
		if err != nil {
			return fmt.Errorf("create session id: %w", err)
		}
		sess := &registry.Session{
			ID:           id,
			Name:         name,
			RepoID:       repoID,
			Index:        idx,
			BaseRef:      baseRef,
			Branch:       branch,
			WorktreePath: worktreePath,
			CreatedAt:    now,
			UpdatedAt:    now,
			Setup: registry.SetupState{
				State: registry.SetupCreating,
			},
			Agents: map[string]*registry.Agent{},
		}
		repoEntry.Sessions[id] = sess
		createdRepo = repoEntry
		createdSession = sess
		return nil
	}); err != nil {
		return err
	}

	if err := gitx.AddWorktree(ctx, repo.Root, createdSession.WorktreePath, branch, baseRef); err != nil {
		cleanupReservation(store, repoID, createdSession.ID)
		return fmt.Errorf("create git worktree: %w", err)
	}

	if err := sessionpkg.WriteEnvFile(createdSession.WorktreePath, createdRepo, createdSession); err != nil {
		cleanupCreatedWorktree(ctx, store, repo.Root, repoID, createdSession.ID, createdSession.WorktreePath)
		return fmt.Errorf("write %s: %w", sessionpkg.EnvFileName, err)
	}
	if err := gitx.AddExclude(ctx, createdSession.WorktreePath, sessionpkg.EnvFileName); err != nil {
		cleanupCreatedWorktree(ctx, store, repo.Root, repoID, createdSession.ID, createdSession.WorktreePath)
		return err
	}

	setupState := registry.SetupSkipped
	setupMessage := ""
	if !parsed.bools["no-setup"] && effective.OnCreate != "" {
		if err := hooks.Run(ctx, effective.OnCreate, createdRepo, createdSession, env.Stdout, env.Stderr); err != nil {
			setupState = registry.SetupFailed
			setupMessage = err.Error()
			_ = updateSetup(store, repoID, createdSession.ID, setupState, setupMessage)
			return err
		}
		setupState = registry.SetupOK
	}
	if err := updateSetup(store, repoID, createdSession.ID, setupState, setupMessage); err != nil {
		return err
	}

	if err := editor.Launch(effective.Editors, editorName, createdSession.WorktreePath, env.Stderr); err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "created session %s at %s\n", createdSession.Name, createdSession.WorktreePath)
	return nil
}

func updateSetup(store registry.Store, repoID, sessionID, state, message string) error {
	return store.Update(func(reg *registry.Registry) error {
		repo := reg.Repos[repoID]
		if repo == nil || repo.Sessions[sessionID] == nil {
			return fmt.Errorf("session %s disappeared from registry", sessionID)
		}
		repo.Sessions[sessionID].Setup = registry.SetupState{State: state, Message: message}
		repo.Sessions[sessionID].UpdatedAt = time.Now().UTC()
		return nil
	})
}

func cleanupReservation(store registry.Store, repoID, sessionID string) {
	_ = store.Update(func(reg *registry.Registry) error {
		removeSession(reg, repoID, sessionID)
		return nil
	})
}

func cleanupCreatedWorktree(ctx context.Context, store registry.Store, repoRoot, repoID, sessionID, worktreePath string) {
	_ = os.Remove(filepath.Join(worktreePath, sessionpkg.EnvFileName))
	if dirty, _, err := gitx.Dirty(ctx, worktreePath); err == nil && !dirty {
		_ = gitx.RemoveWorktree(ctx, repoRoot, worktreePath, false)
		cleanupReservation(store, repoID, sessionID)
	}
}

func renderWorktreePath(template, repoRoot, repoName, name, branch string, index int) string {
	rendered := template
	replacements := map[string]string{
		"{repo}":   repoName,
		"{name}":   name,
		"{branch}": branch,
		"{index}":  fmt.Sprintf("%d", index),
	}
	for key, value := range replacements {
		rendered = strings.ReplaceAll(rendered, key, value)
	}
	if filepath.IsAbs(rendered) {
		return filepath.Clean(rendered)
	}
	return filepath.Clean(filepath.Join(repoRoot, rendered))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
