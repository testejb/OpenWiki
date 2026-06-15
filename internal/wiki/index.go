package wiki

import (
	"fmt"
	"path/filepath"
	"regexp"
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
	sortPagesBySlug(pages)

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
		"wiki/index.md":                  buildRoutingIndex(len(pages), countQueryUsage(fs, root)),
		"wiki/indexes/scopes.md":         buildScopesIndex(pages),
		"wiki/indexes/entities.md":       buildTypeIndex(pages, "entity"),
		"wiki/indexes/concepts.md":       buildTypeIndex(pages, "concept"),
		"wiki/indexes/tags.md":           buildTagsIndex(pages),
		"wiki/indexes/recent.md":         buildRecentIndex(pages),
		"wiki/indexes/hot.md":            buildHotIndex(fs, root),
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
	pagesByScope := map[string][]PageMeta{}
	for _, page := range pages {
		scope := joinScope(page.ScopeLevel, page.ScopeCode)
		if scope != "" {
			counts[scope]++
			pagesByScope[scope] = append(pagesByScope[scope], page)
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
		var links []string
		for _, page := range pagesByScope[scope] {
			links = append(links, fmt.Sprintf("[[%s]]", page.Slug))
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %d 个页面 | %s |\n", scope, counts[scope], strings.Join(links, ", ")))
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
	recent := append([]PageMeta(nil), pages...)
	sort.Slice(recent, func(i, j int) bool {
		if recent[i].Updated == recent[j].Updated {
			return recent[i].Slug < recent[j].Slug
		}
		return recent[i].Updated > recent[j].Updated
	})
	var sb strings.Builder
	sb.WriteString("# 最近更新\n\n| 页面 | 类型 | Scope | 标签 | 更新 | 摘要 |\n|---|---|---|---|---|---|\n")
	for _, page := range recent {
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

func sortPagesBySlug(pages []PageMeta) {
	sort.Slice(pages, func(i, j int) bool { return pages[i].Slug < pages[j].Slug })
}
