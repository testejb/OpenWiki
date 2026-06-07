# ListPages 改为文件系统扫描 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `wiki.ListPages` 从解析 `index.md` 表格改为扫描文件系统目录，读取 `.md` 文件的 frontmatter 提取元信息。

**Architecture:** 修改 `internal/wiki/page.go` 中的 `ListPages` 函数，遍历 `pageDirs` 中定义的三个目录，对每个 `.md` 文件读取并解析 frontmatter。函数签名不变，调用方无需修改。

**Tech Stack:** Go

---

### Task 1: 重写 `ListPages` 为文件系统扫描

**Files:**
- Modify: `internal/wiki/page.go:49-57`

- [ ] **Step 1: 替换 `ListPages` 实现**

将 `internal/wiki/page.go` 中的 `ListPages` 函数（第 49-57 行）替换为：

```go
func ListPages(fs FS, root string) ([]PageMeta, error) {
	var pages []PageMeta

	for pt, dir := range pageDirs {
		fullDir := filepath.Join(root, dir)
		entries, err := fs.ReadDir(fullDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			slug := strings.TrimSuffix(entry.Name(), ".md")
			pagePath := filepath.Join(fullDir, entry.Name())

			data, err := fs.ReadFile(pagePath)
			if err != nil {
				continue
			}

			meta := extractPageMeta(slug, string(pt), string(data))
			pages = append(pages, meta)
		}
	}

	return pages, nil
}
```

- [ ] **Step 2: 新增 `extractPageMeta` 辅助函数**

在 `page.go` 中 `ListPages` 之后新增：

```go
func extractPageMeta(slug, pageType, content string) PageMeta {
	meta := PageMeta{
		Slug: slug,
		Type: pageType,
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return meta
	}

	var fm map[string]interface{}
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(parts[1])), &fm); err != nil {
		return meta
	}

	if t, ok := fm["title"].(string); ok {
		meta.Title = t
	}
	if t, ok := fm["tags"].([]interface{}); ok {
		for _, tag := range t {
			if s, ok := tag.(string); ok {
				meta.Tags = append(meta.Tags, s)
			}
		}
	}
	if s, ok := fm["scope_level"].(string); ok {
		meta.ScopeLevel = s
	}
	if s, ok := fm["scope_code"].(string); ok {
		meta.ScopeCode = s
	}
	if s, ok := fm["updated"].(string); ok {
		meta.Updated = s
	}

	return meta
}
```

- [ ] **Step 3: 删除 `parseIndexTable` 函数**

删除 `page.go` 中第 59-141 行的 `parseIndexTable` 函数（不再被使用）。

- [ ] **Step 4: 编译验证**

```bash
cd /Users/bytedance/git/OpenWiki && go build ./...
```

预期: 编译成功

- [ ] **Step 5: 提交**

```bash
git add internal/wiki/page.go
git commit -m "feat: ListPages 改为扫描文件系统目录代替解析 index.md 表格"
```

---

### Task 2: 更新 `wiki/page_test.go` 中的测试

**Files:**
- Modify: `internal/wiki/page_test.go`

- [ ] **Step 1: 更新 `setupTestWiki`**

将 `setupTestWiki`（第 11-58 行）改为只创建目录和 `.md` 文件，不再写入 index.md：

```go
func setupTestWiki(t *testing.T) (wiki.FS, string) {
	t.Helper()
	fs := wiki.NewMemFS()
	root := "/test-wiki"

	fs.MkdirAll(filepath.Join(root, "wiki", "pages"), 0755)

	pageContent := `---
title: 测试页面
tags: [test, demo]
scope_level: repo
scope_code: test-repo
updated: 2026-06-01
---

# 测试页面

这是测试内容，引用 [[another-page]]。
`
	fs.WriteFile(filepath.Join(root, "wiki", "pages", "test-page.md"), []byte(pageContent), 0644)

	page2Content := `---
