package wiki

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// PageType 页面类型
type PageType string

const (
	PageTypePage    PageType = "page"
	PageTypeEntity  PageType = "entity"
	PageTypeConcept PageType = "concept"
)

// pageDirs 每种类型对应的存储目录（相对于 wiki_root）
var pageDirs = map[PageType]string{
	PageTypePage:    "wiki/pages",
	PageTypeEntity:  "entities",
	PageTypeConcept: "concepts",
}

// searchOrder 跨目录搜索的优先级
var searchOrder = []PageType{PageTypePage, PageTypeEntity, PageTypeConcept}

type PageMeta struct {
	Slug       string   `json:"slug"`
	Title      string   `json:"title"`
	Type       string   `json:"type"`
	Tags       []string `json:"tags"`
	ScopeLevel string   `json:"scope_level"`
	ScopeCode  string   `json:"scope_code"`
	Updated    string   `json:"updated"`
}

type Page struct {
	Slug            string                 `json:"slug"`
	Path            string                 `json:"path"`
	Frontmatter     map[string]interface{} `json:"frontmatter"`
	Body            string                 `json:"body"`
	CrossReferences []string               `json:"cross_references"`
}

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

func GetPage(fs FS, root, slug string) (*Page, error) {
	pagePath, _, err := resolvePagePath(fs, root, slug)
	if err != nil {
		return nil, err
	}

	data, err := fs.ReadFile(pagePath)
	if err != nil {
		return nil, fmt.Errorf("读取页面失败 %s: %w", slug, err)
	}

	page, err := parsePage(slug, pagePath, string(data))
	if err != nil {
		return nil, fmt.Errorf("解析页面失败 %s: %w", slug, err)
	}
	return page, nil
}

func GetPages(fs FS, root string, slugs []string) ([]*Page, error) {
	var pages []*Page
	for _, slug := range slugs {
		page, err := GetPage(fs, root, slug)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

// resolvePagePath 按 searchOrder 查找页面文件，返回路径和类型
func resolvePagePath(fs FS, root, slug string) (string, PageType, error) {
	for _, pt := range searchOrder {
		dir := pageDirs[pt]
		pagePath := filepath.Join(root, dir, slug+".md")
		if _, err := fs.Stat(pagePath); err == nil {
			return pagePath, pt, nil
		}
	}
	return "", "", fmt.Errorf("页面不存在: %s", slug)
}

func parsePage(slug, path, content string) (*Page, error) {
	page := &Page{
		Slug: slug,
		Path: path,
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) >= 3 {
		fmData := strings.TrimSpace(parts[1])
		var fm map[string]interface{}
		if err := yaml.Unmarshal([]byte(fmData), &fm); err == nil {
			page.Frontmatter = fm
		}
		page.Body = strings.TrimSpace(parts[2])
	} else {
		page.Body = strings.TrimSpace(content)
	}

	re := regexp.MustCompile(`\[\[([a-zA-Z0-9_-]+)\]\]`)
	matches := re.FindAllStringSubmatch(page.Body, -1)
	for _, m := range matches {
		page.CrossReferences = append(page.CrossReferences, m[1])
	}

	return page, nil
}

func ParsePageContent(slug, content string) (*Page, error) {
	return parsePage(slug, "", content)
}

func CreatePage(fs FS, root string, page *Page, pageType ...PageType) error {
	pt := PageTypePage
	if len(pageType) > 0 {
		pt = pageType[0]
	}

	dir := pageDirs[pt]
	pagePath := filepath.Join(root, dir, page.Slug+".md")
	if _, err := fs.Stat(pagePath); err == nil {
		return fmt.Errorf("页面已存在: %s", page.Slug)
	}

	// 确保目录存在
	if err := fs.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	content := buildPageContent(page)
	if err := fs.WriteFile(pagePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入页面失败: %w", err)
	}

	if _, err := RebuildIndex(fs, root); err != nil {
		return fmt.Errorf("重建索引失败: %w", err)
	}

	return nil
}

func buildPageContent(page *Page) string {
	var sb strings.Builder

	if page.Frontmatter != nil && len(page.Frontmatter) > 0 {
		fmData, err := yaml.Marshal(page.Frontmatter)
		if err == nil {
			sb.WriteString("---\n")
			sb.Write(fmData)
			sb.WriteString("---\n\n")
		}
	}

	sb.WriteString(page.Body)
	sb.WriteString("\n")
	return sb.String()
}

func addToIndex(fs FS, root string, page *Page, pt PageType) error {
	indexPath := filepath.Join(root, "wiki", "index.md")
	data, err := fs.ReadFile(indexPath)
	if err != nil {
		return err
	}

	content := string(data)

	title := ""
	tags := ""
	scopeStr := ""
	updated := ""
	if page.Frontmatter != nil {
		if t, ok := page.Frontmatter["title"].(string); ok {
			title = t
		}
		if t, ok := page.Frontmatter["tags"].([]interface{}); ok {
			var tagStrs []string
			for _, tag := range t {
				if s, ok := tag.(string); ok {
					tagStrs = append(tagStrs, s)
				}
			}
			tags = strings.Join(tagStrs, ", ")
		}
		if sl, ok := page.Frontmatter["scope_level"].(string); ok {
			scopeStr = sl
		}
		if sc, ok := page.Frontmatter["scope_code"].(string); ok {
			if scopeStr != "" {
				scopeStr += "/"
			}
			scopeStr += sc
		}
		if u, ok := page.Frontmatter["updated"].(string); ok {
			updated = u
		}
	}

	newLine := fmt.Sprintf("| %s | %s | %s | %s | %s | %s |", page.Slug, title, string(pt), tags, scopeStr, updated)

	lines := strings.Split(content, "\n")

	// 找到所有分隔线位置，按类型选择正确的插入位置
	var separatorPositions []int
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "|---") {
			separatorPositions = append(separatorPositions, i)
		}
	}

	// 根据类型选择插入位置：page→第1个, entity→第2个, concept→第3个
	insertAfter := 0
	switch pt {
	case PageTypeEntity:
		if len(separatorPositions) >= 2 {
			insertAfter = separatorPositions[1]
		} else if len(separatorPositions) >= 1 {
			insertAfter = separatorPositions[0]
		}
	case PageTypeConcept:
		if len(separatorPositions) >= 3 {
			insertAfter = separatorPositions[2]
		} else if len(separatorPositions) >= 1 {
			insertAfter = separatorPositions[0]
		}
	default:
		if len(separatorPositions) >= 1 {
			insertAfter = separatorPositions[0]
		}
	}

	var result []string
	inserted := false
	for i, line := range lines {
		result = append(result, line)
		if !inserted && i == insertAfter {
			result = append(result, newLine)
			inserted = true
		}
	}

	if !inserted {
		result = append(result, newLine)
	}

	return fs.WriteFile(indexPath, []byte(strings.Join(result, "\n")), 0644)
}

