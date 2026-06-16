# OpenWiki File-first Layered Index Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the first foundation slice of the file-first OpenWiki runtime: project-local `openwiki.toml`, local-first discovery, layered index initialization, index check/rebuild CLI, and updated wiki skill protocols.

**Architecture:** Keep Markdown files as the primary write interface and add CLI guardrails for deterministic validation and index repair. `openwiki init` creates project-local `openwiki.toml` and an `openwiki/wiki/indexes/` layered index skeleton. New `openwiki index check/rebuild` commands verify and rebuild Routing Index + Shard Indexes from page files and query usage.

**Tech Stack:** Go 1.26.3, standard library, `gopkg.in/yaml.v3`, `github.com/BurntSushi/toml`, Markdown templates, shell commands, existing `testing` package.

---

## Scope and Phasing

The approved design covers multiple subsystems. This plan implements the first shippable foundation slice:

1. Runtime/config consistency.
2. Project-local init layout.
3. Layered index file model.
4. `openwiki index check/rebuild` guardrails.
5. Skill documentation updates to stop requiring CLI page writes and to use layered indexes.
6. Tests for the new contract.

This plan does not implement vector search, advanced contradiction linting, cloud sync changes, or complete removal of `page create/update/delete`.

## Current Worktree Warning

Before executing this plan, inspect the worktree:

```bash
git status --short
```

At the time this plan was written, the repository already had unrelated uncommitted changes. Do not overwrite or stage unrelated files. Each task below lists exact paths to add/modify.

## File Structure

### New files

- `internal/wiki/index.go`
  - Owns layered index data structures, expected index paths, `CheckIndex`, and `RebuildIndex`.
- `internal/wiki/index_test.go`
  - Unit tests for index check/rebuild behavior.
- `internal/cli/index.go`
  - Adds `openwiki index check` and `openwiki index rebuild` command handlers.
- `internal/cli/index_test.go`
  - CLI integration tests for index commands.
- `skill/wiki-init/templates/index.md`
  - Routing Index template.
- `skill/wiki-init/templates/indexes/scopes.md`
  - Empty scope shard template.
- `skill/wiki-init/templates/indexes/entities.md`
  - Empty entity shard template.
- `skill/wiki-init/templates/indexes/concepts.md`
  - Empty concept shard template.
- `skill/wiki-init/templates/indexes/tags.md`
  - Empty tag shard template.
- `skill/wiki-init/templates/indexes/recent.md`
  - Empty recent shard template.
- `skill/wiki-init/templates/indexes/hot.md`
  - Empty hot shard template.
- `skill/wiki-init/templates/indexes/query-usage.jsonl`
  - Empty query usage file.

### Existing files to modify

- `internal/config/discovery.go`
  - Ensure discovery order is `explicit → env → local → global`.
- `internal/config/discovery_test.go`
  - Ensure local-priority test exists and passes.
- `internal/config/config.go`
  - Add helper to resolve `wiki_root` relative to config file path.
- `internal/config/config_test.go`
  - Add relative wiki root resolution tests.
- `internal/config/validate.go`
  - Validate layered index layout.
- `internal/config/validate_test.go`
  - Add validation coverage for missing `wiki/indexes/`.
- `internal/wiki/init.go`
  - Create `wiki/indexes/` and layered index files during init.
- `internal/wiki/init_test.go`
  - Assert new layout is created.
- `internal/cli/root.go`
  - Register `index` command.
- `internal/cli/init.go`
  - Ensure created output includes `openwiki.toml` and indexes paths.
- `internal/cli/init_test.go`
  - Assert project-local `openwiki.toml` and `./openwiki/` default.
- `internal/cli/status.go`
  - Include index health summary.
- `internal/cli/status_test.go`
  - Assert status reports index health.
- `skill/wiki-init/SKILL.md`
  - Use `openwiki.toml`, project-local init, layered indexes.
- `skill/wiki-ingest/SKILL.md`
  - Direct file writes, update shard indexes, no required `openwiki page create`.
- `skill/wiki-query/SKILL.md`
  - Routing Index + Shard Indexes, query usage append.
- `skill/wiki-update/SKILL.md`
  - Direct file edits and shard index maintenance.
- `skill/wiki-lint/SKILL.md`
  - Layered index checks and rebuild guidance.
- `README.md`, `README.en.md`, `README.ja.md`
  - Update product contract and layout docs.

---

### Task 1: Lock local-first config discovery and relative wiki_root resolution

