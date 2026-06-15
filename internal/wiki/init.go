package wiki

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

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

const logTemplate = `# 操作日志

| 时间 | 操作 | 详情 |
|------|------|------|
`

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

func Init(fs FS, root string, cfg interface{}) error {
	openwikiPath := filepath.Join(root, "openwiki.toml")
	if _, err := fs.Stat(openwikiPath); err == nil {
		return fmt.Errorf("wiki 实例已存在: %s", root)
	}

	return initInternal(fs, root, cfg)
}

func InitForce(fs FS, root string, cfg interface{}) error {
	return initInternal(fs, root, cfg)
}

func initInternal(fs FS, root string, cfg interface{}) error {
	dirs := []string{
		filepath.Join(root, "wiki", "pages"),
		filepath.Join(root, "wiki", "indexes"),
		filepath.Join(root, "raw"),
		filepath.Join(root, "concepts"),
		filepath.Join(root, "entities"),
	}
	for _, dir := range dirs {
		if err := fs.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败 %s: %w", dir, err)
		}
	}

	if err := fs.WriteFile(filepath.Join(root, "wiki", "index.md"), []byte(indexTemplate), 0644); err != nil {
		return fmt.Errorf("创建 index.md 失败: %w", err)
	}

	if err := fs.WriteFile(filepath.Join(root, "wiki", "log.md"), []byte(logTemplate), 0644); err != nil {
		return fmt.Errorf("创建 log.md 失败: %w", err)
	}

	indexFiles := map[string]string{
		filepath.Join(root, "wiki", "indexes", "scopes.md"):         scopesIndexTemplate,
		filepath.Join(root, "wiki", "indexes", "entities.md"):       entitiesIndexTemplate,
		filepath.Join(root, "wiki", "indexes", "concepts.md"):       conceptsIndexTemplate,
		filepath.Join(root, "wiki", "indexes", "tags.md"):           tagsIndexTemplate,
		filepath.Join(root, "wiki", "indexes", "recent.md"):         recentIndexTemplate,
		filepath.Join(root, "wiki", "indexes", "hot.md"):            hotIndexTemplate,
		filepath.Join(root, "wiki", "indexes", "query-usage.jsonl"): "",
	}
	for path, content := range indexFiles {
		if err := fs.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("创建索引文件失败 %s: %w", path, err)
		}
	}

	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("编码 TOML 失败: %w", err)
	}

	openwikiPath := filepath.Join(root, "openwiki.toml")
	if err := fs.WriteFile(openwikiPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("创建 openwiki.toml 失败: %w", err)
	}

	return nil
}
