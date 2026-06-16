# Indexes Bidirectional Links Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make generated `wiki/index.md` refer to shard index pages with bidirectional links like `[[indexes/scopes]]`.

**Architecture:** The top-level routing index text is generated in two places: the init template in `internal/wiki/init.go` and the rebuild template in `internal/wiki/index.go`. Update only those templates and add focused tests around their rendered output.

**Tech Stack:** Go, standard `testing` package, in-memory wiki filesystem test helpers.

---

## File Structure

- Modify `internal/wiki/init_test.go`: strengthen `TestInitCreatesLayeredIndexLayout` to assert `openwiki init` output uses `[[indexes/<name>]]` links and omits old ``indexes/*.md`` inline-code references.
- Modify `internal/wiki/init.go`: update `indexTemplate` routing list and shard index table to use bidirectional links.
- Modify `internal/wiki/index_test.go`: add a focused `TestRebuildIndexUsesBidirectionalLinksForShardIndexes` test for `RebuildIndex` output.
- Modify `internal/wiki/index.go`: update `buildRoutingIndex` routing list and shard index table to use bidirectional links.

---

### Task 1: Update init template using TDD

**Files:**
- Modify: `internal/wiki/init_test.go`
- Modify: `internal/wiki/init.go`

- [ ] **Step 1: Write the failing init test assertions**

In `internal/wiki/init_test.go`, inside `TestInitCreatesLayeredIndexLayout`, replace the two old assertions:

```go
	if !strings.Contains(index, "indexes/scopes.md") {
		t.Fatalf("expected routing index to mention indexes/scopes.md, got:\n%s", index)
	}
	if !strings.Contains(index, "indexes/hot.md") {
		t.Fatalf("expected routing index to mention indexes/hot.md, got:\n%s", index)
	}
```

with:

```go
	assertShardIndexLinksUseWikiLinks(t, index)
```

Then add this helper at the end of `internal/wiki/init_test.go`:

```go
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
```

- [ ] **Step 2: Run the init test and verify it fails**

Run:

```bash
go test ./internal/wiki -run TestInitCreatesLayeredIndexLayout -count=1
```

Expected: FAIL because `[[indexes/scopes]]` is missing from the current init template.

- [ ] **Step 3: Update `indexTemplate` in `internal/wiki/init.go`**

In `internal/wiki/init.go`, update only the shard references inside `const indexTemplate`.

Replace the routing list block with:

```go
1. Scope 线索：项目、仓库、模块、领域 → [[indexes/scopes]]
2. Entity 线索：人、组织、项目、工具 → [[indexes/entities]]
3. Concept 线索：设计、原则、决策、分析 → [[indexes/concepts]]
4. Tag 线索：关键词、主题标签 → [[indexes/tags]]
5. Recency 线索：当前、最近、最新状态 → [[indexes/recent]]
6. 不确定时：先读 [[indexes/hot]] 和 [[indexes/recent]]
```

Replace the shard index table rows with:

```go
| [[indexes/scopes]] | 按 scope 分组的入口 | 问题含项目/模块/领域线索 |
| [[indexes/entities]] | 实体页入口 | 问题涉及具体人/组织/项目/工具 |
| [[indexes/concepts]] | 概念与分析入口 | 问题涉及设计原则/决策背景 |
| [[indexes/tags]] | 标签入口 | 问题含关键词但 scope 不明确 |
| [[indexes/recent]] | 最近更新 | 问题问当前状态或最新变化 |
| [[indexes/hot]] | 查询热度入口 | 不确定或常见问题优先 |
```

Do not change the actual generated files under `wiki/indexes/`.

- [ ] **Step 4: Run the init test and verify it passes**

Run:

```bash
go test ./internal/wiki -run TestInitCreatesLayeredIndexLayout -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 1**

Run:

```bash
git add internal/wiki/init.go internal/wiki/init_test.go
git commit -m "feat: 初始化索引使用双向链接"
```

---

### Task 2: Update rebuild template using TDD

**Files:**
- Modify: `internal/wiki/index_test.go`
- Modify: `internal/wiki/index.go`

- [ ] **Step 1: Write the failing rebuild test**

In `internal/wiki/index_test.go`, add this test after `TestRebuildIndexIndexesExistingPages`:

```go
func TestRebuildIndexUsesBidirectionalLinksForShardIndexes(t *testing.T) {
	fs := wiki.NewMemFS()
	root := "/test-wiki"
	if err := wiki.Init(fs, root, map[string]interface{}{"wiki_root": root}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := wiki.RebuildIndex(fs, root); err != nil {
		t.Fatalf("RebuildIndex returned error: %v", err)
	}

	data, err := fs.ReadFile(filepath.Join(root, "wiki", "index.md"))
	if err != nil {
		t.Fatalf("read routing index failed: %v", err)
	}
	index := string(data)

	for _, link := range []string{
		"[[indexes/scopes]]",
		"[[indexes/entities]]",
		"[[indexes/concepts]]",
		"[[indexes/tags]]",
		"[[indexes/recent]]",
		"[[indexes/hot]]",
	} {
		if !strings.Contains(index, link) {
			t.Fatalf("expected rebuilt routing index to contain shard wiki link %s, got:\n%s", link, index)
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
			t.Fatalf("expected rebuilt routing index not to contain old shard path reference %s, got:\n%s", oldRef, index)
		}
	}
}
```

- [ ] **Step 2: Run the rebuild test and verify it fails**

Run:

```bash
go test ./internal/wiki -run TestRebuildIndexUsesBidirectionalLinksForShardIndexes -count=1
```

Expected: FAIL because `buildRoutingIndex` still emits old ``indexes/*.md`` references.

- [ ] **Step 3: Update `buildRoutingIndex` in `internal/wiki/index.go`**

In `internal/wiki/index.go`, update only the shard references inside `buildRoutingIndex`.

Replace the routing list block with:

```go
1. Scope 线索：项目、仓库、模块、领域 → [[indexes/scopes]]
2. Entity 线索：人、组织、项目、工具 → [[indexes/entities]]
3. Concept 线索：设计、原则、决策、分析 → [[indexes/concepts]]
4. Tag 线索：关键词、主题标签 → [[indexes/tags]]
5. Recency 线索：当前、最近、最新状态 → [[indexes/recent]]
6. 不确定时：先读 [[indexes/hot]] 和 [[indexes/recent]]
```

Replace the shard index table rows with:

```go
| [[indexes/scopes]] | 按 scope 分组的入口 | 问题含项目/模块/领域线索 |
| [[indexes/entities]] | 实体页入口 | 问题涉及具体人/组织/项目/工具 |
| [[indexes/concepts]] | 概念与分析入口 | 问题涉及设计原则/决策背景 |
| [[indexes/tags]] | 标签入口 | 问题含关键词但 scope 不明确 |
| [[indexes/recent]] | 最近更新 | 问题问当前状态或最新变化 |
| [[indexes/hot]] | 查询热度入口 | 不确定或常见问题优先 |
```

Keep `time.Now().Format("2006-01-02")`, `pageCount`, and `usageCount` formatting unchanged.

- [ ] **Step 4: Run the rebuild test and verify it passes**

Run:

```bash
go test ./internal/wiki -run TestRebuildIndexUsesBidirectionalLinksForShardIndexes -count=1
```

Expected: PASS.

- [ ] **Step 5: Run all wiki package tests**

Run:

```bash
go test ./internal/wiki -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 2**

Run:

```bash
git add internal/wiki/index.go internal/wiki/index_test.go
git commit -m "feat: 重建索引使用双向链接"
```

---

### Task 3: Final verification

**Files:**
- No source changes expected.

- [ ] **Step 1: Run the full Go test suite**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 2: Check git status**

Run:

```bash
git status --short
```

Expected: clean except for any pre-existing user-owned artifacts explicitly noted before implementation. Do not stage or overwrite unrelated files.

- [ ] **Step 3: Report changed files and verification evidence**

Report these items:

- `internal/wiki/init.go`
- `internal/wiki/init_test.go`
- `internal/wiki/index.go`
- `internal/wiki/index_test.go`
- verification command output summary from `go test ./... -count=1`

