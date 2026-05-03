package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"sessions/internal/config"
	"sessions/internal/editor"
	gitx "sessions/internal/git"
	"sessions/internal/hooks"
	"sessions/internal/paths"
	"sessions/internal/registry"
	sessionpkg "sessions/internal/session"
	"sessions/internal/trust"
)

func Remove(ctx context.Context, env Env, args []string) error {
	parsed, err := parseArgs(args, nil, map[string]bool{"force": true})
	if err != nil {
		return err
	}
	if len(parsed.pos) > 1 {
		return fmt.Errorf("usage: sessions remove [<name>] [--force]")
	}
	force := parsed.bools["force"]

	store, err := registryStore()
	if err != nil {
		return err
	}
	reg, err := store.Read()
	if err != nil {
		return err
	}
	currentID, currentRoot, _ := currentRepoID(ctx, env.Cwd)
	var repo *registry.Repo
	var sess *registry.Session
	if len(parsed.pos) == 1 {
		var err error
		repo, sess, err = resolveSessionByName(reg, currentID, parsed.pos[0])
		if err != nil {
			return err
		}
	} else if r, s, ok := resolveSessionByWorktree(reg, currentRoot); ok {
		repo, sess = r, s
	} else if r, s, ok := resolveSessionByID(reg, env.getenv("SESSION_ID")); ok {
		repo, sess = r, s
	} else {
		return fmt.Errorf("could not resolve session from cwd; pass a session name")
	}

	exists := pathExists(sess.WorktreePath)
	if !exists {
		if !force {
			return fmt.Errorf("worktree path %s is missing; use --force to remove stale registry state", sess.WorktreePath)
		}
		return finishRemove(ctx, env, store, repo, sess)
	}

	repoCfg, err := config.LoadRepo(repo.MainWorktree)
	if err != nil {
		return err
	}
	if dirty, lines, err := gitx.Dirty(ctx, sess.WorktreePath); err != nil {
		return err
	} else if dirty && !force {
		return fmt.Errorf("worktree has uncommitted changes; use --force to remove it (%d dirty path(s))", len(lines))
	} else if dirty {
		fmt.Fprintf(env.Stderr, "warning: removing dirty worktree with %d dirty path(s)\n", len(lines))
	}

	for _, warning := range gitx.UpstreamWarnings(ctx, sess.WorktreePath) {
		fmt.Fprintf(env.Stderr, "warning: %s\n", warning)
	}

	if repoCfg.OnRemove != "" {
		trustPath, err := paths.TrustedPath()
		if err != nil {
			return err
		}
		if err := trust.NewStore(trustPath).Require(repo.ID, repo.CommonDir, repoCfg.Path, repoCfg.OnCreate, repoCfg.OnRemove, env.Stdin, env.Stderr); err != nil {
			return err
		}
		if err := hooks.Run(ctx, repoCfg.OnRemove, repo, sess, env.Stdout, env.Stderr); err != nil {
			if !force {
				return err
			}
			fmt.Fprintf(env.Stderr, "warning: remove hook failed but --force was set: %v\n", err)
		}
	}

	effective := config.Merge(config.Default(), repoCfg, repo.Name)
	if globalCfg, err := config.LoadGlobal(); err == nil {
		effective = config.Merge(globalCfg, repoCfg, repo.Name)
	} else {
		fmt.Fprintf(env.Stderr, "warning: could not load global config for editor close: %v\n", err)
	}
	_, err = editor.Close(effective.Editors, effective.Editor, sess.WorktreePath, env.Stderr)
	if err != nil {
		fmt.Fprintf(env.Stderr, "warning: editor close failed: %v\n", err)
	}

	_ = os.Remove(filepath.Join(sess.WorktreePath, sessionpkg.EnvFileName))
	if err := gitx.RemoveWorktree(ctx, repo.MainWorktree, sess.WorktreePath, force); err != nil {
		if force {
			if removeErr := os.RemoveAll(sess.WorktreePath); removeErr == nil {
				return finishRemove(ctx, env, store, repo, sess)
			}
		}
		return fmt.Errorf("remove git worktree: %w", err)
	}
	return finishRemove(ctx, env, store, repo, sess)
}

func finishRemove(ctx context.Context, env Env, store registry.Store, repo *registry.Repo, sess *registry.Session) error {
	deleteSessionBranch(ctx, env, repo, sess)
	if err := removeRegistryEntry(store, repo.ID, sess.ID); err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "removed session %s\n", sess.Name)
	return nil
}

func deleteSessionBranch(ctx context.Context, env Env, repo *registry.Repo, sess *registry.Session) {
	if err := gitx.DeleteBranch(ctx, repo.MainWorktree, sess.Branch); err != nil {
		fmt.Fprintf(env.Stderr, "warning: branch %s was not deleted: %v\n", sess.Branch, err)
	}
}

func removeRegistryEntry(store registry.Store, repoID, sessionID string) error {
	return store.Update(func(reg *registry.Registry) error {
		removeSession(reg, repoID, sessionID)
		return nil
	})
}
