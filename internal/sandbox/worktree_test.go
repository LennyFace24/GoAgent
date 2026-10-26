package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("base-content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	return repo
}

func TestLocalWorktreeCreateExecuteAndDestroy(t *testing.T) {
	repo := initGitRepo(t)
	worktree, err := CreateLocalWorktree(repo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worktree.Destroy() })

	output, err := worktree.Execute(context.Background(), "cat README.md", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "base-content") {
		t.Fatalf("worktree did not contain repository content: %q", output)
	}

	if err := os.WriteFile(filepath.Join(worktree.WorkDir, "sandbox.txt"), []byte("local-change\n"), 0644); err != nil {
		t.Fatal(err)
	}
	output, err = worktree.Execute(context.Background(), "cat sandbox.txt", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "local-change") {
		t.Fatalf("worktree command did not run in worktree: %q", output)
	}

	if err := worktree.Destroy(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree.Path); !os.IsNotExist(err) {
		t.Fatalf("worktree path still exists: %v", err)
	}
	if got := runGit(t, repo, "show-ref", "--verify", "refs/heads/"+worktree.Branch); got == "" {
		t.Fatal("branch was not retained")
	}
}

func TestCreateLocalWorktreeRejectsNonGitDirectory(t *testing.T) {
	if _, err := CreateLocalWorktree(t.TempDir()); err == nil {
		t.Fatal("expected non-git workspace to fail")
	}
}
