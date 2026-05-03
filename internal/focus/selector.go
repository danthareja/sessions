package focus

import (
	"sort"

	"sessions/internal/attention"
	"sessions/internal/registry"
	sessionpkg "sessions/internal/session"
)

type Candidate struct {
	Repo    *registry.Repo
	Session *registry.Session
	Agent   *registry.Agent
}

func Next(repos []*registry.Repo, policy attention.Policy) (Candidate, bool) {
	var candidates []Candidate
	for _, repo := range repos {
		for _, sess := range sessionpkg.SortedSessions(repo) {
			for _, agent := range sess.Agents {
				if agent == nil || !policy.Attention(agent.State) {
					continue
				}
				candidates = append(candidates, Candidate{Repo: repo, Session: sess, Agent: agent})
			}
		}
	}
	if len(candidates) == 0 {
		return Candidate{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		leftPriority := registry.AgentPriority(left.Agent.State)
		rightPriority := registry.AgentPriority(right.Agent.State)
		if leftPriority != rightPriority {
			return leftPriority > rightPriority
		}
		if !left.Agent.UpdatedAt.Equal(right.Agent.UpdatedAt) {
			return left.Agent.UpdatedAt.Before(right.Agent.UpdatedAt)
		}
		if left.Repo.Name != right.Repo.Name {
			return left.Repo.Name < right.Repo.Name
		}
		if left.Session.Name != right.Session.Name {
			return left.Session.Name < right.Session.Name
		}
		return left.Agent.Name < right.Agent.Name
	})
	return candidates[0], true
}
