package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPathUnsetUsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("BR_CONFIG_DIR", "")

	got, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "br", "config.yml")
	if got != want {
		t.Fatalf("configPath() = %q, want %q", got, want)
	}
}

func TestConfigPathAbsoluteBRConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BR_CONFIG_DIR", dir)

	got, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "config.yml")
	if got != want {
		t.Fatalf("configPath() = %q, want %q", got, want)
	}
}

func TestConfigPathRelativeBRConfigDir(t *testing.T) {
	base := t.TempDir()
	base, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join("cfg", "work")
	t.Setenv("BR_CONFIG_DIR", rel)

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(base); err != nil {
		t.Fatal(err)
	}

	got, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, rel, "config.yml")
	if got != want {
		t.Fatalf("configPath() = %q, want %q", got, want)
	}
}

func TestConfigPathBRConfigDirWithoutHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BR_CONFIG_DIR", dir)
	t.Setenv("HOME", "")

	got, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "config.yml")
	if got != want {
		t.Fatalf("configPath() = %q, want %q", got, want)
	}
}

func TestPathMatchesConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BR_CONFIG_DIR", dir)

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	want, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	if path != want {
		t.Fatalf("Path() = %q, configPath() = %q", path, want)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BR_CONFIG_DIR", dir)

	cfg := &Config{Token: "test-token"}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != cfg.Token {
		t.Fatalf("Load().Token = %q, want %q", loaded.Token, cfg.Token)
	}
}

func TestSavePermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BR_CONFIG_DIR", dir)

	if err := Save(&Config{Token: "x"}); err != nil {
		t.Fatal(err)
	}

	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm()&0700 != 0700 {
		t.Fatalf("config dir owner mode = %o, want owner rwx", dirInfo.Mode().Perm()&0700)
	}
	if dirInfo.Mode().Perm()&0022 != 0 {
		t.Fatalf("config dir is group/world-writable: %o", dirInfo.Mode().Perm())
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm()&0600 != 0600 {
		t.Fatalf("config file owner mode = %o, want owner rw", fileInfo.Mode().Perm()&0600)
	}
	if fileInfo.Mode().Perm()&0022 != 0 {
		t.Fatalf("config file is group/world-writable: %o", fileInfo.Mode().Perm())
	}
}

func TestGetTokenEnvBeatsStoredConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BR_CONFIG_DIR", dir)
	if err := Save(&Config{Token: "stored-token"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BITRISE_API_TOKEN", "env-token")

	token, err := GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if token != "env-token" {
		t.Fatalf("GetToken() = %q, want env-token", token)
	}
}

func TestDeprecatedDefaultAppValueUsesBRConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BR_CONFIG_DIR", dir)

	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("default_app: legacy-slug\n"), 0600); err != nil {
		t.Fatal(err)
	}

	slug, ok, err := DeprecatedDefaultAppValue()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected deprecated default_app to be found")
	}
	if slug != "legacy-slug" {
		t.Fatalf("slug = %q, want legacy-slug", slug)
	}
}