**Files:**
- Modify: `internal/config/discovery.go`
- Modify: `internal/config/discovery_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Test: `internal/config/...`

- [ ] **Step 1: Write failing test for relative wiki_root resolution**

Add this test to `internal/config/config_test.go`:

```go
func TestLoadResolvesRelativeWikiRootFromConfigDir(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "./openwiki"

[wiki]
primary_language = "zh"
secondary_language = "en"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(dir, "openwiki")
	if cfg.WikiRoot != expected {
		t.Fatalf("expected wiki_root=%s, got %s", expected, cfg.WikiRoot)
	}
}
```

Ensure imports include:

```go
import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/openwiki/internal/config"
)
```

If the file already imports some of these packages, merge without duplicates.

- [ ] **Step 2: Run the new test to verify it fails**

Run:

```bash
go test ./internal/config/... -run TestLoadResolvesRelativeWikiRootFromConfigDir -count=1 -v
```

Expected: FAIL because `Load` currently returns `./openwiki` unchanged.

- [ ] **Step 3: Implement config-relative wiki_root resolution**

In `internal/config/config.go`, update `Load` so relative `wiki_root` values are resolved from the config file directory:

```go
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析 TOML 配置失败: %w", err)
	}

	if cfg.WikiRoot != "" && !filepath.IsAbs(cfg.WikiRoot) {
		cfg.WikiRoot = filepath.Clean(filepath.Join(filepath.Dir(path), cfg.WikiRoot))
	}

	return &cfg, nil
}
```

Add `path/filepath` to the imports:

```go
import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)
```

- [ ] **Step 4: Ensure local discovery is before global discovery**

In `internal/config/discovery.go`, ensure `Discover` checks in this order:

```go
func (d *DefaultDiscoverer) Discover(explicitPath string) (*DiscoveryResult, error) {
	if explicitPath != "" {
		path := expandPath(explicitPath)
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, path)
		}
		return &DiscoveryResult{Path: path, Source: "explicit"}, nil
	}

	if envPath := d.Getenv("OPENWIKI_CONFIG"); envPath != "" {
		path := expandPath(envPath)
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("%w: OPENWIKI_CONFIG=%s", ErrConfigNotFound, envPath)
		}
		return &DiscoveryResult{Path: path, Source: "env"}, nil
	}

	cwd, err := d.Getwd()
	if err != nil {
		return nil, fmt.Errorf("获取当前工作目录失败: %w", err)
	}
	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "openwiki.toml")
		if _, err := os.Stat(candidate); err == nil {
			return &DiscoveryResult{Path: candidate, Source: "local"}, nil
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}

	globalPath := filepath.Join(d.HomeDir, ".openwiki", "openwiki.toml")
	if _, err := os.Stat(globalPath); err == nil {
		return &DiscoveryResult{Path: globalPath, Source: "global"}, nil
	}

	return nil, ErrConfigNotFound
}
```

- [ ] **Step 5: Ensure local-priority test exists**

If `internal/config/discovery_test.go` does not already contain it, add:

```go
func TestDiscoverLocalPriorityOverGlobal(t *testing.T) {
	homeDir := t.TempDir()
	openwikiDir := filepath.Join(homeDir, ".openwiki")
	if err := os.MkdirAll(openwikiDir, 0755); err != nil {
		t.Fatalf("failed to create .openwiki dir: %v", err)
	}
	createTestTOML(t, openwikiDir)

	localDir := t.TempDir()
	localTomlPath := createTestTOML(t, localDir)

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
```

- [ ] **Step 6: Run config tests**

Run:

```bash
go test ./internal/config/... -count=1 -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go internal/config/discovery.go internal/config/discovery_test.go
git commit -m "feat: 统一项目本地配置发现规则"
```

---

### Task 2: Create layered index templates during wiki initialization

**Files:**
- Create: `skill/wiki-init/templates/index.md`
- Create: `skill/wiki-init/templates/indexes/scopes.md`
- Create: `skill/wiki-init/templates/indexes/entities.md`
- Create: `skill/wiki-init/templates/indexes/concepts.md`
- Create: `skill/wiki-init/templates/indexes/tags.md`
- Create: `skill/wiki-init/templates/indexes/recent.md`
- Create: `skill/wiki-init/templates/indexes/hot.md`
- Create: `skill/wiki-init/templates/indexes/query-usage.jsonl`
- Modify: `internal/wiki/init.go`
- Modify: `internal/wiki/init_test.go`
- Test: `internal/wiki/...`

- [ ] **Step 1: Write failing init layout test**

Add this test to `internal/wiki/init_test.go`:

```go
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
	if strings.Contains(index, "| Slug | 标题 | 类型 | 标签 | 适用范围 | 最后更新 |") {
		t.Fatalf("routing index must not use the old full page table template")
	}
}
```

Ensure imports include:

```go
import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/openwiki/internal/wiki"
)
```

- [ ] **Step 2: Run the new test to verify it fails**

Run:

```bash
go test ./internal/wiki/... -run TestInitCreatesLayeredIndexLayout -count=1 -v
```

Expected: FAIL because `wiki/indexes/` files are not created yet.

- [ ] **Step 3: Add template files**

Create `skill/wiki-init/templates/index.md`:

```markdown
# Wiki 索引

## 概览

本 Wiki 使用 OpenWiki 分层索引结构。顶层 index.md 只负责检索路由，不列出全量页面。

## 检索路由

按以下顺序选择分片索引：

1. Scope 线索：项目、仓库、模块、领域 → `indexes/scopes.md`
2. Entity 线索：人、组织、项目、工具 → `indexes/entities.md`
3. Concept 线索：设计、原则、决策、分析 → `indexes/concepts.md`
4. Tag 线索：关键词、主题标签 → `indexes/tags.md`
5. Recency 线索：当前、最近、最新状态 → `indexes/recent.md`
6. 不确定时：先读 `indexes/hot.md` 和 `indexes/recent.md`

## 分片索引

| 索引 | 覆盖内容 | 何时读取 |
|---|---|---|
| `indexes/scopes.md` | 按 scope 分组的入口 | 问题含项目/模块/领域线索 |
| `indexes/entities.md` | 实体页入口 | 问题涉及具体人/组织/项目/工具 |
| `indexes/concepts.md` | 概念与分析入口 | 问题涉及设计原则/决策背景 |
| `indexes/tags.md` | 标签入口 | 问题含关键词但 scope 不明确 |
| `indexes/recent.md` | 最近更新 | 问题问当前状态或最新变化 |
| `indexes/hot.md` | 查询热度入口 | 不确定或常见问题优先 |

## 索引状态

- Last rebuilt: never
- Page count: 0
- Index health: ok
- Query usage records: 0
- Known gaps:
  - none
```

Create `skill/wiki-init/templates/indexes/scopes.md`:

```markdown
# Scope 索引

| Scope | 说明 | 分片 |
|---|---|---|
```

Create `skill/wiki-init/templates/indexes/entities.md`:

```markdown
# Entity 索引

| 页面 | Entity Type | 标签 | 更新 | 摘要 |
|---|---|---|---|---|
```

Create `skill/wiki-init/templates/indexes/concepts.md`:

```markdown
# Concept 索引

| 页面 | Concept Type | Scope | 标签 | 更新 | 摘要 |
|---|---|---|---|---|---|
```

Create `skill/wiki-init/templates/indexes/tags.md`:

```markdown
# Tag 索引

```

Create `skill/wiki-init/templates/indexes/recent.md`:

```markdown
# 最近更新

| 页面 | 类型 | Scope | 标签 | 更新 | 摘要 |
|---|---|---|---|---|---|
```

Create `skill/wiki-init/templates/indexes/hot.md`:

```markdown
# 热门入口

> 自动生成自最近查询记录。

## 最近 30 天高频页面

| 页面 | 命中次数 | 最近命中 | 常见问题 |
|---|---:|---|---|

## 高频查询主题

