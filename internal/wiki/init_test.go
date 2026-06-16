package wiki_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/openwiki/internal/wiki"
)

func TestInitCreatesEntitiesDir(t *testing.T) {
	fs := wiki.NewMemFS()
	root := "/test-wiki"

	cfg := map[string]interface{}{
		"wiki_root": root,
	}

	err := wiki.Init(fs, root, cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	entitiesDir := filepath.Join(root, "entities")
	if _, err := fs.Stat(entitiesDir); err != nil {
		t.Errorf("expected entities/ directory to exist at %s, got error: %v", entitiesDir, err)
	}
}

func TestInitCreatesLayeredIndexLayout(t *testing.T) {
	fs := wiki.NewMemFS()
	root := "/test-wiki"
	cfg := map[string]interface{}{"wiki_root": root}

	if err := wiki.Init(fs, root, cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	expectedFiles := []string{
		filepath.Join(root, "wiki", "index.md"),
		filepath.Join(root, "wiki", "indexes", "scopes.md"),
		filepath.Join(root, "wiki", "indexes", "entities.md"),
		filepath.Join(root, "wiki", "indexes", "concepts.md"),
		filepath.Join(root, "wiki", "indexes", "tags.md"),
		filepath.Join(root, "wiki", "indexes", "recent.md"),
		filepath.Join(root, "wiki", "indexes", "hot.md"),
		filepath.Join(root, "wiki", "indexes", "query-usage.jsonl"),
	}

	for _, path := range expectedFiles {
		if _, err := fs.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	indexBytes, err := fs.ReadFile(filepath.Join(root, "wiki", "index.md"))
	if err != nil {
		t.Fatalf("failed to read index.md: %v", err)
	}
	index := string(indexBytes)
	if !strings.Contains(index, "## 检索路由") {
		t.Fatalf("expected routing index to contain 检索路由, got:\n%s", index)
	}
	assertShardIndexLinksUseWikiLinks(t, index)
	if strings.Contains(index, "| Slug | 标题 | 类型 | 标签 | 适用范围 | 最后更新 |") {
		t.Fatalf("routing index must not use the old full page table template")
	}

	queryUsageBytes, err := fs.ReadFile(filepath.Join(root, "wiki", "indexes", "query-usage.jsonl"))
	if err != nil {
		t.Fatalf("failed to read query-usage.jsonl: %v", err)
	}
	if len(queryUsageBytes) != 0 {
		t.Fatalf("expected query-usage.jsonl to be empty, got %q", string(queryUsageBytes))
	}
}

func assertShardIndexLinksUseWikiLinks(t *testing.T, index string) {
	t.Helper()

	for _, link := range []string{
		"[[indexes/scopes]]",
		"[[indexes/entities]]",
		"[[indexes/concepts]]",
		"[[indexes/tags]]",
		"[[indexes/recent]]",
		"[[indexes/hot]]",
	} {
		if !strings.Contains(index, link) {
			t.Fatalf("expected routing index to contain shard wiki link %s, got:\n%s", link, index)
		}
	}

	for _, oldRef := range []string{
		"`indexes/scopes.md`",
		"`indexes/entities.md`",
		"`indexes/concepts.md`",
		"`indexes/tags.md`",
		"`indexes/recent.md`",
		"`indexes/hot.md`",
	} {
		if strings.Contains(index, oldRef) {
			t.Fatalf("expected routing index not to contain old shard path reference %s, got:\n%s", oldRef, index)
		}
	}
}
