package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/openwiki/internal/config"
)

func createTestTOML(t *testing.T, dir string) string {
	t.Helper()
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "/Users/me/wiki"

[wiki]
primary_language = "zh"
secondary_language = "en"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test toml: %v", err)
	}
	return tomlPath
}

func TestDiscoverExplicitPath(t *testing.T) {
	dir := t.TempDir()
	tomlPath := createTestTOML(t, dir)

	d := &config.DefaultDiscoverer{
		HomeDir: "/nonexistent",
		Getenv:  func(string) string { return "" },
		Getwd:   func() (string, error) { return "/", nil },
	}

	result, err := d.Discover(tomlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Path != tomlPath {
		t.Errorf("expected path=%s, got %s", tomlPath, result.Path)
	}
	if result.Source != "explicit" {
		t.Errorf("expected source=explicit, got %s", result.Source)
	}
}

func TestDiscoverEnvVar(t *testing.T) {
	dir := t.TempDir()
	tomlPath := createTestTOML(t, dir)

	d := &config.DefaultDiscoverer{
		HomeDir: "/nonexistent",
		Getenv: func(key string) string {
			if key == "OPENWIKI_CONFIG" {
				return tomlPath
			}
			return ""
		},
		Getwd: func() (string, error) { return "/", nil },
	}

	result, err := d.Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Path != tomlPath {
		t.Errorf("expected path=%s, got %s", tomlPath, result.Path)
	}
	if result.Source != "env" {
		t.Errorf("expected source=env, got %s", result.Source)
	}
}

func TestDiscoverGlobalConfig(t *testing.T) {
	homeDir := t.TempDir()
	openwikiDir := filepath.Join(homeDir, ".openwiki")
	if err := os.MkdirAll(openwikiDir, 0755); err != nil {
		t.Fatalf("failed to create .openwiki dir: %v", err)
	}
	tomlPath := createTestTOML(t, openwikiDir)

	d := &config.DefaultDiscoverer{
		HomeDir: homeDir,
		Getenv:  func(string) string { return "" },
		Getwd:   func() (string, error) { return "/", nil },
	}

	result, err := d.Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Path != tomlPath {
		t.Errorf("expected path=%s, got %s", tomlPath, result.Path)
	}
	if result.Source != "global" {
		t.Errorf("expected source=global, got %s", result.Source)
	}
}

func TestDiscoverLocalConfig(t *testing.T) {
	dir := t.TempDir()
	tomlPath := createTestTOML(t, dir)

	d := &config.DefaultDiscoverer{
		HomeDir: "/nonexistent",
		Getenv:  func(string) string { return "" },
		Getwd:   func() (string, error) { return dir, nil },
	}

	result, err := d.Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Path != tomlPath {
		t.Errorf("expected path=%s, got %s", tomlPath, result.Path)
	}
	if result.Source != "local" {
		t.Errorf("expected source=local, got %s", result.Source)
	}
}

func TestDiscoverLocalConfigParentDir(t *testing.T) {
	dir := t.TempDir()
	tomlPath := createTestTOML(t, dir)
	childDir := filepath.Join(dir, "sub", "deep", "path")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatalf("failed to create child dir: %v", err)
	}

	d := &config.DefaultDiscoverer{
		HomeDir: "/nonexistent",
		Getenv:  func(string) string { return "" },
		Getwd:   func() (string, error) { return childDir, nil },
	}

	result, err := d.Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Path != tomlPath {
		t.Errorf("expected path=%s, got %s", tomlPath, result.Path)
	}
	if result.Source != "local" {
		t.Errorf("expected source=local, got %s", result.Source)
	}
}

func TestDiscoverNotFound(t *testing.T) {
	d := &config.DefaultDiscoverer{
		HomeDir: "/nonexistent",
		Getenv:  func(string) string { return "" },
		Getwd:   func() (string, error) { return "/", nil },
	}

	_, err := d.Discover("")
	if err == nil {
		t.Fatal("expected error when no config found, got nil")
	}
}

func TestDiscoverLocalPriorityOverGlobal(t *testing.T) {
	// 同时创建 local 和 global 配置
	homeDir := t.TempDir()
	openwikiDir := filepath.Join(homeDir, ".openwiki")
	if err := os.MkdirAll(openwikiDir, 0755); err != nil {
		t.Fatalf("failed to create .openwiki dir: %v", err)
	}
	createTestTOML(t, openwikiDir) // global config

	localDir := t.TempDir()
	localTomlPath := createTestTOML(t, localDir) // local config

	d := &config.DefaultDiscoverer{
		HomeDir: homeDir,
		Getenv:  func(string) string { return "" },
		Getwd:   func() (string, error) { return localDir, nil },
	}

	result, err := d.Discover("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Path != localTomlPath {
		t.Errorf("expected local config path=%s, got %s", localTomlPath, result.Path)
	}
	if result.Source != "local" {
		t.Errorf("expected source=local, got %s", result.Source)
	}
}