| 主题 | 相关页面 | 命中次数 |
|---|---|---:|
```

Create empty `skill/wiki-init/templates/indexes/query-usage.jsonl`:

```text
```

- [ ] **Step 4: Update `internal/wiki/init.go` templates**

Replace the old `indexTemplate` with the routing template:

```go
const indexTemplate = `# Wiki 索引

## 概览

本 Wiki 使用 OpenWiki 分层索引结构。顶层 index.md 只负责检索路由，不列出全量页面。

## 检索路由

按以下顺序选择分片索引：

1. Scope 线索：项目、仓库、模块、领域 → ` + "`" + `indexes/scopes.md` + "`" + `
2. Entity 线索：人、组织、项目、工具 → ` + "`" + `indexes/entities.md` + "`" + `
3. Concept 线索：设计、原则、决策、分析 → ` + "`" + `indexes/concepts.md` + "`" + `
4. Tag 线索：关键词、主题标签 → ` + "`" + `indexes/tags.md` + "`" + `
5. Recency 线索：当前、最近、最新状态 → ` + "`" + `indexes/recent.md` + "`" + `
6. 不确定时：先读 ` + "`" + `indexes/hot.md` + "`" + ` 和 ` + "`" + `indexes/recent.md` + "`" + `

## 分片索引

| 索引 | 覆盖内容 | 何时读取 |
|---|---|---|
| ` + "`" + `indexes/scopes.md` + "`" + ` | 按 scope 分组的入口 | 问题含项目/模块/领域线索 |
| ` + "`" + `indexes/entities.md` + "`" + ` | 实体页入口 | 问题涉及具体人/组织/项目/工具 |
| ` + "`" + `indexes/concepts.md` + "`" + ` | 概念与分析入口 | 问题涉及设计原则/决策背景 |
| ` + "`" + `indexes/tags.md` + "`" + ` | 标签入口 | 问题含关键词但 scope 不明确 |
| ` + "`" + `indexes/recent.md` + "`" + ` | 最近更新 | 问题问当前状态或最新变化 |
| ` + "`" + `indexes/hot.md` + "`" + ` | 查询热度入口 | 不确定或常见问题优先 |

## 索引状态

- Last rebuilt: never
- Page count: 0
- Index health: ok
- Query usage records: 0
- Known gaps:
  - none
`
```

Add shard templates below `logTemplate`:

```go
const scopesIndexTemplate = `# Scope 索引

| Scope | 说明 | 分片 |
|---|---|---|
`

const entitiesIndexTemplate = `# Entity 索引

| 页面 | Entity Type | 标签 | 更新 | 摘要 |
|---|---|---|---|---|
`

const conceptsIndexTemplate = `# Concept 索引

| 页面 | Concept Type | Scope | 标签 | 更新 | 摘要 |
|---|---|---|---|---|---|
`

const tagsIndexTemplate = `# Tag 索引

`

const recentIndexTemplate = `# 最近更新

| 页面 | 类型 | Scope | 标签 | 更新 | 摘要 |
|---|---|---|---|---|---|
`

const hotIndexTemplate = `# 热门入口

> 自动生成自最近查询记录。

## 最近 30 天高频页面

| 页面 | 命中次数 | 最近命中 | 常见问题 |
|---|---:|---|---|

## 高频查询主题

| 主题 | 相关页面 | 命中次数 |
|---|---|---:|
`
```

- [ ] **Step 5: Create indexes directory and files in init**

In `initInternal`, include the indexes directory:

```go
dirs := []string{
	filepath.Join(root, "wiki", "pages"),
	filepath.Join(root, "wiki", "indexes"),
	filepath.Join(root, "raw"),
	filepath.Join(root, "concepts"),
	filepath.Join(root, "entities"),
}
```

After writing `log.md`, write shard index files:

```go
indexFiles := map[string]string{
	filepath.Join(root, "wiki", "indexes", "scopes.md"):          scopesIndexTemplate,
	filepath.Join(root, "wiki", "indexes", "entities.md"):        entitiesIndexTemplate,
	filepath.Join(root, "wiki", "indexes", "concepts.md"):        conceptsIndexTemplate,
	filepath.Join(root, "wiki", "indexes", "tags.md"):            tagsIndexTemplate,
	filepath.Join(root, "wiki", "indexes", "recent.md"):          recentIndexTemplate,
	filepath.Join(root, "wiki", "indexes", "hot.md"):             hotIndexTemplate,
	filepath.Join(root, "wiki", "indexes", "query-usage.jsonl"): "",
}
for path, content := range indexFiles {
	if err := fs.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("创建索引文件失败 %s: %w", path, err)
	}
}
```

- [ ] **Step 6: Run wiki tests**

Run:

```bash
go test ./internal/wiki/... -count=1 -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/wiki/init.go internal/wiki/init_test.go skill/wiki-init/templates/index.md skill/wiki-init/templates/indexes
git commit -m "feat: 初始化分层索引文件结构"
```

---

### Task 3: Add index check and rebuild core logic

**Files:**
- Create: `internal/wiki/index.go`
- Create: `internal/wiki/index_test.go`
- Test: `internal/wiki/...`

- [ ] **Step 1: Write failing tests for `CheckIndex`**

Create `internal/wiki/index_test.go`:

```go
package wiki_test

import (
	"path/filepath"
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
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/wiki/... -run 'TestCheckIndex' -count=1 -v
```

Expected: FAIL because `wiki.CheckIndex` is not defined.

- [ ] **Step 3: Implement `internal/wiki/index.go`**

Create `internal/wiki/index.go`:

```go
package wiki

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var requiredIndexFiles = []string{
	"wiki/index.md",
	"wiki/indexes/scopes.md",
	"wiki/indexes/entities.md",
	"wiki/indexes/concepts.md",
	"wiki/indexes/tags.md",
	"wiki/indexes/recent.md",
	"wiki/indexes/hot.md",
	"wiki/indexes/query-usage.jsonl",
}

type IndexCheckResult struct {
	Health         string   `json:"health"`
	MissingFiles   []string `json:"missing_files,omitempty"`
	UnindexedPages []string `json:"unindexed_pages,omitempty"`
	PageCount      int      `json:"page_count"`
}

type IndexRebuildResult struct {
	RebuiltFiles []string `json:"rebuilt_files"`
	PageCount    int      `json:"page_count"`
	BackupPath   string   `json:"backup_path,omitempty"`
}

func CheckIndex(fs FS, root string) (*IndexCheckResult, error) {
	result := &IndexCheckResult{Health: "ok"}

	for _, rel := range requiredIndexFiles {
		if _, err := fs.Stat(filepath.Join(root, rel)); err != nil {
			result.MissingFiles = append(result.MissingFiles, rel)
		}
	}

	pages, err := ListPages(fs, root)
	if err != nil {
		return nil, err
	}
	result.PageCount = len(pages)

	indexed := make(map[string]bool)
	for _, rel := range []string{
		"wiki/indexes/scopes.md",
		"wiki/indexes/entities.md",
		"wiki/indexes/concepts.md",
		"wiki/indexes/tags.md",
		"wiki/indexes/recent.md",
		"wiki/indexes/hot.md",
	} {
		data, err := fs.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		for _, slug := range extractWikiLinks(string(data)) {
			indexed[slug] = true
		}
	}

	for _, page := range pages {
		if !indexed[page.Slug] {
			result.UnindexedPages = append(result.UnindexedPages, page.Slug)
		}
	}
	sort.Strings(result.MissingFiles)
	sort.Strings(result.UnindexedPages)

	if len(result.MissingFiles) > 0 || len(result.UnindexedPages) > 0 {
		result.Health = "warning"
	}

	return result, nil
}

func RebuildIndex(fs FS, root string) (*IndexRebuildResult, error) {
	pages, err := ListPages(fs, root)
	if err != nil {
		return nil, err
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Slug < pages[j].Slug })

	if err := fs.MkdirAll(filepath.Join(root, "wiki", "indexes"), 0755); err != nil {
		return nil, fmt.Errorf("创建 indexes 目录失败: %w", err)
	}

	result := &IndexRebuildResult{PageCount: len(pages)}
	indexPath := filepath.Join(root, "wiki", "index.md")
	if data, err := fs.ReadFile(indexPath); err == nil {
		backupRel := fmt.Sprintf("wiki/index.md.bak-%s", time.Now().Format("20060102150405"))
		backupPath := filepath.Join(root, backupRel)
		if err := fs.WriteFile(backupPath, data, 0644); err != nil {
			return nil, fmt.Errorf("备份 index.md 失败: %w", err)
		}
		result.BackupPath = backupRel
	}

	files := map[string]string{
		"wiki/index.md":                      buildRoutingIndex(len(pages), countQueryUsage(fs, root)),
		"wiki/indexes/scopes.md":             buildScopesIndex(pages),
		"wiki/indexes/entities.md":           buildTypeIndex(pages, "entity"),
		"wiki/indexes/concepts.md":           buildTypeIndex(pages, "concept"),
		"wiki/indexes/tags.md":               buildTagsIndex(pages),
		"wiki/indexes/recent.md":             buildRecentIndex(pages),
		"wiki/indexes/hot.md":                buildHotIndex(fs, root),
		"wiki/indexes/query-usage.jsonl": ensureQueryUsage(fs, root),
	}

	var rels []string
	for rel := range files {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		if err := fs.WriteFile(filepath.Join(root, rel), []byte(files[rel]), 0644); err != nil {
			return nil, fmt.Errorf("写入索引文件失败 %s: %w", rel, err)
		}
		result.RebuiltFiles = append(result.RebuiltFiles, rel)
	}

	return result, nil
}

