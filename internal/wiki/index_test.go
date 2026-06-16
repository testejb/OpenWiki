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
	want := []string{
		"test-page (missing wiki/indexes/recent.md)",
		"test-page (missing wiki/indexes/scopes.md)",
		"test-page (missing wiki/indexes/tags.md)",
	}
	if len(result.UnindexedPages) != len(want) {
		t.Fatalf("expected unindexed page coverage %#v, got %#v", want, result.UnindexedPages)
	}
	for _, item := range want {
		if !containsString(result.UnindexedPages, item) {
			t.Fatalf("expected unindexed page coverage %q, got %#v", item, result.UnindexedPages)
		}
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

func TestCheckIndexReportsPageMissingFromTagsShardEvenIfRecentIndexed(t *testing.T) {
	fs := wiki.NewMemFS()
	root := "/test-wiki"
	if err := wiki.Init(fs, root, map[string]interface{}{"wiki_root": root}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	content := `---
title: 测试页面
tags: [test]
updated: 2026-06-15
---

# 测试页面
`
	if err := fs.WriteFile(filepath.Join(root, "wiki", "pages", "test-page.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write page failed: %v", err)
	}
	if err := fs.WriteFile(filepath.Join(root, "wiki", "indexes", "recent.md"), []byte("# 最近更新\n\n- [[test-page]]\n"), 0644); err != nil {
		t.Fatalf("write recent index failed: %v", err)
	}

	result, err := wiki.CheckIndex(fs, root)
	if err != nil {
		t.Fatalf("CheckIndex returned error: %v", err)
	}
	want := "test-page (missing wiki/indexes/tags.md)"
	if !containsString(result.UnindexedPages, want) {
		t.Fatalf("expected missing tags shard coverage %q, got %#v", want, result.UnindexedPages)
	}
}

func TestRebuildIndexPreservesQueryUsageAndUsesItForHotIndex(t *testing.T) {
	fs := wiki.NewMemFS()
	root := "/test-wiki"
	if err := wiki.Init(fs, root, map[string]interface{}{"wiki_root": root}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	content := `---
title: 测试页面
tags: [test]
updated: 2026-06-15
---

# 测试页面
`
	if err := fs.WriteFile(filepath.Join(root, "wiki", "pages", "test-page.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write page failed: %v", err)
	}
	usage := "{\"time\":\"2026-06-15T10:00:00+08:00\",\"cited_pages\":[\"test-page\"],\"read_pages\":[\"other-page\"],\"intent_tags\":[\"test\"]}\n"
	if err := fs.WriteFile(filepath.Join(root, "wiki", "indexes", "query-usage.jsonl"), []byte(usage), 0644); err != nil {
		t.Fatalf("write query usage failed: %v", err)
	}

	if _, err := wiki.RebuildIndex(fs, root); err != nil {
		t.Fatalf("RebuildIndex returned error: %v", err)
	}
	preserved, err := fs.ReadFile(filepath.Join(root, "wiki", "indexes", "query-usage.jsonl"))
	if err != nil {
		t.Fatalf("read query usage failed: %v", err)
	}
	if string(preserved) != usage {
		t.Fatalf("expected query usage preserved, got %q", string(preserved))
	}
	hot, err := fs.ReadFile(filepath.Join(root, "wiki", "indexes", "hot.md"))
	if err != nil {
		t.Fatalf("read hot index failed: %v", err)
	}
	if !strings.Contains(string(hot), "[[test-page]]") {
		t.Fatalf("expected hot index to include cited page, got:\n%s", string(hot))
	}
}

func TestRebuildIndexBacksUpAllIndexesWithUniquePaths(t *testing.T) {
	fs := wiki.NewMemFS()
	root := "/test-wiki"
	if err := wiki.Init(fs, root, map[string]interface{}{"wiki_root": root}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	first, err := wiki.RebuildIndex(fs, root)
	if err != nil {
		t.Fatalf("first RebuildIndex returned error: %v", err)
	}
	second, err := wiki.RebuildIndex(fs, root)
	if err != nil {
		t.Fatalf("second RebuildIndex returned error: %v", err)
	}
	if len(first.BackupPaths) <= 1 {
		t.Fatalf("expected first rebuild to back up more than one index, got %#v", first.BackupPaths)
	}
	if len(second.BackupPaths) <= 1 {
		t.Fatalf("expected second rebuild to back up more than one index, got %#v", second.BackupPaths)
	}
	all := append(append([]string{}, first.BackupPaths...), second.BackupPaths...)
	seen := map[string]bool{}
	for _, rel := range all {
		if seen[rel] {
			t.Fatalf("backup path reused across rapid rebuilds: %s in %#v", rel, all)
		}
		seen[rel] = true
		if _, err := fs.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("backup path %s does not exist: %v", rel, err)
		}
	}
	if !containsPrefix(first.BackupPaths, "wiki/indexes/tags.md.bak-") {
		t.Fatalf("expected tags index backup in first rebuild, got %#v", first.BackupPaths)
	}
	if first.BackupPath == "" || second.BackupPath == "" {
		t.Fatalf("expected compatibility BackupPath fields, got first=%q second=%q", first.BackupPath, second.BackupPath)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func containsPrefix(items []string, prefix string) bool {
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}
