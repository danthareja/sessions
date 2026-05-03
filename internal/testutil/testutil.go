package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TempGitRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", root, "init", "-b", "main").Run(); err != nil {
		run(t, root, "git", "init")
		run(t, root, "git", "checkout", "-b", "main")
	}
	run(t, root, "git", "config", "user.email", "sessions@example.test")
	run(t, root, "git", "config", "user.name", "Sessions Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", "README.md")
	run(t, root, "git", "commit", "-m", "initial")
	return root
}

func Run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	run(t, dir, name, args...)
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}

func IsGitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}