title: 另一个页面
tags: [demo]
scope_level: domain
scope_code: test-domain
updated: 2026-05-30
---

# 另一个页面

另一个页面的内容。
`
	fs.WriteFile(filepath.Join(root, "wiki", "pages", "another-page.md"), []byte(page2Content), 0644)

	return fs, root
}
```

- [ ] **Step 2: 更新 `TestListPages`**

`TestListPages`（第 60-78 行）保持不变，因为 `ListPages` 签名不变，且 `setupTestWiki` 依然创建了 2 个 `.md` 文件。

- [ ] **Step 3: 更新 `TestGetPageFromEntitiesDir`**

`TestGetPageFromEntitiesDir`（第 111-151 行）需要去掉 index.md 写入部分，只保留 entities 目录和文件创建：

```go
func TestGetPageFromEntitiesDir(t *testing.T) {
	fs, root := setupTestWiki(t)

	// 在 entities/ 目录下创建页面
	entityContent := `---
title: Andrej Karpathy
entity_type: person
tags: [entity, person]
scope_level: wisdom
scope_code: ai-research
updated: 2026-06-06
---

# Andrej Karpathy

AI 研究员。
`
	fs.MkdirAll(filepath.Join(root, "entities"), 0755)
	fs.WriteFile(filepath.Join(root, "entities", "andrej-karpathy.md"), []byte(entityContent), 0644)

	page, err := wiki.GetPage(fs, root, "andrej-karpathy")
	if err != nil {
		t.Fatalf("GetPage from entities/ failed: %v", err)
	}
	if page.Slug != "andrej-karpathy" {
		t.Errorf("expected slug=andrej-karpathy, got %s", page.Slug)
	}
}
```

- [ ] **Step 4: 更新 `TestGetPagePriorityPagesFirst`**

`TestGetPagePriorityPagesFirst`（第 153-184 行）需要去掉 index.md 写入部分：

```go
func TestGetPagePriorityPagesFirst(t *testing.T) {
	fs, root := setupTestWiki(t)

	// 在 wiki/pages/ 和 entities/ 同时创建同名页面
	fs.MkdirAll(filepath.Join(root, "entities"), 0755)
	fs.WriteFile(filepath.Join(root, "entities", "duplicate.md"), []byte("entity version"), 0644)

	// 同时在 pages 下创建
	fs.WriteFile(filepath.Join(root, "wiki", "pages", "duplicate.md"), []byte("page version"), 0644)

	page, err := wiki.GetPage(fs, root, "duplicate")
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}
	// 应该返回 wiki/pages/ 下的（优先级最高）
	if !strings.Contains(page.Path, "wiki/pages") {
		t.Errorf("expected page from wiki/pages/, got path: %s", page.Path)
	}
}
```

- [ ] **Step 5: 更新 `TestCreatePage`**

`TestCreatePage`（第 199-238 行）的最后部分（第 227-237 行）验证 `ListPages` 返回的页面中是否包含新页面。由于 `CreatePage` 会调用 `addToIndex`（写入 index.md），但 `ListPages` 现在不读 index.md，所以需要改为验证文件是否存在：

```go
func TestCreatePage(t *testing.T) {
	fs, root := setupTestWiki(t)

	page := &wiki.Page{
		Slug: "new-page",
		Frontmatter: map[string]interface{}{
			"title":       "新页面",
			"tags":        []string{"new"},
			"scope_level": "repo",
			"scope_code":  "new-repo",
			"updated":     "2026-06-01",
		},
		Body: "# 新页面\n\n这是新页面的内容。",
	}

	err := wiki.CreatePage(fs, root, page)
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	created, err := wiki.GetPage(fs, root, "new-page")
	if err != nil {
		t.Fatalf("GetPage after create failed: %v", err)
	}
	if created.Slug != "new-page" {
		t.Errorf("expected slug=new-page, got %s", created.Slug)
	}

	// 验证文件存在
	pagePath := filepath.Join(root, "wiki", "pages", "new-page.md")
	if _, err := fs.Stat(pagePath); err != nil {
		t.Error("new-page file not found after create")
	}
}
```