func UpdatePage(fs FS, root string, page *Page, newType ...PageType) error {
	pagePath, currentType, err := resolvePagePath(fs, root, page.Slug)
	if err != nil {
		return err
	}

	// 确定目标类型和目录
	targetType := currentType
	if len(newType) > 0 {
		targetType = newType[0]
	}

	content := buildPageContent(page)

	if targetType != currentType {
		// 类型变更：删除原文件，写入新目录
		if err := fs.Remove(pagePath); err != nil {
			return fmt.Errorf("删除原页面失败: %w", err)
		}
		newDir := pageDirs[targetType]
		if err := fs.MkdirAll(filepath.Join(root, newDir), 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
		newPath := filepath.Join(root, newDir, page.Slug+".md")
		if err := fs.WriteFile(newPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("写入页面失败: %w", err)
		}
	} else {
		if err := fs.WriteFile(pagePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("写入页面失败: %w", err)
		}
	}

	if _, err := RebuildIndex(fs, root); err != nil {
		return fmt.Errorf("重建索引失败: %w", err)
	}

	return nil
}

func DeletePage(fs FS, root, slug string) error {
	pagePath, _, err := resolvePagePath(fs, root, slug)
	if err != nil {
		return err
	}

	if err := fs.Remove(pagePath); err != nil {
		return fmt.Errorf("删除页面文件失败: %w", err)
	}

	if _, err := RebuildIndex(fs, root); err != nil {
		return fmt.Errorf("重建索引失败: %w", err)
	}

	return nil
}

func updateIndexRow(fs FS, root string, page *Page) error {
	indexPath := filepath.Join(root, "wiki", "index.md")
	data, err := fs.ReadFile(indexPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| "+page.Slug+" |") {
			title := ""
			tags := ""
			scopeStr := ""
			updated := ""
			pageType := "page"
			if page.Frontmatter != nil {
				if t, ok := page.Frontmatter["title"].(string); ok {
					title = t
				}
				if t, ok := page.Frontmatter["tags"].([]interface{}); ok {
					var tagStrs []string
					for _, tag := range t {
						if s, ok := tag.(string); ok {
							tagStrs = append(tagStrs, s)
						}
					}
					tags = strings.Join(tagStrs, ", ")
				}
				if sl, ok := page.Frontmatter["scope_level"].(string); ok {
					scopeStr = sl
				}
				if sc, ok := page.Frontmatter["scope_code"].(string); ok {
					if scopeStr != "" {
						scopeStr += "/"
					}
					scopeStr += sc
				}
				if u, ok := page.Frontmatter["updated"].(string); ok {
					updated = u
				}
			}
			result = append(result, fmt.Sprintf("| %s | %s | %s | %s | %s | %s |", page.Slug, title, pageType, tags, scopeStr, updated))
		} else {
			result = append(result, line)
		}
	}

	return fs.WriteFile(indexPath, []byte(strings.Join(result, "\n")), 0644)
}

func removeFromIndex(fs FS, root, slug string) error {
	indexPath := filepath.Join(root, "wiki", "index.md")
	data, err := fs.ReadFile(indexPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| "+slug+" |") {
			continue
		}
		result = append(result, line)
	}

	return fs.WriteFile(indexPath, []byte(strings.Join(result, "\n")), 0644)
}
