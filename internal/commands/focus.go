package commands

import (
	"context"
	"fmt"
	"sort"

	"sessions/internal/attention"
	"sessions/internal/config"
	focuspkg "sessions/internal/focus"
	"sessions/internal/registry"
)

func Focus(ctx context.Context, env Env, args []string) error {
	parsed, err := parseArgs(args,
		map[string]bool{"agent": true},
		map[string]bool{"next": true, "all": true},
	)
	if err != nil {
		return err
	}
	if parsed.bools["all"] && !parsed.bools["next"] {
		return fmt.Errorf("--all can only be used with --next")
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
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

	if parsed.bools["next"] {
		if len(parsed.pos) != 0 || parsed.strings["agent"] != "" {
			return fmt.Errorf("usage: sessions focus --next [--all]")
		}
		repos := scopedRepos(ctx, env.Cwd, reg)
		if parsed.bools["all"] {
			repos = allRepos(reg)
		}
		candidate, ok := focuspkg.Next(repos, attention.New(globalCfg.Attention.States))
		if !ok {
			fmt.Fprintln(env.Stdout, "No sessions need attention.")
			return nil
		}
		effective, err := effectiveForRepo(globalCfg, candidate.Repo)
		if err != nil {
			return err
		}
		return focuspkg.Route(ctx, effective, candidate.Repo, candidate.Session, candidate.Agent, env.Stdout, env.Stderr)
	}

	if len(parsed.pos) != 1 {
		return fmt.Errorf("usage: sessions focus <session> [--agent <agent>]")
	}
	currentID, _, _ := currentRepoID(ctx, env.Cwd)
	repo, sess, err := resolveSessionByName(reg, currentID, parsed.pos[0])
	if err != nil {
		return err
	}
	var agent *registry.Agent
	if agentName, ok := parsed.strings["agent"]; ok {
		if err := validateAgentName(agentName); err != nil {
			return err
		}
		if sess.Agents == nil || sess.Agents[agentName] == nil {
			return fmt.Errorf("agent %q not found in session %q", agentName, sess.Name)
		}
		agent = sess.Agents[agentName]
	}
	effective, err := effectiveForRepo(globalCfg, repo)
	if err != nil {
		return err
	}
	return focuspkg.Route(ctx, effective, repo, sess, agent, env.Stdout, env.Stderr)
}

func allRepos(reg *registry.Registry) []*registry.Repo {
	repos := make([]*registry.Repo, 0, len(reg.Repos))
	for _, repo := range reg.Repos {
		repos = append(repos, repo)
	}
	sort.Slice(repos, func(i, j int) bool {
		if repos[i].Name != repos[j].Name {
			return repos[i].Name < repos[j].Name
		}
		return repos[i].ID < repos[j].ID
	})
	return repos
}

func effectiveForRepo(globalCfg config.Config, repo *registry.Repo) (config.Effective, error) {
	repoCfg := config.RepoConfig{}
	if repo.MainWorktree != "" {
		loaded, err := config.LoadRepo(repo.MainWorktree)
		if err != nil {
			return config.Effective{}, err
		}
		repoCfg = loaded
	}
	return config.Merge(globalCfg, repoCfg, repo.Name), nil
}
