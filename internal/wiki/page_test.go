package wiki_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/openwiki/internal/wiki"
)

func setupTestWiki(t *testing.T) (wiki.FS, string) {
	t.Helper()
	fs := wiki.NewMemFS()
	root := "/test-wiki"

	fs.MkdirAll(filepath.Join(root, "wiki", "pages"), 0755)

	indexContent := `# Wiki 索引

## 资料页

| Slug | 标题 | 类型 | 标签 | 适用范围 | 最后更新 |
|------|------|------|------|----------|----------|
| test-page | 测试页面 | page | test, demo | repo/test-repo | 2026-06-01 |
| another-page | 另一个页面 | page | demo | domain/test-domain | 2026-05-30 |
`
	fs.WriteFile(filepath.Join(root, "wiki", "index.md"), []byte(indexContent), 0644)

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

func TestListPages(t *testing.T) {
	fs, root := setupTestWiki(t)

	pages, err := wiki.ListPages(fs, root)
	if err != nil {
		t.Fatalf("ListPages failed: %v", err)
	}

	if len(pages) != 2 {
		t.Errorf("expected 2 pages, got %d", len(pages))
	}

	pageMap := make(map[string]wiki.PageMeta)
	for _, p := range pages {
		pageMap[p.Slug] = p
	}

	if p, ok := pageMap["test-page"]; !ok {
		t.Error("expected test-page in results")
	} else if p.Title != "测试页面" {
		t.Errorf("expected title=测试页面, got %s", p.Title)
	}

	if p, ok := pageMap["another-page"]; !ok {
		t.Error("expected another-page in results")
	} else if p.Title != "另一个页面" {
		t.Errorf("expected title=另一个页面, got %s", p.Title)
	}
}

func TestGetPage(t *testing.T) {
	fs, root := setupTestWiki(t)

	page, err := wiki.GetPage(fs, root, "test-page")
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}

	if page.Slug != "test-page" {
		t.Errorf("expected slug=test-page, got %s", page.Slug)
	}
	if page.Frontmatter["title"] != "测试页面" {
		t.Errorf("expected title=测试页面, got %v", page.Frontmatter["title"])
	}
	if len(page.CrossReferences) != 1 {
		t.Errorf("expected 1 cross reference, got %d", len(page.CrossReferences))
	}
	if page.CrossReferences[0] != "another-page" {
		t.Errorf("expected cross reference=another-page, got %s", page.CrossReferences[0])
	}
}

func TestGetPageNotFound(t *testing.T) {
	fs, root := setupTestWiki(t)

	_, err := wiki.GetPage(fs, root, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent page, got nil")
	}
}

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

	// 更新 index.md 添加 entity 条目
	indexContent := `# Wiki 索引

## 资料页

| Slug | 标题 | 类型 | 标签 | 适用范围 | 最后更新 |
|------|------|------|------|----------|----------|
| test-page | 测试页面 | page | test, demo | repo/test-repo | 2026-06-01 |
| another-page | 另一个页面 | page | demo | domain/test-domain | 2026-05-30 |
| andrej-karpathy | Andrej Karpathy | entity | entity, person | wisdom/ai-research | 2026-06-06 |
`
	fs.WriteFile(filepath.Join(root, "wiki", "index.md"), []byte(indexContent), 0644)

	page, err := wiki.GetPage(fs, root, "andrej-karpathy")
	if err != nil {
		t.Fatalf("GetPage from entities/ failed: %v", err)
	}
	if page.Slug != "andrej-karpathy" {
		t.Errorf("expected slug=andrej-karpathy, got %s", page.Slug)
	}
}

