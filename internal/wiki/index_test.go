package wiki_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/openwiki/internal/wiki"
)

func TestCheckIndexReportsMissingShard(t *testing.T) {
	fs := wiki.NewMemFS()
	root := "/test-wiki"
	if err := wiki.Init(fs, root, map[string]interface{}{"wiki_root": root}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := fs.Remove(filepath.Join(root, "wiki", "indexes", "tags.md")); err != nil {
		t.Fatalf("remove tags index failed: %v", err)
	}

	result, err := wiki.CheckIndex(fs, root)
	if err != nil {
		t.Fatalf("CheckIndex returned error: %v", err)
	}
	if result.Health != "warning" {
		t.Fatalf("expected health=warning, got %s", result.Health)
	}
	if len(result.MissingFiles) != 1 || result.MissingFiles[0] != "wiki/indexes/tags.md" {
		t.Fatalf("expected missing tags shard, got %#v", result.MissingFiles)
	}
}

func TestCheckIndexReportsUnindexedPage(t *testing.T) {
	fs := wiki.NewMemFS()
	root := "/test-wiki"
	if err := wiki.Init(fs, root, map[string]interface{}{"wiki_root": root}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	content := `---
title: 测试页面
summary: 测试摘要
tags: [test]
scope_level: repo
scope_code: openwiki
updated: 2026-06-15
---

# 测试页面
`
	if err := fs.WriteFile(filepath.Join(root, "wiki", "pages", "test-page.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write page failed: %v", err)
	}

	result, err := wiki.CheckIndex(fs, root)
	if err != nil {
		t.Fatalf("CheckIndex returned error: %v", err)
	}
	if result.Health != "warning" {
		t.Fatalf("expected health=warning, got %s", result.Health)
	}
	if len(result.UnindexedPages) != 1 || result.UnindexedPages[0] != "test-page" {
		t.Fatalf("expected unindexed test-page, got %#v", result.UnindexedPages)
	}
}

func TestRebuildIndexIndexesExistingPages(t *testing.T) {
	fs := wiki.NewMemFS()
	root := "/test-wiki"
	if err := wiki.Init(fs, root, map[string]interface{}{"wiki_root": root}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	content := `---
title: 测试页面
summary: 测试摘要
tags: [test, demo]
scope_level: repo
scope_code: openwiki
updated: 2026-06-15
---

# 测试页面
`
	if err := fs.WriteFile(filepath.Join(root, "wiki", "pages", "test-page.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write page failed: %v", err)
	}

	result, err := wiki.RebuildIndex(fs, root)
	if err != nil {
		t.Fatalf("RebuildIndex returned error: %v", err)
	}
	if result.PageCount != 1 {
		t.Fatalf("expected page count 1, got %d", result.PageCount)
	}

	tags, err := fs.ReadFile(filepath.Join(root, "wiki", "indexes", "tags.md"))
	if err != nil {
		t.Fatalf("read tags index failed: %v", err)
	}
	if !strings.Contains(string(tags), "[[test-page]]") {
		t.Fatalf("expected tags index to include test-page, got:\n%s", string(tags))
	}

	check, err := wiki.CheckIndex(fs, root)
	if err != nil {
		t.Fatalf("CheckIndex returned error: %v", err)
	}
	if check.Health != "ok" {
		t.Fatalf("expected health ok after rebuild, got %#v", check)
	}
}
