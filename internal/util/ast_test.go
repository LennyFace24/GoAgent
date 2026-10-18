package util

import "testing"

func TestAnalyzeBashCommandAllowsExplicitReadOnlyQueries(t *testing.T) {
	for _, command := range []string{
		"pwd",
		"ls -la",
		"cat README.md",
		"grep -n foo file.txt",
		"git status",
		"git log --oneline -n 5",
		"git branch --list",
		"git -C repo status",
		"python --version",
		"go version",
		"date +%s",
	} {
		if got := AnalyzeBashCommand(command); got != BashAccessRead {
			t.Errorf("AnalyzeBashCommand(%q) = %v, want read", command, got)
		}
	}
}

func TestAnalyzeBashCommandNeverAutoAllowsWritesOrAmbiguity(t *testing.T) {
	for _, command := range []string{
		"git branch -D feature",
		"git branch new-feature",
		"git tag -d v1",
		"git remote add origin https://example.com/repo.git",
		"git remote set-url origin https://example.com/other.git",
		"git -C repo branch -D feature",
		"make",
		"less README.md",
		"more README.md",
		"python -c 'print(1)'",
		"python scripts/build.py",
		"node scripts/build.js",
		"find . -delete",
		"find . -exec rm {} \\;",
		"sed -i 's/a/b/' file.txt",
		"awk '{print $1}' file.txt",
		"echo hi > out.txt",
		"cat file.txt | sh",
		"date -s '2020-01-01'",
		"date 010100002020",
		"sort -o out.txt in.txt",
		"hostname new-name",
		"sudo ls",
	} {
		if got := AnalyzeBashCommand(command); got == BashAccessRead {
			t.Errorf("AnalyzeBashCommand(%q) = read, want write or unknown", command)
		}
	}
}

func TestAnalyzeBashCommandEmptyAndUnparseableAreUnknown(t *testing.T) {
	if got := AnalyzeBashCommand("   "); got != BashAccessUnknown {
		t.Fatalf("empty command = %v, want unknown", got)
	}
}
