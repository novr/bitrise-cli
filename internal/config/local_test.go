package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindLocalConfigPrefersCwdOverAncestor(t *testing.T) {
	root := initGitRepo(t)
	sub := filepath.Join(root, "pkg-a")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, LocalConfigFileName), []byte("app: root-app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, LocalConfigFileName), []byte("app: pkg-app\n"), 0644); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}

	cfg, path, err := FindLocalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App != "pkg-app" {
		t.Fatalf("app = %q, want pkg-app", cfg.App)
	}
	if !strings.HasSuffix(path, filepath.Join("pkg-a", LocalConfigFileName)) {
		t.Fatalf("path = %q", path)
	}
}

func TestFindLocalConfigInheritsFromGitRoot(t *testing.T) {
	root := initGitRepo(t)
	sub := filepath.Join(root, "pkg-a")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, LocalConfigFileName), []byte("app: root-app\n"), 0644); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := FindLocalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App != "root-app" {
		t.Fatalf("app = %q, want root-app", cfg.App)
	}
}

func TestFindLocalConfigOutsideGitOnlyCwd(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Dir(dir)
	if err := os.WriteFile(filepath.Join(parent, LocalConfigFileName), []byte("app: parent\n"), 0644); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := FindLocalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config outside git, got %+v", cfg)
	}
}

func TestFindLocalConfigSkipsEmptyApp(t *testing.T) {
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, LocalConfigFileName), []byte("app:\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "pkg")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, LocalConfigFileName), []byte("app: child\n"), 0644); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := FindLocalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App != "child" {
		t.Fatalf("app = %q, want child", cfg.App)
	}
}

func TestFindLocalConfigSkipsWhitespaceOnlyApp(t *testing.T) {
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, LocalConfigFileName), []byte("app: \"   \"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "pkg")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, LocalConfigFileName), []byte("app: child\n"), 0644); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := FindLocalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App != "child" {
		t.Fatalf("app = %q, want child", cfg.App)
	}
}

func TestFindLocalConfigInvalidYAML(t *testing.T) {
	root := initGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, LocalConfigFileName), []byte("app: [\n"), 0644); err != nil {
		t.Fatal(err)
	}

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	_, _, err := FindLocalConfig()
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestWriteLocalConfig(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteLocalConfig(dir, "my-app")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadLocalConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App != "my-app" {
		t.Fatalf("app = %q", cfg.App)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}
