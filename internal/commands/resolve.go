package commands

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"sessions/internal/git"
	"sessions/internal/paths"
	"sessions/internal/registry"
)

func registryStore() (registry.Store, error) {
	path, err := paths.RegistryPath()
	if err != nil {
		return registry.Store{}, err
	}
	return registry.NewStore(path), nil
}

func currentRepoID(ctx context.Context, cwd string) (string, string, bool) {
	repo, err := git.Discover(ctx, cwd)
	if err != nil {
		return "", "", false
	}
	return registry.RepoID(repo.CommonDir), git.NormalizePath(repo.Root), true
}

func resolveSessionByName(reg *registry.Registry, currentRepoID, name string) (*registry.Repo, *registry.Session, error) {
	if name == "" {
		return nil, nil, errors.New("session name is required")
	}
	if repoName, sessionName, ok := strings.Cut(name, "/"); ok {
		for _, repo := range reg.Repos {
			if repo.Name == repoName || repo.ID == repoName {
				for _, sess := range repo.Sessions {
					if sess.Name == sessionName || sess.ID == sessionName {
						return repo, sess, nil
					}
				}
				return nil, nil, fmt.Errorf("session %q not found in repo %q", sessionName, repoName)
			}
		}
		return nil, nil, fmt.Errorf("repo %q not found", repoName)
	}

	if currentRepoID != "" {
		if repo := reg.Repos[currentRepoID]; repo != nil {
			for _, sess := range repo.Sessions {
				if sess.Name == name || sess.ID == name {
					return repo, sess, nil
				}
			}
		}
	}

	var foundRepo *registry.Repo
	var foundSession *registry.Session
	for _, repo := range reg.Repos {
		for _, sess := range repo.Sessions {
			if sess.Name == name || sess.ID == name {
				if foundSession != nil {
					return nil, nil, fmt.Errorf("session %q is ambiguous; use repo/name", name)
				}
				foundRepo = repo
				foundSession = sess
			}
		}
	}
	if foundSession == nil {
		return nil, nil, fmt.Errorf("session %q not found", name)
	}
	return foundRepo, foundSession, nil
}

func resolveSessionByWorktree(reg *registry.Registry, worktreeRoot string) (*registry.Repo, *registry.Session, bool) {
	if worktreeRoot == "" {
		return nil, nil, false
	}
	needle := git.NormalizePath(worktreeRoot)
	for _, repo := range reg.Repos {
		for _, sess := range repo.Sessions {
			if git.NormalizePath(sess.WorktreePath) == needle {
				return repo, sess, true
			}
		}
	}
	return nil, nil, false
}

func resolveSessionByID(reg *registry.Registry, id string) (*registry.Repo, *registry.Session, bool) {
	if id == "" {
		return nil, nil, false
	}
	for _, repo := range reg.Repos {
		for _, sess := range repo.Sessions {
			if sess.ID == id {
				return repo, sess, true
			}
		}
	}
	return nil, nil, false
}

func removeSession(reg *registry.Registry, repoID, sessionID string) {
	repo := reg.Repos[repoID]
	if repo == nil {
		return
	}
	delete(repo.Sessions, sessionID)
	if len(repo.Sessions) == 0 {
		delete(reg.Repos, repoID)
	}
}

func pathExists(path string) bool {
	_, err := filepath.EvalSymlinks(path)
	return err == nil
}
