package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/openwiki/internal/cli"
	"github.com/bytedance/openwiki/internal/output"
)

func createRequiredWikiLayout(t *testing.T, dir string) {
	t.Helper()
	for _, rel := range []string{
		"raw",
		"wiki/pages",
		"wiki/indexes",
		"entities",
		"concepts",
	} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0755); err != nil {
			t.Fatalf("failed to create %s: %v", rel, err)
		}
	}
	for _, rel := range []string{
		"wiki/index.md",
		"wiki/log.md",
	} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("# test\n"), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", rel, err)
		}
	}
}

func createConfigTOML(t *testing.T, dir string) string {
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

func TestConfigShow(t *testing.T) {
	dir := t.TempDir()
	tomlPath := createConfigTOML(t, dir)

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{"--config", tomlPath, "config", "show", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true, got error: %v", resp.Error)
	}
}

func TestConfigGet(t *testing.T) {
	dir := t.TempDir()
	tomlPath := createConfigTOML(t, dir)

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{"--config", tomlPath, "config", "get", "wiki_root", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true, got error: %v", resp.Error)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if data["value"] != "/Users/me/wiki" {
		t.Errorf("expected value=/Users/me/wiki, got %v", data["value"])
	}
}

func TestConfigSet(t *testing.T) {
	dir := t.TempDir()
	tomlPath := createConfigTOML(t, dir)

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{"--config", tomlPath, "config", "set", "wiki.primary_language", "en", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true, got error: %v", resp.Error)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if data["old_value"] != "zh" {
		t.Errorf("expected old_value=zh, got %v", data["old_value"])
	}
	if data["new_value"] != "en" {
		t.Errorf("expected new_value=en, got %v", data["new_value"])
	}

	content, _ := os.ReadFile(tomlPath)
	if !strings.Contains(string(content), "en") {
		t.Error("expected file to contain updated value 'en'")
	}
}

func TestConfigValidate(t *testing.T) {
	dir := t.TempDir()
	createRequiredWikiLayout(t, dir)
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "` + dir + `"

[wiki]
primary_language = "zh"
secondary_language = "en"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test toml: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{"--config", tomlPath, "config", "validate", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true, got error: %v", resp.Error)
	}
}

func TestConfigValidateInvalid(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = ""

[wiki]
primary_language = "fr"
secondary_language = "en"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test toml: %v", err)
	}

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{"--config", tomlPath, "config", "validate", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Fatal("expected success=false for invalid config")
	}
}

func TestConfigPath(t *testing.T) {
	dir := t.TempDir()
	tomlPath := createConfigTOML(t, dir)

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{"--config", tomlPath, "config", "path", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true, got error: %v", resp.Error)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if data["source"] != "explicit" {
		t.Errorf("expected source=explicit, got %v", data["source"])
	}
}
