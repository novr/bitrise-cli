package cmd

import (
	"context"
	"strings"
	"testing"
)

func TestResolveBranchFilterPassthrough(t *testing.T) {
	got, err := resolveBranchFilter(context.Background(), "main")
	if err != nil {
		t.Fatal(err)
	}
	if got != "main" {
		t.Fatalf("got %q, want main", got)
	}
}

func TestResolveBranchFilterCurrent(t *testing.T) {
	root := initTestGitRepo(t)
	runGitInDir(t, root, "config", "commit.gpgsign", "false")
	runGitInDir(t, root, "commit", "--allow-empty", "-m", "init")
	runGitInDir(t, root, "checkout", "-b", "feature/foo")
	t.Chdir(root)

	got, err := resolveBranchFilter(context.Background(), branchCurrent)
	if err != nil {
		t.Fatal(err)
	}
	if got != "feature/foo" {
		t.Fatalf("got %q, want feature/foo", got)
	}
}

func TestResolveBranchFilterDetachedHEAD(t *testing.T) {
	root := initTestGitRepo(t)
	runGitInDir(t, root, "config", "commit.gpgsign", "false")
	runGitInDir(t, root, "commit", "--allow-empty", "-m", "init")
	runGitInDir(t, root, "checkout", "--detach")
	t.Chdir(root)

	_, err := resolveBranchFilter(context.Background(), branchCurrent)
	if err == nil {
		t.Fatal("expected error for detached HEAD")
	}
	if !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveBranchFilterNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	_, err := resolveBranchFilter(context.Background(), branchCurrent)
	if err == nil {
		t.Fatal("expected error outside git repo")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("unexpected error: %v", err)
	}
}
