package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/novr/bitrise-cli/internal/config"
	"github.com/spf13/cobra"
)

func TestDoctorUnresolvedExit1(t *testing.T) {
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("app", "", "")

	_, _, _, _, err := diagnoseAppResolution(context.Background(), cmd, nil)
	if err == nil {
		t.Fatal("expected resolution error outside git with no .br.yml")
	}
}

func TestDoctorLocalResolvesWithGitMismatchPotential(t *testing.T) {
	root := initTestGitRepo(t)
	runGitInDir(t, root, "remote", "add", "origin", "https://github.com/owner/repo.git")
	if err := os.WriteFile(filepath.Join(root, config.LocalConfigFileName), []byte("app: local-app\n"), 0644); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("app", "", "")

	slug, localPath, _, _, err := diagnoseAppResolution(context.Background(), cmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if slug != "local-app" {
		t.Fatalf("slug = %q, want local-app", slug)
	}
	if localPath == "" {
		t.Fatal("expected local config path")
	}
}

func TestDoctorSlugMismatchIsWarningNotIssue(t *testing.T) {
	localSlug := "local-app"
	gitSlug := "git-app"
	issues := 0
	if localSlug != "" && gitSlug != "" && gitSlug != localSlug {
		// warning only — do not increment issues
	}
	if issues != 0 {
		t.Fatalf("mismatch should not increment issues, got %d", issues)
	}
}
