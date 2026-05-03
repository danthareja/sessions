package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sessions/internal/registry"
)

const EnvFileName = ".env.sessions"

func WriteEnvFile(path string, repo *registry.Repo, sess *registry.Session) error {
	content := EnvFile(repo, sess)
	return os.WriteFile(filepath.Join(path, EnvFileName), []byte(content), 0o600)
}

func EnvFile(repo *registry.Repo, sess *registry.Session) string {
	pairs := [][2]string{
		{"SESSION_ID", sess.ID},
		{"SESSION_NAME", sess.Name},
		{"SESSION_INDEX", fmt.Sprintf("%d", sess.Index)},
		{"SESSION_REPO_NAME", repo.Name},
		{"SESSION_REPO_ROOT", repo.MainWorktree},
		{"SESSION_WORKTREE", sess.WorktreePath},
		{"SESSION_BRANCH", sess.Branch},
		{"SESSION_BASE_REF", sess.BaseRef},
	}
	var b strings.Builder
	for _, pair := range pairs {
		fmt.Fprintf(&b, "%s=%s\n", pair[0], dotenvQuote(pair[1]))
	}
	return b.String()
}

func EnvMap(repo *registry.Repo, sess *registry.Session) map[string]string {
	return map[string]string{
		"SESSION_ID":        sess.ID,
		"SESSION_NAME":      sess.Name,
		"SESSION_INDEX":     fmt.Sprintf("%d", sess.Index),
		"SESSION_REPO_NAME": repo.Name,
		"SESSION_REPO_ROOT": repo.MainWorktree,
		"SESSION_WORKTREE":  sess.WorktreePath,
		"SESSION_BRANCH":    sess.Branch,
		"SESSION_BASE_REF":  sess.BaseRef,
	}
}

func dotenvQuote(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}
