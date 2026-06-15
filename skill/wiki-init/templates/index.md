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