func extractWikiLinks(content string) []string {
	matches := regexpWikiLink.FindAllStringSubmatch(content, -1)
	var slugs []string
	for _, m := range matches {
		if len(m) > 1 {
			slugs = append(slugs, m[1])
		}
	}
	return slugs
}

var regexpWikiLink = regexp.MustCompile(`\[\[([a-zA-Z0-9_-]+)\]\]`)
```

Add `regexp` to imports in the snippet above:

```go
import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)
```

- [ ] **Step 4: Add rebuild helper functions**

Append these helpers to `internal/wiki/index.go`:

```go
func buildRoutingIndex(pageCount, usageCount int) string {
	return fmt.Sprintf(`# Wiki 索引

## 概览

本 Wiki 使用 OpenWiki 分层索引结构。顶层 index.md 只负责检索路由，不列出全量页面。

## 检索路由

按以下顺序选择分片索引：

1. Scope 线索：项目、仓库、模块、领域 → `+"`"+`indexes/scopes.md`+"`"+`
2. Entity 线索：人、组织、项目、工具 → `+"`"+`indexes/entities.md`+"`"+`
3. Concept 线索：设计、原则、决策、分析 → `+"`"+`indexes/concepts.md`+"`"+`
4. Tag 线索：关键词、主题标签 → `+"`"+`indexes/tags.md`+"`"+`
5. Recency 线索：当前、最近、最新状态 → `+"`"+`indexes/recent.md`+"`"+`
6. 不确定时：先读 `+"`"+`indexes/hot.md`+"`"+` 和 `+"`"+`indexes/recent.md`+"`"+`

## 分片索引

| 索引 | 覆盖内容 | 何时读取 |
|---|---|---|
| `+"`"+`indexes/scopes.md`+"`"+` | 按 scope 分组的入口 | 问题含项目/模块/领域线索 |
| `+"`"+`indexes/entities.md`+"`"+` | 实体页入口 | 问题涉及具体人/组织/项目/工具 |
| `+"`"+`indexes/concepts.md`+"`"+` | 概念与分析入口 | 问题涉及设计原则/决策背景 |
| `+"`"+`indexes/tags.md`+"`"+` | 标签入口 | 问题含关键词但 scope 不明确 |
| `+"`"+`indexes/recent.md`+"`"+` | 最近更新 | 问题问当前状态或最新变化 |
| `+"`"+`indexes/hot.md`+"`"+` | 查询热度入口 | 不确定或常见问题优先 |

## 索引状态

- Last rebuilt: %s
- Page count: %d
- Index health: ok
- Query usage records: %d
- Known gaps:
  - none
`, time.Now().Format("2006-01-02"), pageCount, usageCount)
}

