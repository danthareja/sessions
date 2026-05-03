package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Repo struct {
	Root         string
	CommonDir    string
	MainWorktree string
}

func Discover(ctx context.Context, cwd string) (Repo, error) {
	root, err := output(ctx, cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repo{}, fmt.Errorf("not inside a git repo: %w", err)
	}
	root = absFrom(cwd, root)

	commonDir, err := output(ctx, root, "rev-parse", "--git-common-dir")
	if err != nil {
		return Repo{}, fmt.Errorf("resolve git common dir: %w", err)
	}
	commonDir = absFrom(root, commonDir)

	mainWorktree, err := firstWorktree(ctx, root)
	if err != nil {
		mainWorktree = root
	}

	return Repo{
		Root:         clean(root),
		CommonDir:    clean(commonDir),
		MainWorktree: clean(mainWorktree),
	}, nil
}

func DefaultBranch(ctx context.Context, repoRoot string) (string, error) {
	if ref, err := output(ctx, repoRoot, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			return ref, nil
		}
	}
	for _, branch := range []string{"main", "master", "trunk"} {
		if err := command(ctx, repoRoot, nil, nil, nil, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
			return branch, nil
		}
	}
	return "", errors.New("could not resolve a default branch; pass --base or set base_ref in .sessions.toml")
}

func ValidateBranchName(ctx context.Context, repoRoot, branch string) error {
	if strings.TrimSpace(branch) == "" {
		return errors.New("branch name is empty")
	}
	if err := command(ctx, repoRoot, nil, nil, nil, "check-ref-format", "--branch", branch); err != nil {
		return fmt.Errorf("invalid branch name %q: %w", branch, err)
	}
	return nil
}

func BranchExists(ctx context.Context, repoRoot, branch string) bool {
	if strings.TrimSpace(branch) == "" {
		return false
	}
	return command(ctx, repoRoot, nil, nil, nil, "show-ref", "--verify", "--quiet", "refs/heads/"+branch) == nil
}

func AddWorktree(ctx context.Context, repoRoot, worktreePath, branch, baseRef string) error {
	return command(ctx, repoRoot, nil, nil, nil, "worktree", "add", "-b", branch, worktreePath, baseRef)
}

func RemoveWorktree(ctx context.Context, repoRoot, worktreePath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, worktreePath)
	return command(ctx, repoRoot, nil, nil, nil, args...)
}

func DeleteBranch(ctx context.Context, repoRoot, branch string) error {
	if !BranchExists(ctx, repoRoot, branch) {
		return nil
	}
	return command(ctx, repoRoot, nil, nil, nil, "branch", "-d", branch)
}

func Dirty(ctx context.Context, worktreePath string) (bool, []string, error) {
	out, err := output(ctx, worktreePath, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return false, nil, fmt.Errorf("check dirty worktree: %w", err)
	}
	var dirty []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		path := porcelainPath(line)
		if path == ".env.sessions" {
			continue
		}
		dirty = append(dirty, line)
	}
	return len(dirty) > 0, dirty, nil
}

func AddExclude(ctx context.Context, worktreePath, pattern string) error {
	excludePath, err := output(ctx, worktreePath, "rev-parse", "--git-path", "info/exclude")
	if err == nil {
		excludePath = absFrom(worktreePath, excludePath)
		if err := appendExclude(excludePath, pattern); err == nil {
			return nil
		}
	}

	gitDir, err := output(ctx, worktreePath, "rev-parse", "--git-dir")
	if err != nil {
		return fmt.Errorf("resolve worktree git dir: %w", err)
	}
	gitDir = absFrom(worktreePath, gitDir)
	if err := appendExclude(filepath.Join(gitDir, "info", "exclude"), pattern); err == nil {
		return nil
	}

	commonDir, err := output(ctx, worktreePath, "rev-parse", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("resolve common git dir for exclude fallback: %w", err)
	}
	commonDir = absFrom(worktreePath, commonDir)
	if err := appendExclude(filepath.Join(commonDir, "info", "exclude"), pattern); err != nil {
		return fmt.Errorf("add %s to git exclude: %w", pattern, err)
	}
	return nil
}

func UpstreamWarnings(ctx context.Context, worktreePath string) []string {
	upstream, err := output(ctx, worktreePath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil || strings.TrimSpace(upstream) == "" {
		return []string{"branch has no upstream; commits may be unpushed"}
	}
	countText, err := output(ctx, worktreePath, "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		return []string{"could not determine whether branch has unpushed commits"}
	}
	count, err := strconv.Atoi(strings.TrimSpace(countText))
	if err != nil {
		return []string{"could not determine whether branch has unpushed commits"}
	}
	if count > 0 {
		return []string{fmt.Sprintf("branch has %d commit(s) not present on upstream %s", count, strings.TrimSpace(upstream))}
	}
	return nil
}

func NormalizePath(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	if evaluated, err := filepath.EvalSymlinks(path); err == nil {
		path = evaluated
	}
	return clean(path)
}

func Run(ctx context.Context, cwd string, stdin *bytes.Buffer, stdout, stderr *bytes.Buffer, args ...string) error {
	return command(ctx, cwd, stdin, stdout, stderr, args...)
}

func output(ctx context.Context, cwd string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	err := command(ctx, cwd, nil, &stdout, &stderr, args...)
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func command(ctx context.Context, cwd string, stdin *bytes.Buffer, stdout, stderr *bytes.Buffer, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cwd
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var localStdout bytes.Buffer
	if stdout != nil {
		cmd.Stdout = stdout
	} else {
		cmd.Stdout = &localStdout
	}
	var localStderr bytes.Buffer
	if stderr != nil {
		cmd.Stderr = stderr
	} else {
		cmd.Stderr = &localStderr
	}
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(localStderr.String())
		if stderr != nil {
			msg = strings.TrimSpace(stderr.String())
		}
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

func firstWorktree(ctx context.Context, repoRoot string) (string, error) {
	out, err := output(ctx, repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "worktree ")), nil
		}
	}
	return "", errors.New("git worktree list returned no worktree")
}

func appendExclude(path, pattern string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == pattern {
			return nil
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(pattern + "\n")
	return err
}

func porcelainPath(line string) string {
	if len(line) < 4 {
		return strings.TrimSpace(line)
	}
	path := strings.TrimSpace(line[3:])
	if _, after, ok := strings.Cut(path, " -> "); ok {
		path = after
	}
	return strings.Trim(path, `"`)
}

func absFrom(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

func clean(path string) string {
	return filepath.Clean(path)
}