- [ ] **Step 6: 更新 `TestListPagesWithType`**

`TestListPagesWithType`（第 317-369 行）依赖 `CreatePage` 内部调用 `addToIndex` 写入 index.md，然后 `ListPages` 读 index.md 获取 type。现在 `ListPages` 改用文件系统扫描，每个文件所在目录就决定了 type。测试逻辑基本不变，但需要确认 `setupTestWiki` 的初始页面来自 `wiki/pages/` 目录（type=page），entity 和 concept 来自各自目录。

更新后的测试：

```go
func TestListPagesWithType(t *testing.T) {
	fs, root := setupTestWiki(t)

	// 创建三种类型的页面
	createPage := func(slug, title, pageType string) {
		page := &wiki.Page{
			Slug: slug,
			Frontmatter: map[string]interface{}{
				"title":       title,
				"tags":        []string{"test"},
				"scope_level": "repo",
				"scope_code":  "test",
				"updated":     "2026-06-06",
			},
			Body: "content",
		}
		var pt wiki.PageType
		switch pageType {
		case "entity":
			pt = wiki.PageTypeEntity
		case "concept":
			pt = wiki.PageTypeConcept
		default:
			pt = wiki.PageTypePage
		}
		if err := wiki.CreatePage(fs, root, page, pt); err != nil {
			t.Fatalf("CreatePage %s failed: %v", slug, err)
		}
	}

	createPage("entity-test", "Entity Test", "entity")
	createPage("concept-test", "Concept Test", "concept")

	pages, err := wiki.ListPages(fs, root)
	if err != nil {
		t.Fatalf("ListPages failed: %v", err)
	}

	typeMap := make(map[string]string)
	for _, p := range pages {
		typeMap[p.Slug] = p.Type
	}

	if typeMap["test-page"] != "page" {
		t.Errorf("expected test-page type=page, got %s", typeMap["test-page"])
	}
	if typeMap["entity-test"] != "entity" {
		t.Errorf("expected entity-test type=entity, got %s", typeMap["entity-test"])
	}
	if typeMap["concept-test"] != "concept" {
		t.Errorf("expected concept-test type=concept, got %s", typeMap["concept-test"])
	}
}
```

- [ ] **Step 7: 更新 `TestDeleteEntityPage`**

`TestDeleteEntityPage`（第 371-409 行）的最后部分（第 403-408 行）验证 `ListPages` 中不再包含已删除的页面。改为验证文件不存在：

```go
func TestDeleteEntityPage(t *testing.T) {
	fs, root := setupTestWiki(t)

	// 创建 entity 页面
	page := &wiki.Page{
		Slug: "to-delete",
		Frontmatter: map[string]interface{}{
			"title":       "To Delete",
			"entity_type": "tool",
			"tags":        []string{"entity"},
			"scope_level": "repo",
			"scope_code":  "test",
			"updated":     "2026-06-06",
		},
		Body: "content",
	}
	if err := wiki.CreatePage(fs, root, page, wiki.PageTypeEntity); err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// 删除
	if err := wiki.DeletePage(fs, root, "to-delete"); err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}

	// 验证文件已删除
	pagePath := filepath.Join(root, "entities", "to-delete.md")
	if _, err := fs.Stat(pagePath); err == nil {
		t.Error("expected entity page to be deleted")
	}

	// 验证 index 中已移除
	pages, _ := wiki.ListPages(fs, root)
	for _, p := range pages {
		if p.Slug == "to-delete" {
			t.Error("to-delete should not be in index after delete")
		}
	}
}
```

- [ ] **Step 8: 更新 `TestDeletePage`**

`TestDeletePage`（第 549-568 行）的最后部分（第 562-567 行）同样验证 `ListPages`。保持不变即可，因为 `DeletePage` 会删除文件，`ListPages` 扫描文件系统自然找不到。

