package cmd

import (
	"context"
	"os"
	"path/filepath"
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

func TestResolveBranchFilterCurrentTrimmed(t *testing.T) {
	root := initTestGitRepo(t)
	runGitInDir(t, root, "commit", "--allow-empty", "-m", "init")
	t.Chdir(root)

	got, err := resolveBranchFilter(context.Background(), "  @current  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "main" {
		t.Fatalf("got %q, want main", got)
	}
}

func TestResolveBranchFilterCurrentFromSubdir(t *testing.T) {
	root := initTestGitRepo(t)
	runGitInDir(t, root, "commit", "--allow-empty", "-m", "init")
	runGitInDir(t, root, "checkout", "-b", "feature/foo")
	sub := filepath.Join(root, "pkg")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

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

func TestCurrentGitBranchGitNotFound(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := currentGitBranch(context.Background())
	if err == nil {
		t.Fatal("expected error when git is not in PATH")
	}
	if !strings.Contains(err.Error(), "git not found in PATH") {
		t.Fatalf("unexpected error: %v", err)
	}
}