func TestGetPagePriorityPagesFirst(t *testing.T) {
	fs, root := setupTestWiki(t)

	// 在 wiki/pages/ 和 entities/ 同时创建同名页面
	fs.MkdirAll(filepath.Join(root, "entities"), 0755)
	fs.WriteFile(filepath.Join(root, "entities", "duplicate.md"), []byte("entity version"), 0644)

	// 更新 index.md
	indexContent := `# Wiki 索引

## 资料页

| Slug | 标题 | 类型 | 标签 | 适用范围 | 最后更新 |
|------|------|------|------|----------|----------|
| test-page | 测试页面 | page | test, demo | repo/test-repo | 2026-06-01 |
| another-page | 另一个页面 | page | demo | domain/test-domain | 2026-05-30 |
| duplicate | 重复页面 | entity | test | repo/test | 2026-06-06 |
`
	fs.WriteFile(filepath.Join(root, "wiki", "index.md"), []byte(indexContent), 0644)

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

func TestGetPagesBatch(t *testing.T) {
	fs, root := setupTestWiki(t)

	pages, err := wiki.GetPages(fs, root, []string{"test-page", "another-page"})
	if err != nil {
		t.Fatalf("GetPages failed: %v", err)
	}

	if len(pages) != 2 {
		t.Errorf("expected 2 pages, got %d", len(pages))
	}
}

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

	pages, _ := wiki.ListPages(fs, root)
	found := false
	for _, p := range pages {
		if p.Slug == "new-page" {
			found = true
			break
		}
	}
	if !found {
		t.Error("new-page not found in index after create")
	}
}

func TestCreatePageAlreadyExists(t *testing.T) {
	fs, root := setupTestWiki(t)

	page := &wiki.Page{
		Slug: "test-page",
		Frontmatter: map[string]interface{}{
			"title": "重复",
		},
		Body: "重复内容",
	}

	err := wiki.CreatePage(fs, root, page)
	if err == nil {
		t.Fatal("expected error for duplicate page, got nil")
	}
}

func TestCreateEntityPage(t *testing.T) {
	fs, root := setupTestWiki(t)

	page := &wiki.Page{
		Slug: "andrej-karpathy",
		Frontmatter: map[string]interface{}{
			"title":       "Andrej Karpathy",
			"entity_type": "person",
			"tags":        []string{"entity", "person"},
			"scope_level": "wisdom",
			"scope_code":  "ai-research",
			"updated":     "2026-06-06",
		},
		Body: "# Andrej Karpathy\n\n核心身份：AI 研究员。",
	}

	err := wiki.CreatePage(fs, root, page, wiki.PageTypeEntity)
	if err != nil {
		t.Fatalf("CreatePage with PageTypeEntity failed: %v", err)
	}

	// 验证文件路径在 entities/ 目录下
	expectedPath := filepath.Join(root, "entities", "andrej-karpathy.md")
	if _, err := fs.Stat(expectedPath); err != nil {
		t.Errorf("expected entity page at %s, got error: %v", expectedPath, err)
	}

	// 验证不会写入 wiki/pages/ 目录
	wrongPath := filepath.Join(root, "wiki", "pages", "andrej-karpathy.md")
	if _, err := fs.Stat(wrongPath); err == nil {
		t.Errorf("entity page should NOT be at %s", wrongPath)
	}
}

func TestCreateConceptPage(t *testing.T) {
	fs, root := setupTestWiki(t)

	page := &wiki.Page{
		Slug: "transformer-architecture",
		Frontmatter: map[string]interface{}{
			"title":       "Transformer Architecture",
			"tags":        []string{"concept", "deep-learning"},
			"scope_level": "wisdom",
			"scope_code":  "ai-research",
			"updated":     "2026-06-06",
		},
		Body: "# Transformer Architecture\n\n核心概念。",
	}

	err := wiki.CreatePage(fs, root, page, wiki.PageTypeConcept)
	if err != nil {
		t.Fatalf("CreatePage with PageTypeConcept failed: %v", err)
	}

	expectedPath := filepath.Join(root, "concepts", "transformer-architecture.md")
	if _, err := fs.Stat(expectedPath); err != nil {
		t.Errorf("expected concept page at %s, got error: %v", expectedPath, err)
	}
}

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

func TestUpdatePage(t *testing.T) {
	fs, root := setupTestWiki(t)

	page := &wiki.Page{
		Slug: "test-page",
		Frontmatter: map[string]interface{}{
			"title":       "更新后的标题",
			"tags":        []string{"updated"},
			"scope_level": "repo",
			"scope_code":  "test-repo",
			"updated":     "2026-06-02",
		},
		Body: "# 更新后的内容",
	}

	err := wiki.UpdatePage(fs, root, page)
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	updated, err := wiki.GetPage(fs, root, "test-page")
	if err != nil {
		t.Fatalf("GetPage after update failed: %v", err)
	}
	if updated.Frontmatter["title"] != "更新后的标题" {
		t.Errorf("expected title=更新后的标题, got %v", updated.Frontmatter["title"])
	}
	if updated.Body != "# 更新后的内容" {
		t.Errorf("expected body=# 更新后的内容, got %s", updated.Body)
	}
}