func buildScopesIndex(pages []PageMeta) string {
	counts := map[string]int{}
	for _, page := range pages {
		scope := joinScope(page.ScopeLevel, page.ScopeCode)
		if scope != "" {
			counts[scope]++
		}
	}
	var scopes []string
	for scope := range counts {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	var sb strings.Builder
	sb.WriteString("# Scope 索引\n\n| Scope | 说明 | 分片 |\n|---|---|---|\n")
	for _, scope := range scopes {
		sb.WriteString(fmt.Sprintf("| `%s` | %d 个页面 | |\n", scope, counts[scope]))
	}
	return sb.String()
}

func buildTypeIndex(pages []PageMeta, pageType string) string {
	title := "Entity 索引"
	if pageType == "concept" {
		title = "Concept 索引"
	}
	var sb strings.Builder
	sb.WriteString("# " + title + "\n\n| 页面 | 类型 | Scope | 标签 | 更新 | 摘要 |\n|---|---|---|---|---|---|\n")
	for _, page := range pages {
		if page.Type != pageType {
			continue
		}
		sb.WriteString(formatIndexRow(page))
	}
	return sb.String()
}

func buildTagsIndex(pages []PageMeta) string {
	byTag := map[string][]PageMeta{}
	for _, page := range pages {
		for _, tag := range page.Tags {
			byTag[tag] = append(byTag[tag], page)
		}
	}
	var tags []string
	for tag := range byTag {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	var sb strings.Builder
	sb.WriteString("# Tag 索引\n\n")
	for _, tag := range tags {
		sb.WriteString("## " + tag + "\n\n")
		for _, page := range byTag[tag] {
			sb.WriteString(fmt.Sprintf("- [[%s]] — %s | %s | %s | %s\n", page.Slug, page.Title, page.Type, joinScope(page.ScopeLevel, page.ScopeCode), page.Updated))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func buildRecentIndex(pages []PageMeta) string {
	sort.Slice(pages, func(i, j int) bool {
		if pages[i].Updated == pages[j].Updated {
			return pages[i].Slug < pages[j].Slug
		}
		return pages[i].Updated > pages[j].Updated
	})
	var sb strings.Builder
	sb.WriteString("# 最近更新\n\n| 页面 | 类型 | Scope | 标签 | 更新 | 摘要 |\n|---|---|---|---|---|---|\n")
	for _, page := range pages {
		sb.WriteString(formatIndexRow(page))
	}
	return sb.String()
}

func buildHotIndex(fs FS, root string) string {
	return `# 热门入口

> 自动生成自最近查询记录。

## 最近 30 天高频页面

| 页面 | 命中次数 | 最近命中 | 常见问题 |
|---|---:|---|---|

## 高频查询主题

| 主题 | 相关页面 | 命中次数 |
|---|---|---:|
`
}

func ensureQueryUsage(fs FS, root string) string {
	data, err := fs.ReadFile(filepath.Join(root, "wiki", "indexes", "query-usage.jsonl"))
	if err != nil {
		return ""
	}
	return string(data)
}

func countQueryUsage(fs FS, root string) int {
	data, err := fs.ReadFile(filepath.Join(root, "wiki", "indexes", "query-usage.jsonl"))
	if err != nil {
		return 0
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

func formatIndexRow(page PageMeta) string {
	return fmt.Sprintf("| [[%s]] | %s | %s | %s | %s | %s |\n",
		page.Slug,
		page.Type,
		joinScope(page.ScopeLevel, page.ScopeCode),
		strings.Join(page.Tags, ","),
		page.Updated,
		page.Title,
	)
}

func joinScope(level, code string) string {
	if level == "" {
		return code
	}
	if code == "" {
		return level
	}
	return level + "/" + code
}
```

- [ ] **Step 5: Run index tests**

Run:

```bash
go test ./internal/wiki/... -run 'TestCheckIndex|TestInitCreatesLayeredIndexLayout' -count=1 -v
```

Expected: PASS.

- [ ] **Step 6: Add rebuild test**

Append to `internal/wiki/index_test.go`:

```go
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
```

Ensure `strings` is imported.

- [ ] **Step 7: Run wiki tests**

Run:

```bash
go test ./internal/wiki/... -count=1 -v
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/wiki/index.go internal/wiki/index_test.go
git commit -m "feat: 添加分层索引检查和重建能力"
```

---

### Task 4: Add `openwiki index check/rebuild` CLI

**Files:**
- Create: `internal/cli/index.go`
- Create: `internal/cli/index_test.go`
- Modify: `internal/cli/root.go`
- Test: `internal/cli/...`

- [ ] **Step 1: Write failing CLI tests**

Create `internal/cli/index_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/cli/... -run 'TestIndexCheckJSON|TestIndexRebuildJSON' -count=1 -v
```

Expected: FAIL with unknown command `index`.

- [ ] **Step 3: Register index command**

In `internal/cli/root.go`, add to the switch:

```go
case "index":
	return runIndex(stdout, stderr, &opts, subArgs)
```

Update help text command list:

```text
  index    检查和重建分层索引
```

- [ ] **Step 4: Implement `internal/cli/index.go`**

Create `internal/cli/index.go`:

```go
package cli

import (
	"fmt"
	"io"

	"github.com/bytedance/openwiki/internal/output"
	"github.com/bytedance/openwiki/internal/wiki"
)

func runIndex(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("index 需要子命令: check, rebuild")
	}

	subcommand := args[0]
	subArgs := args[1:]
	_ = subArgs

	switch subcommand {
	case "check":
		return runIndexCheck(stdout, stderr, opts)
	case "rebuild":
		return runIndexRebuild(stdout, stderr, opts)
	default:
		return fmt.Errorf("未知 index 子命令: %s", subcommand)
	}
}

func runIndexCheck(stdout, stderr io.Writer, opts *GlobalOptions) error {
	cfg, _, err := discoverConfig(opts)
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{Code: "CONFIG_NOT_FOUND", Message: err.Error()})
		}
		return err
	}

	result, err := wiki.CheckIndex(wiki.NewOsFS(), cfg.WikiRoot)
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{Code: "INDEX_CHECK_FAILED", Message: err.Error()})
		}
		return err
	}

	if opts.JSON {
		return output.JSON(stdout, true, result, nil)
	}

	fmt.Fprintf(stdout, "索引健康状态: %s\n", result.Health)
	fmt.Fprintf(stdout, "页面总数: %d\n", result.PageCount)
	if len(result.MissingFiles) > 0 {
		fmt.Fprintf(stdout, "缺失索引文件: %v\n", result.MissingFiles)
	}
	if len(result.UnindexedPages) > 0 {
		fmt.Fprintf(stdout, "未索引页面: %v\n", result.UnindexedPages)
	}
	return nil
}

