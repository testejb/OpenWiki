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

func TestInitCreatesDirectoryStructure(t *testing.T) {
	dir := t.TempDir()
	wikiRoot := filepath.Join(dir, "test-wiki")

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{
		"init", wikiRoot,
		"--non-interactive", "--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
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

	expectedDirs := []string{
		"wiki/pages",
		"wiki/indexes",
		"raw",
		"concepts",
		"entities",
	}
	for _, d := range expectedDirs {
		p := filepath.Join(wikiRoot, d)
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			t.Errorf("expected directory %s to exist", p)
		}
	}

	expectedFiles := []string{
		"openwiki.toml",
		"wiki/index.md",
		"wiki/log.md",
		"wiki/indexes/scopes.md",
		"wiki/indexes/query-usage.jsonl",
	}
	for _, f := range expectedFiles {
		p := filepath.Join(wikiRoot, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", p)
		}
	}

	tomlContent, err := os.ReadFile(filepath.Join(wikiRoot, "openwiki.toml"))
	if err != nil {
		t.Fatalf("failed to read openwiki.toml: %v", err)
	}
	if !strings.Contains(string(tomlContent), "wiki_root") {
		t.Error("expected openwiki.toml to contain wiki_root")
	}

	created, ok := data["created"].([]interface{})
	if !ok {
		t.Fatalf("expected created to be a list, got %T", data["created"])
	}
	for _, expected := range []string{
		wikiRoot + "/wiki/indexes/",
		wikiRoot + "/wiki/indexes/scopes.md",
		wikiRoot + "/wiki/indexes/query-usage.jsonl",
		wikiRoot + "/entities/",
	} {
		if !containsStringValue(created, expected) {
			t.Errorf("expected created list to contain %s, got %v", expected, created)
		}
	}
}

func containsStringValue(values []interface{}, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func TestInitDefaultWikiRoot(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{
		"init",
		"--non-interactive", "--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
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
	if data["wiki_root"] != "./openwiki/" {
		t.Errorf("expected wiki_root='./openwiki/', got %v", data["wiki_root"])
	}

	expectedDirs := []string{
		"openwiki/wiki/pages",
		"openwiki/raw",
		"openwiki/concepts",
		"openwiki/entities",
	}
	for _, d := range expectedDirs {
		p := filepath.Join(dir, d)
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			t.Errorf("expected directory %s to exist", p)
		}
	}

	expectedFiles := []string{
		"openwiki/openwiki.toml",
		"openwiki/wiki/index.md",
		"openwiki/wiki/log.md",
	}
	for _, f := range expectedFiles {
		p := filepath.Join(dir, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", p)
		}
	}
}

func TestInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	wikiRoot := filepath.Join(dir, "test-wiki")

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{
		"init", wikiRoot,
		"--non-interactive", "--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	err = cli.RunWithIO([]string{
		"init", wikiRoot,
		"--non-interactive", "--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Fatal("expected success=false for already existing wiki")
	}
	if resp.Error == nil {
		t.Fatal("expected error to be non-nil")
	}
	if resp.Error.Code != "WIKI_ALREADY_EXISTS" {
		t.Errorf("expected code=WIKI_ALREADY_EXISTS, got %s", resp.Error.Code)
	}
}

func TestInitForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	wikiRoot := filepath.Join(dir, "test-wiki")

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{
		"init", wikiRoot,
		"--non-interactive", "--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	err = cli.RunWithIO([]string{
		"init", wikiRoot,
		"--force", "--non-interactive", "--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("force init failed: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true for force init, got error: %v", resp.Error)
	}
}

func TestInitDefaultWikiRootAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{
		"init",
		"--non-interactive", "--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	err = cli.RunWithIO([]string{
		"init",
		"--non-interactive", "--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Fatal("expected success=false for already existing wiki")
	}
	if resp.Error == nil {
		t.Fatal("expected error to be non-nil")
	}
	if resp.Error.Code != "WIKI_ALREADY_EXISTS" {
		t.Errorf("expected code=WIKI_ALREADY_EXISTS, got %s", resp.Error.Code)
	}
}

func TestInitDefaultWikiRootForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer os.Chdir(origDir)

	var stdout, stderr bytes.Buffer

	err := cli.RunWithIO([]string{
		"init",
		"--non-interactive", "--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()

	err = cli.RunWithIO([]string{
		"init",
		"--force", "--non-interactive", "--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("force init failed: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Fatalf("expected success=true for force init, got error: %v", resp.Error)
	}
}
