package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/novr/bitrise-cli/internal/api"
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

	_, err := resolveAppSlugDetailed(context.Background(), cmd, nil, true)
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

	res, err := resolveAppSlugDetailed(context.Background(), cmd, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Slug != "local-app" {
		t.Fatalf("slug = %q, want local-app", res.Slug)
	}
	if res.LocalPath == "" {
		t.Fatal("expected local config path")
	}
}

func TestDoctorSlugMismatchIsWarningNotIssue(t *testing.T) {
	root := initTestGitRepo(t)
	runGitInDir(t, root, "remote", "add", "origin", "https://github.com/owner/repo.git")
	if err := os.WriteFile(filepath.Join(root, config.LocalConfigFileName), []byte("app: local-app\n"), 0644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/apps/local-app/builds"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[],"paging":{}}`))
		case strings.HasPrefix(r.URL.Path, "/apps"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{
					"slug":      "git-app",
					"title":     "Git App",
					"repo_url":  "https://github.com/owner/repo.git",
				}},
				"paging": map[string]string{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	t.Setenv("BITRISE_API_TOKEN", "test-token")

	client := api.NewClientWithBaseURL("test-token", srv.URL)
	cmd := &cobra.Command{}
	cmd.Flags().String("app", "", "")

	res, err := resolveAppSlugDetailed(context.Background(), cmd, client, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.GitSlug != "git-app" {
		t.Fatalf("git slug = %q, want git-app", res.GitSlug)
	}

	var buf bytes.Buffer
	doctorSlugWarnings(res, &buf)
	out := buf.String()
	if !strings.Contains(out, "differs from git-detected app") {
		t.Fatalf("expected mismatch warning, got: %q", out)
	}
}

func TestSamePath(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, ".br.yml")
	if err := os.WriteFile(a, []byte("app: x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !samePath(a, a) {
		t.Fatal("same path should match")
	}
}