func runIndexRebuild(stdout, stderr io.Writer, opts *GlobalOptions) error {
	cfg, _, err := discoverConfig(opts)
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{Code: "CONFIG_NOT_FOUND", Message: err.Error()})
		}
		return err
	}

	result, err := wiki.RebuildIndex(wiki.NewOsFS(), cfg.WikiRoot)
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{Code: "INDEX_REBUILD_FAILED", Message: err.Error()})
		}
		return err
	}

	if opts.JSON {
		return output.JSON(stdout, true, result, nil)
	}

	fmt.Fprintf(stdout, "索引已重建，页面总数: %d\n", result.PageCount)
	for _, file := range result.RebuiltFiles {
		fmt.Fprintf(stdout, "  %s\n", file)
	}
	if result.BackupPath != "" {
		fmt.Fprintf(stdout, "旧 index 备份: %s\n", result.BackupPath)
	}
	return nil
}
```

- [ ] **Step 5: Run index CLI tests**

Run:

```bash
go test ./internal/cli/... -run 'TestIndexCheckJSON|TestIndexRebuildJSON' -count=1 -v
```

Expected: PASS.

- [ ] **Step 6: Run all CLI tests**

Run:

```bash
go test ./internal/cli/... -count=1 -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/root.go internal/cli/index.go internal/cli/index_test.go
git commit -m "feat: 添加分层索引命令"
```

---

### Task 5: Add index health to status and config validation

**Files:**
- Modify: `internal/config/validate.go`
- Modify: `internal/config/validate_test.go`
- Modify: `internal/cli/status.go`
- Modify: `internal/cli/status_test.go`
- Test: `internal/config/...`, `internal/cli/...`

- [ ] **Step 1: Write failing validation test for missing indexes directory**

Add to `internal/config/validate_test.go`:

```go
func TestValidateMissingIndexesDirectory(t *testing.T) {
	dir := t.TempDir()
	wikiRoot := filepath.Join(dir, "openwiki")
	if err := os.MkdirAll(filepath.Join(wikiRoot, "wiki", "pages"), 0755); err != nil {
		t.Fatalf("mkdir pages failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(wikiRoot, "raw"), 0755); err != nil {
		t.Fatalf("mkdir raw failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(wikiRoot, "entities"), 0755); err != nil {
		t.Fatalf("mkdir entities failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(wikiRoot, "concepts"), 0755); err != nil {
		t.Fatalf("mkdir concepts failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wikiRoot, "wiki", "index.md"), []byte("# Wiki 索引\n"), 0644); err != nil {
		t.Fatalf("write index failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wikiRoot, "wiki", "log.md"), []byte("# 操作日志\n"), 0644); err != nil {
		t.Fatalf("write log failed: %v", err)
	}

	cfg := &config.Config{
		WikiRoot: wikiRoot,
		Wiki: config.WikiConfig{PrimaryLanguage: "zh", SecondaryLanguage: "en"},
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error for missing wiki/indexes directory")
	}
	if !strings.Contains(err.Error(), "wiki/indexes") {
		t.Fatalf("expected error to mention wiki/indexes, got %v", err)
	}
}
```

Ensure imports include `os`, `path/filepath`, `strings`, `testing`.

- [ ] **Step 2: Run validation test to verify failure**

Run:

```bash
go test ./internal/config/... -run TestValidateMissingIndexesDirectory -count=1 -v
```

Expected: FAIL because `Validate` does not check layout.

- [ ] **Step 3: Implement layout validation**

In `internal/config/validate.go`, after checking `wiki_root` exists and before language validation, add:

```go
requiredPaths := []string{
	"raw",
	"wiki",
	filepath.Join("wiki", "pages"),
	filepath.Join("wiki", "indexes"),
	filepath.Join("wiki", "index.md"),
	filepath.Join("wiki", "log.md"),
	"entities",
	"concepts",
}
for _, rel := range requiredPaths {
	path := filepath.Join(cfg.WikiRoot, rel)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &ValidationError{
			Code:    "WIKI_LAYOUT_INVALID",
			Message: fmt.Sprintf("wiki_root 缺少必要路径: %s", rel),
			Details: map[string]interface{}{"field": "wiki_root", "path": path},
		}
	}
}
```

Add `path/filepath` to imports.

- [ ] **Step 4: Write failing status test for index health**

Add to `internal/cli/status_test.go`:

```go
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
		t.Fatalf("expected success=true, got error: %#v", resp.Error)
	}

	data := resp.Data.(map[string]interface{})
	indexData, ok := data["index"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data.index to be present, got %#v", data)
	}
	if indexData["health"] == "" {
		t.Fatalf("expected index.health to be non-empty")
	}
}
```

- [ ] **Step 5: Update status types and logic**

In `internal/cli/status.go`, add an `Index` field:

```go
type StatusResult struct {
	Pages   PageStats        `json:"pages"`
	Config  ConfigInfo       `json:"config"`
	Index   IndexStatus      `json:"index"`
	Details []PageDetail     `json:"details,omitempty"`
}

type IndexStatus struct {
	Health         string   `json:"health"`
	MissingFiles   []string `json:"missing_files,omitempty"`
	UnindexedPages []string `json:"unindexed_pages,omitempty"`
}
```

Before constructing `statusResult`, call:

```go
indexCheck, indexErr := wiki.CheckIndex(fs, cfg.WikiRoot)
indexStatus := IndexStatus{Health: "unknown"}
if indexErr == nil {
	indexStatus = IndexStatus{
		Health:         indexCheck.Health,
		MissingFiles:   indexCheck.MissingFiles,
		UnindexedPages: indexCheck.UnindexedPages,
	}
}
```

Then include it:

```go
statusResult := StatusResult{
	Pages: stats,
	Config: ConfigInfo{
		Source: result.Source,
		Path:   result.Path,
	},
	Index:   indexStatus,
	Details: details,
}
```

For text output, add:

```go
fmt.Fprintf(stdout, "索引健康状态: %s\n", indexStatus.Health)
if len(indexStatus.MissingFiles) > 0 {
	fmt.Fprintf(stdout, "缺失索引文件: %v\n", indexStatus.MissingFiles)
}
if len(indexStatus.UnindexedPages) > 0 {
	fmt.Fprintf(stdout, "未索引页面: %v\n", indexStatus.UnindexedPages)
}
```

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/config/... -count=1 -v
go test ./internal/cli/... -run 'TestStatus|TestIndex' -count=1 -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/config/validate.go internal/config/validate_test.go internal/cli/status.go internal/cli/status_test.go
git commit -m "feat: 校验分层索引运行状态"
```

---

### Task 6: Update skill docs for file-first layered index protocol

**Files:**
- Modify: `skill/wiki-init/SKILL.md`
- Modify: `skill/wiki-ingest/SKILL.md`
- Modify: `skill/wiki-query/SKILL.md`
- Modify: `skill/wiki-update/SKILL.md`
- Modify: `skill/wiki-lint/SKILL.md`
- Test: static grep commands

- [ ] **Step 1: Update `wiki-init` skill**

Replace `skill/wiki-init/SKILL.md` content with this contract:

```markdown
---
name: wiki-init
description: Use when bootstrapping a new project-local OpenWiki instance backed by openwiki.toml and layered indexes.
---
# Wiki Init

Bootstrap a project-local OpenWiki runtime.

## Runtime Contract

- `openwiki.toml` is the only canonical runtime contract.
- `WIKI.md` is not used as the runtime contract.
- The default config location is the current project directory.
- The default wiki root is `./openwiki/`.

## Process

### 1. Confirm target

If the user does not specify a wiki root, use:

```text
./openwiki/
```

If `openwiki.toml` already exists in the current directory, treat the directory as an existing OpenWiki project. Do not overwrite unless the user explicitly asks for force reinitialization.

### 2. Initialize with CLI

```bash
openwiki init --non-interactive --json
```

For a custom wiki root:

```bash
openwiki init <wiki-root> --non-interactive --json
```

For force repair or overwrite of config only:

```bash
openwiki init <wiki-root> --force --non-interactive --json
```

`--force` must not be treated as permission to delete existing wiki data.

### 3. Expected layout

```text
<project>/
├── openwiki.toml
└── openwiki/
    ├── raw/
    ├── wiki/
    │   ├── index.md
    │   ├── log.md
    │   ├── pages/
    │   └── indexes/
    │       ├── scopes.md
    │       ├── entities.md
    │       ├── concepts.md
    │       ├── tags.md
    │       ├── recent.md
    │       ├── hot.md
    │       └── query-usage.jsonl
    ├── entities/
    └── concepts/
```

### 4. Validate

Run:

```bash
openwiki config validate
openwiki status
openwiki index check
```

### 5. Confirm

Tell the user:

- Configuration file: `openwiki.toml`
- Wiki root: resolved `wiki_root`
- Routing index: `wiki/index.md`
- Shard indexes: `wiki/indexes/`
- Next steps: add sources to `raw/`, use `wiki-ingest`, use `wiki-query`, run `openwiki status` periodically.
```

- [ ] **Step 2: Update `wiki-ingest` skill**

In `skill/wiki-ingest/SKILL.md`, make these explicit edits:

1. Replace any instruction requiring `openwiki page create` for primary writing with:

```markdown
AI writes Markdown files directly using templates. Do not require `openwiki page create` for content writes.
```

2. Add a section:

```markdown
## Layered Index Write Protocol

After writing or updating a page, update the relevant shard indexes:

- Summary page in `wiki/pages/` → update `wiki/indexes/scopes.md`, `wiki/indexes/tags.md`, and `wiki/indexes/recent.md`
- Entity page in `entities/` → update `wiki/indexes/entities.md`, `wiki/indexes/tags.md`, and `wiki/indexes/recent.md`
- Concept page in `concepts/` → update `wiki/indexes/concepts.md`, `wiki/indexes/tags.md`, and `wiki/indexes/recent.md`

Do not append all page rows to `wiki/index.md`. `wiki/index.md` is a Routing Index and must stay lightweight.

If shard updates fail or are uncertain, warn the user that the page may not be discoverable and recommend:

```bash
openwiki index rebuild
```
```

3. Replace the query/write verification command with:

```markdown
After writing files, verify:

```bash
openwiki index check
openwiki status
```
```

- [ ] **Step 3: Update `wiki-query` skill**

In `skill/wiki-query/SKILL.md`, replace the query pre-read flow with:

```markdown
## Query Flow

1. Read `openwiki.toml` and resolve `wiki_root`.
2. Read `wiki/index.md` as the lightweight Routing Index.
3. Use the routing dimensions to choose shard indexes:
   - Scope clue → `wiki/indexes/scopes.md`
   - Entity clue → `wiki/indexes/entities.md`
   - Concept clue → `wiki/indexes/concepts.md`
   - Tag clue → `wiki/indexes/tags.md`
   - Recent/current clue → `wiki/indexes/recent.md`
   - Unclear query → `wiki/indexes/hot.md` and `wiki/indexes/recent.md`
4. Read 1-3 relevant shard indexes.
5. Select candidate pages.
6. Read candidate pages in full.
7. Answer with `[[slug]]` citations.
8. Append a JSON line to `wiki/indexes/query-usage.jsonl`.
```

Add query usage format:

```markdown
Append query usage as one JSON object per line:

```json
{"time":"2026-06-15T15:30:00+08:00","query":"用户问题","matched_indexes":["indexes/tags.md"],"read_pages":["slug"],"cited_pages":["slug"],"intent_tags":["tag"]}
```
```

- [ ] **Step 4: Update `wiki-update` skill**

Add this section to `skill/wiki-update/SKILL.md`:

```markdown
## Layered Index Update Protocol

When changing page title, summary, tags, scope, type, or updated date:

1. Remove stale entries from old shard indexes.
2. Add updated entries to new shard indexes.
3. Update `wiki/indexes/recent.md`.
4. Keep `wiki/index.md` lightweight; do not add full page rows to it.
5. Append `wiki/log.md`.
6. If unsure whether all shard indexes were updated correctly, run or recommend:

```bash
openwiki index check
openwiki index rebuild
```
```

- [ ] **Step 5: Update `wiki-lint` skill**

Add this section to `skill/wiki-lint/SKILL.md`:

```markdown
## Layered Index Checks

Audit:

- `wiki/index.md` exists and is a Routing Index.
- `wiki/index.md` does not contain full all-page tables.
- Required shard indexes exist under `wiki/indexes/`.
- Every page in `wiki/pages/`, `entities/`, and `concepts/` appears in at least one appropriate shard index.
- Shard indexes do not link to missing pages.
- `wiki/indexes/hot.md` is not stale relative to `wiki/indexes/query-usage.jsonl`.

If index inconsistencies are found, recommend:

```bash
openwiki index rebuild
```
```

- [ ] **Step 6: Run static checks**

Run:

```bash
! grep -R "WIKI.md.*canonical\|canonical.*WIKI.md" skill/wiki-init skill/wiki-ingest skill/wiki-query skill/wiki-update skill/wiki-lint
grep -R "openwiki.toml" skill/wiki-init skill/wiki-ingest skill/wiki-query skill/wiki-update skill/wiki-lint
grep -R "wiki/indexes" skill/wiki-init skill/wiki-ingest skill/wiki-query skill/wiki-update skill/wiki-lint
grep -R "query-usage.jsonl" skill/wiki-query
```

Expected:

- First command exits 0 because no canonical `WIKI.md` wording remains.
- Other commands print matching lines.

- [ ] **Step 7: Commit**

```bash
git add skill/wiki-init/SKILL.md skill/wiki-ingest/SKILL.md skill/wiki-query/SKILL.md skill/wiki-update/SKILL.md skill/wiki-lint/SKILL.md
git commit -m "feat: 更新技能分层索引协议"
```

---

### Task 7: Update README docs for the new runtime contract

**Files:**
- Modify: `README.md`
- Modify: `README.en.md`
- Modify: `README.ja.md`
- Test: grep commands

- [ ] **Step 1: Update Chinese README runtime sections**

In `README.md`, replace the old runtime model section with:

```markdown
## 运行模型

OpenWiki 使用项目本地运行模型：

```text
<project>/
├── openwiki.toml            # 唯一运行时契约
└── openwiki/                # 默认 wiki_root
    ├── raw/                 # 原始素材
    ├── wiki/
    │   ├── index.md         # 轻量 Routing Index
    │   ├── log.md           # 操作日志
    │   ├── pages/           # 普通知识页
    │   └── indexes/         # 分片索引
    │       ├── scopes.md
    │       ├── entities.md
    │       ├── concepts.md
    │       ├── tags.md
    │       ├── recent.md
    │       ├── hot.md
    │       └── query-usage.jsonl
    ├── entities/            # 实体页
    └── concepts/            # 概念、分析、查询沉淀
```

核心原则：

- `openwiki.toml` 是唯一运行时契约；
- 默认 `wiki_root` 是 `./openwiki/`；
- 页面文件是内容事实来源；
- `wiki/index.md` 是轻量 Routing Index，不列全量页面；
- `wiki/indexes/` 是可增长的分片索引层；
- AI skill 直接写 Markdown 文件；
- CLI 负责配置、校验、状态、索引检查和索引重建。
```

Also update quick start discovery text to:

```markdown
运行时查找规则：

1. `--config` / `-c` 显式指定
2. `OPENWIKI_CONFIG`
3. 从当前工作目录向上搜索 `openwiki.toml`
4. `~/.openwiki/openwiki.toml`
```

- [ ] **Step 2: Update English README with equivalent content**

In `README.en.md`, add equivalent wording:

```markdown
## Runtime Model

OpenWiki uses a project-local runtime model. `openwiki.toml` is the only canonical runtime contract, and the default `wiki_root` is `./openwiki/`.

`wiki/index.md` is a lightweight Routing Index. It does not list every page. Growing indexes live under `wiki/indexes/`.

AI skills write Markdown files directly. The CLI provides guardrails for config discovery, validation, status, index checks, and index rebuilds.
```

Add discovery order:

```markdown
Discovery order:

1. `--config` / `-c`
2. `OPENWIKI_CONFIG`
3. Search upward from the current working directory for `openwiki.toml`
4. `~/.openwiki/openwiki.toml`
```

- [ ] **Step 3: Update Japanese README with equivalent content**

In `README.ja.md`, add equivalent wording:

```markdown
## ランタイムモデル

OpenWiki はプロジェクトローカルのランタイムモデルを使用します。`openwiki.toml` が唯一の canonical runtime contract で、デフォルトの `wiki_root` は `./openwiki/` です。

`wiki/index.md` は軽量な Routing Index です。すべてのページを列挙しません。成長する索引は `wiki/indexes/` に保存します。

AI skill は Markdown ファイルを直接編集します。CLI は設定検出、検証、状態確認、索引チェック、索引再構築の guardrail を提供します。
```

Add discovery order:

```markdown
設定検出順序：

1. `--config` / `-c`
2. `OPENWIKI_CONFIG`
3. 現在の作業ディレクトリから上位へ `openwiki.toml` を検索
4. `~/.openwiki/openwiki.toml`
```

- [ ] **Step 4: Run documentation checks**

Run:

```bash
grep -n "openwiki.toml 是唯一运行时契约" README.md
grep -n "Routing Index" README.md README.en.md README.ja.md
grep -n "OPENWIKI_CONFIG" README.md README.en.md README.ja.md
```

Expected: matching lines in all README files.

- [ ] **Step 5: Commit**

```bash
git add README.md README.en.md README.ja.md
git commit -m "feat: 更新运行时和分层索引文档"
```

---

### Task 8: Add end-to-end coverage for init → index rebuild → index check

**Files:**
- Modify: `tests/e2e/init_test.go`
- Modify: `tests/e2e/harness/harness.go`
- Test: `./tests/e2e/...`

- [ ] **Step 1: Add E2E assertion helper**

In `tests/e2e/harness/harness.go`, add:

```go
func AssertLayeredIndexLayout(t testing.TB, wikiRoot string) {
	t.Helper()
	expected := []string{
		filepath.Join(wikiRoot, "wiki", "index.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "scopes.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "entities.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "concepts.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "tags.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "recent.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "hot.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "query-usage.jsonl"),
	}
	for _, path := range expected {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected layered index path %s: %v", path, err)
		}
	}
}
```

Ensure imports include:

```go
import (
	"os"
	"path/filepath"
	"testing"
)
```

Merge with existing imports.

- [ ] **Step 2: Add E2E test for index commands**

In `tests/e2e/init_test.go`, add:

```go
func TestInitCreatesLayeredIndexAndIndexCommandsPass(t *testing.T) {
	instance := harness.NewInstance(t)

	result := instance.RunOpenWiki(t, "init", "--non-interactive", "--json")
	if result.ExitCode != 0 {
		t.Fatalf("init failed: stdout=%s stderr=%s", result.Stdout, result.Stderr)
	}

	wikiRoot := filepath.Join(instance.WorkDir, "openwiki")
	harness.AssertLayeredIndexLayout(t, wikiRoot)

	check := instance.RunOpenWiki(t, "index", "check", "--json")
	if check.ExitCode != 0 {
		t.Fatalf("index check failed: stdout=%s stderr=%s", check.Stdout, check.Stderr)
	}
	if !strings.Contains(check.Stdout, `"success": true`) {
		t.Fatalf("expected index check success JSON, got %s", check.Stdout)
	}

	rebuild := instance.RunOpenWiki(t, "index", "rebuild", "--json")
	if rebuild.ExitCode != 0 {
		t.Fatalf("index rebuild failed: stdout=%s stderr=%s", rebuild.Stdout, rebuild.Stderr)
	}
	if !strings.Contains(rebuild.Stdout, `"success": true`) {
		t.Fatalf("expected index rebuild success JSON, got %s", rebuild.Stdout)
	}
}
```

Ensure imports include `path/filepath` and `strings`.

- [ ] **Step 3: Run E2E tests**

Run:

```bash
go test ./tests/e2e/... -count=1 -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add tests/e2e/init_test.go tests/e2e/harness/harness.go
git commit -m "feat: 添加分层索引端到端测试"
```

---

### Task 9: Full verification and final cleanup

**Files:**
- No planned file changes unless previous tasks reveal formatting issues.
- Test: entire repository.

- [ ] **Step 1: Format Go files**

Run:

```bash
gofmt -w internal/config/*.go internal/wiki/*.go internal/cli/*.go tests/e2e/**/*.go
```

Expected: command exits 0.

- [ ] **Step 2: Run full Go test suite**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 3: Run build**

Run:

```bash
go build ./...
```

Expected: PASS.

- [ ] **Step 4: Check skill/docs static assertions**

Run:

```bash
! grep -R "WIKI.md.*canonical\|canonical.*WIKI.md" README.md README.en.md README.ja.md skill/wiki-init skill/wiki-ingest skill/wiki-query skill/wiki-update skill/wiki-lint
grep -R "wiki/indexes" README.md skill/wiki-init skill/wiki-ingest skill/wiki-query skill/wiki-update skill/wiki-lint
grep -R "query-usage.jsonl" skill/wiki-query docs/superpowers/specs/2026-06-15-openwiki-file-first-layered-index-design.md
```

Expected: first command exits 0; other commands print matches.

- [ ] **Step 5: Inspect git status**

Run:

```bash
git status --short
```

Expected: only intentional files are modified. If unrelated pre-existing changes remain, do not stage them.

- [ ] **Step 6: Final commit for formatting or integration fixes if needed**

If Step 1 changed files not committed in earlier tasks, commit them:

```bash
git add internal/config internal/wiki internal/cli tests/e2e README.md README.en.md README.ja.md skill/wiki-init skill/wiki-ingest skill/wiki-query skill/wiki-update skill/wiki-lint
git commit -m "feat: 完成分层索引基础改造"
```

If there are no new changes, skip this commit.