func TestUpdatePageNotFound(t *testing.T) {
	fs, root := setupTestWiki(t)

	page := &wiki.Page{
		Slug: "nonexistent",
		Body: "content",
	}

	err := wiki.UpdatePage(fs, root, page)
	if err == nil {
		t.Fatal("expected error for nonexistent page, got nil")
	}
}

func TestUpdatePagePreserveType(t *testing.T) {
	fs, root := setupTestWiki(t)

	// 创建 entity 页面
	page := &wiki.Page{
		Slug: "entity-to-update",
		Frontmatter: map[string]interface{}{
			"title":       "Entity To Update",
			"entity_type": "person",
			"tags":        []string{"entity"},
			"scope_level": "repo",
			"scope_code":  "test",
			"updated":     "2026-06-06",
		},
		Body: "original content",
	}
	if err := wiki.CreatePage(fs, root, page, wiki.PageTypeEntity); err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// 更新（不传 newType，应保持 entity 类型）
	page.Frontmatter["title"] = "Updated Entity"
	page.Body = "updated content"
	if err := wiki.UpdatePage(fs, root, page); err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	// 验证文件仍在 entities/ 目录
	entityPath := filepath.Join(root, "entities", "entity-to-update.md")
	if _, err := fs.Stat(entityPath); err != nil {
		t.Errorf("entity page should still be at %s, got error: %v", entityPath, err)
	}

	// 验证内容已更新
	updated, err := wiki.GetPage(fs, root, "entity-to-update")
	if err != nil {
		t.Fatalf("GetPage after update failed: %v", err)
	}
	if updated.Frontmatter["title"] != "Updated Entity" {
		t.Errorf("expected title=Updated Entity, got %v", updated.Frontmatter["title"])
	}
}

func TestUpdatePageChangeType(t *testing.T) {
	fs, root := setupTestWiki(t)

	// 创建 entity 页面
	page := &wiki.Page{
		Slug: "entity-to-migrate",
		Frontmatter: map[string]interface{}{
			"title":       "Entity To Migrate",
			"entity_type": "tool",
			"tags":        []string{"entity"},
			"scope_level": "repo",
			"scope_code":  "test",
			"updated":     "2026-06-06",
		},
		Body: "original content",
	}
	if err := wiki.CreatePage(fs, root, page, wiki.PageTypeEntity); err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	// 变更类型为 concept
	page.Frontmatter["title"] = "Migrated Concept"
	page.Body = "migrated content"
	if err := wiki.UpdatePage(fs, root, page, wiki.PageTypeConcept); err != nil {
		t.Fatalf("UpdatePage with type change failed: %v", err)
	}

	// 验证原 entity 文件已删除
	entityPath := filepath.Join(root, "entities", "entity-to-migrate.md")
	if _, err := fs.Stat(entityPath); err == nil {
		t.Error("original entity file should be deleted after type change")
	}

	// 验证新 concept 文件存在
	conceptPath := filepath.Join(root, "concepts", "entity-to-migrate.md")
	if _, err := fs.Stat(conceptPath); err != nil {
		t.Errorf("migrated concept page should be at %s, got error: %v", conceptPath, err)
	}

	// 验证内容
	updated, err := wiki.GetPage(fs, root, "entity-to-migrate")
	if err != nil {
		t.Fatalf("GetPage after migration failed: %v", err)
	}
	if updated.Frontmatter["title"] != "Migrated Concept" {
		t.Errorf("expected title=Migrated Concept, got %v", updated.Frontmatter["title"])
	}
}

func TestDeletePage(t *testing.T) {
	fs, root := setupTestWiki(t)

	err := wiki.DeletePage(fs, root, "test-page")
	if err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}

	_, err = wiki.GetPage(fs, root, "test-page")
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}

	pages, _ := wiki.ListPages(fs, root)
	for _, p := range pages {
		if p.Slug == "test-page" {
			t.Error("test-page should not be in index after delete")
		}
	}
}

func TestDeletePageNotFound(t *testing.T) {
	fs, root := setupTestWiki(t)

	err := wiki.DeletePage(fs, root, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent page, got nil")
	}
}