- [ ] **Step 9: 新增 `TestListPagesEmptyDir`**

在 `page_test.go` 末尾新增：

```go
func TestListPagesEmptyDir(t *testing.T) {
	fs := wiki.NewMemFS()
	root := "/empty-wiki"

	pages, err := wiki.ListPages(fs, root)
	if err != nil {
		t.Fatalf("ListPages should not error on empty dir: %v", err)
	}
	if len(pages) != 0 {
		t.Errorf("expected 0 pages, got %d", len(pages))
	}
}
```

- [ ] **Step 10: 新增 `TestListPagesSkipsBadFiles`**

在 `page_test.go` 末尾新增：

```go
func TestListPagesSkipsBadFiles(t *testing.T) {
	fs := wiki.NewMemFS()
	root := "/test-wiki"

	fs.MkdirAll(filepath.Join(root, "wiki", "pages"), 0755)

	// 正常文件
	goodContent := `---
title: 正常页面
tags: [test]
scope_level: repo
scope_code: test
updated: 2026-06-06
---

内容
`
	fs.WriteFile(filepath.Join(root, "wiki", "pages", "good.md"), []byte(goodContent), 0644)

	// 损坏的 YAML frontmatter
	badContent := `---
title: [broken yaml
tags: 
---
`
	fs.WriteFile(filepath.Join(root, "wiki", "pages", "bad.md"), []byte(badContent), 0644)

	pages, err := wiki.ListPages(fs, root)
	if err != nil {
		t.Fatalf("ListPages failed: %v", err)
	}

	if len(pages) != 1 {
		t.Errorf("expected 1 page (good), got %d", len(pages))
	}
	if pages[0].Slug != "good" {
		t.Errorf("expected slug=good, got %s", pages[0].Slug)
	}
}
```

- [ ] **Step 11: 运行 wiki 包测试**

```bash
cd /Users/bytedance/git/OpenWiki && go test ./internal/wiki/ -v -count=1
```

预期: 全部 PASS

- [ ] **Step 12: 提交**

```bash
git add internal/wiki/page_test.go
git commit -m "feat: 更新 wiki 包测试适配 ListPages 文件系统扫描"
```

---

### Task 3: 更新 `cli/status_test.go` 中的测试

**Files:**
- Modify: `internal/cli/status_test.go`

- [ ] **Step 1: 更新 `setupTestWiki`**

`setupTestWiki`（第 14-78 行）需要去掉 index.md 写入部分（第 63-71 行），因为 `ListPages` 不再读 index.md 了：

```go
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

	tomlPath := filepath.Join(dir, "openwiki.toml")

	pageDir := filepath.Join(wikiRoot, "wiki", "pages")
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
```

- [ ] **Step 2: 运行 CLI 测试**

```bash
cd /Users/bytedance/git/OpenWiki && go test ./internal/cli/ -v -run "TestStatus" -count=1
```

预期: 全部 PASS

- [ ] **Step 3: 提交**

```bash
git add internal/cli/status_test.go
git commit -m "feat: 更新 status 测试移除 index.md 依赖"
```

---

### Task 4: 运行全部测试并验证

- [ ] **Step 1: 运行全部测试**

```bash
cd /Users/bytedance/git/OpenWiki && go test ./... -count=1
```

预期: 全部 PASS

- [ ] **Step 2: 验证 `~/.openwiki` 下 status 输出**

```bash
cd ~/.openwiki && /Users/bytedance/git/OpenWiki/bin/openwiki status
```

预期: 页面总数 > 0（应显示 `/Users/bytedance/wiki/wiki/pages/` 下的 75 个页面）

- [ ] **Step 3: 构建和提交**

```bash
cd /Users/bytedance/git/OpenWiki && go build -o bin/openwiki .
git add bin/openwiki
git commit -m "feat: 构建更新后的 openwiki 二进制文件"
```