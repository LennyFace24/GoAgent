package sandboxtool

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func gitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("sandbox-base\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	return repo
}

func TestSandboxLocalLifecycle(t *testing.T) {
	repo := gitRepo(t)
	tool, err := NewSandBoxTool("", repo)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	created, err := tool.InvokableRun(ctx, `{"operation":"create","mode":"local"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(created, "sandbox_id=wt_") {
		t.Fatalf("unexpected create output: %q", created)
	}
	id := strings.TrimSpace(strings.Split(strings.Split(created, "sandbox_id=")[1], "\n")[0])

	output, err := tool.InvokableRun(ctx, `{"operation":"execute","sandbox_id":"`+id+`","cmd":"cat README.md"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "sandbox-base") {
		t.Fatalf("unexpected execute output: %q", output)
	}

	destroyed, err := tool.InvokableRun(ctx, `{"operation":"destroy","sandbox_id":"`+id+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(destroyed, "分支") {
		t.Fatalf("unexpected destroy output: %q", destroyed)
	}
}
