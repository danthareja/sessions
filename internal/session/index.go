package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"

	"sessions/internal/registry"
)

const NamePattern = `^[a-z0-9][a-z0-9-]{0,62}$`

var nameRE = regexp.MustCompile(NamePattern)

func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid session name %q; must match %s", name, NamePattern)
	}
	return nil
}

func AllocateIndex(repo *registry.Repo) int {
	used := map[int]bool{}
	for _, s := range repo.Sessions {
		if s.Index > 0 {
			used[s.Index] = true
		}
	}
	for i := 1; ; i++ {
		if !used[i] {
			return i
		}
	}
}

func NewID() (string, error) {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func SortedSessions(repo *registry.Repo) []*registry.Session {
	sessions := make([]*registry.Session, 0, len(repo.Sessions))
	for _, s := range repo.Sessions {
		sessions = append(sessions, s)
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Index != sessions[j].Index {
			return sessions[i].Index < sessions[j].Index
		}
		return sessions[i].Name < sessions[j].Name
	})
	return sessions
}
