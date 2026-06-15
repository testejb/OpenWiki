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

func TestIndexCheckJSON(t *testing.T) {
	dir := t.TempDir()
	tomlPath := setupTestWiki(t, dir)
	writeIndexTestConfig(t, tomlPath)

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--config", tomlPath, "index", "check", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got error: %#v", resp.Error)
	}
}

func TestIndexRebuildJSON(t *testing.T) {
	dir := t.TempDir()
	tomlPath := setupTestWiki(t, dir)
	writeIndexTestConfig(t, tomlPath)
	wikiRoot := filepath.Join(dir, "test-wiki")

	content := `---
title: 页面C
summary: 页面C摘要
tags: [test]
scope_level: repo
scope_code: openwiki
updated: 2026-06-15
---

# 页面C
`
	pagePath := filepath.Join(wikiRoot, "wiki", "pages", "page-c.md")
	if err := os.WriteFile(pagePath, []byte(content), 0644); err != nil {
		t.Fatalf("write page failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--config", tomlPath, "index", "rebuild", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got error: %#v", resp.Error)
	}

	tags, err := os.ReadFile(filepath.Join(wikiRoot, "wiki", "indexes", "tags.md"))
	if err != nil {
		t.Fatalf("read tags index failed: %v", err)
	}
	if !bytes.Contains(tags, []byte("[[page-c]]")) {
		t.Fatalf("expected rebuilt tags index to contain page-c, got:\n%s", string(tags))
	}
}

func writeIndexTestConfig(t *testing.T, tomlPath string) {
	t.Helper()

	content := `wiki_root = "./test-wiki"

[wiki]
primary_language = "zh"
secondary_language = "en"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("write index test config failed: %v", err)
	}
}
