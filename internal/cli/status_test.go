package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/openwiki/internal/cli"
	"github.com/bytedance/openwiki/internal/output"
)

func setupTestWiki(t *testing.T, dir string) string {
	t.Helper()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	wikiRoot := "test-wiki"

	var stdout, stderr bytes.Buffer
	err = cli.RunWithIO([]string{
		"init", wikiRoot,
		"--non-interactive", "--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	wikiRootAbs := filepath.Join(dir, "test-wiki")
	tomlPath := filepath.Join(wikiRootAbs, "openwiki.toml")
	configContent := `wiki_root = "."

[wiki]
primary_language = "zh"
secondary_language = "en"
`
	if err := os.WriteFile(tomlPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	pageDir := filepath.Join(wikiRootAbs, "wiki", "pages")
	if err := os.MkdirAll(pageDir, 0755); err != nil {
		t.Fatalf("mkdir pages failed: %v", err)
	}

	pages := []struct {
		slug, content string
	}{
		{
			"page-a",
			"---\ntitle: 页面A\ntags: [test, demo]\nscope_level: industry\nscope_code: test\nupdated: 2026-06-01\n---\n\n这是页面A的内容\n",
		},
		{
			"page-b",
			"---\ntitle: 页面B\ntags: [guide]\nscope_level: repo\nscope_code: my-repo\nupdated: 2026-06-02\n---\n\n这是页面B的内容\n",
		},
	}
	for _, p := range pages {
		pagePath := filepath.Join(pageDir, p.slug+".md")
		if err := os.WriteFile(pagePath, []byte(p.content), 0644); err != nil {
			t.Fatalf("write page %s failed: %v", p.slug, err)
		}
	}

	return tomlPath
}

func TestStatusJSON(t *testing.T) {
	dir := t.TempDir()
	tomlPath := setupTestWiki(t, dir)

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{"--config", tomlPath, "status", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
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

	pages, ok := data["pages"].(map[string]interface{})
	if !ok {
		t.Fatal("expected pages to be a map")
	}

	total, ok := pages["total"].(float64)
	if !ok {
		t.Fatal("expected total to be a number")
	}
	if total != 2 {
		t.Errorf("expected total=2, got %v", total)
	}

	configData, ok := data["config"].(map[string]interface{})
	if !ok {
		t.Fatal("expected config to be a map")
	}
	if configData["source"] == nil || configData["source"] == "" {
		t.Error("expected config.source to be non-empty")
	}
}

func TestStatusVerbose(t *testing.T) {
	dir := t.TempDir()
	tomlPath := setupTestWiki(t, dir)

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{"--config", tomlPath, "status", "--verbose", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
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

	details, ok := data["details"].([]interface{})
	if !ok {
		t.Fatal("expected details to be an array")
	}
	if len(details) != 2 {
		t.Errorf("expected 2 page details, got %d", len(details))
	}
}

func TestStatusIncludesIndexHealth(t *testing.T) {
	dir := t.TempDir()
	tomlPath := setupTestWiki(t, dir)

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--config", tomlPath, "status", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
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
	index, ok := data["index"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data.index to be a map")
	}
	health, ok := index["health"].(string)
	if !ok || health == "" {
		t.Fatalf("expected data.index.health to be non-empty, got %#v", index["health"])
	}
}
