package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"sessions/internal/output"
	"sessions/internal/registry"
	sessionpkg "sessions/internal/session"
)

func List(ctx context.Context, env Env, args []string) error {
	parsed, err := parseArgs(args, nil, map[string]bool{"json": true})
	if err != nil {
		return err
	}
	if len(parsed.pos) != 0 {
		return fmt.Errorf("usage: sessions ls [--json]")
	}
	store, err := registryStore()
	if err != nil {
		return err
	}
	reg, err := store.Read()
	if err != nil {
		return err
	}
	repos := scopedRepos(ctx, env.Cwd, reg)
	if parsed.bools["json"] {
		scoped := registry.Empty()
		for _, repo := range repos {
			scoped.Repos[repo.ID] = repo
		}
		data, err := json.MarshalIndent(scoped, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(env.Stdout, string(data))
		return nil
	}

	var rows [][]string
	now := time.Now()
	for _, repo := range repos {
		for _, sess := range sessionpkg.SortedSessions(repo) {
			rows = append(rows, []string{
				repo.Name,
				sess.Name,
				strconv.Itoa(sess.Index),
				sess.Branch,
				agentSummary(sess),
				output.RelativeTime(sess.UpdatedAt, now),
			})
		}
	}
	if len(rows) == 0 {
		fmt.Fprintln(env.Stdout, "No sessions.")
		return nil
	}
	output.Table(env.Stdout, []string{"repo", "name", "idx", "branch", "agents", "updated"}, rows)
	return nil
}

func scopedRepos(ctx context.Context, cwd string, reg *registry.Registry) []*registry.Repo {
	if repoID, _, ok := currentRepoID(ctx, cwd); ok {
		if repo := reg.Repos[repoID]; repo != nil {
			return []*registry.Repo{repo}
		}
	}
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

func agentSummary(sess *registry.Session) string {
	if len(sess.Agents) == 0 {
		return "-"
	}
	names := make([]string, 0, len(sess.Agents))
	for name := range sess.Agents {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		state := sess.Agents[name].State
		if state == "" {
			state = "attached"
		}
		parts = append(parts, name+":"+state)
	}
	return strings.Join(parts, " ")
}
