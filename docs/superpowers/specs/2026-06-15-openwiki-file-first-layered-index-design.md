# OpenWiki File-first 分层索引架构设计

**日期:** 2026-06-15  
**主题:** openwiki-file-first-layered-index  
**状态:** 已确认设计

## 1. 背景与目标

OpenWiki 的长期定位是一个面向 AI 维护的本地 Markdown 知识库运行时。它不是传统意义上所有数据都必须通过 CLI CRUD 写入的数据库型工具，而是以文件系统和 Markdown 为主要工作界面，以 CLI 提供确定性的配置、校验、状态和修复能力。

本设计重新统一 OpenWiki 的产品/架构主线，解决以下问题：

- 运行时契约在历史文档中的 `WIKI.md` 与当前实现中的 `openwiki.toml` 不一致；
- `config-dir` 与 `wiki-root` 分离模型过重，不符合当前希望的极简项目本地心智；
- `index.md` 对查询检索非常重要，但如果承载全量索引会持续膨胀，消耗 AI 上下文；
- AI skill 与 CLI 的写入边界需要明确；
- 所有 wiki skill 需要升级到新的分层索引协议。

## 2. 核心决策

本设计确认以下架构决策：

1. **唯一运行时契约:** `openwiki.toml`。
2. **废弃 canonical `WIKI.md`:** `WIKI.md` 不再作为运行时契约，历史文档需要迁移或标注过期。
3. **配置发现顺序:** `explicit → env → local → global`。
4. **初始化心智:** 极简项目本地模型。
5. **默认数据根:** `./openwiki/`。
6. **写入模型:** AI skill 直接写 Markdown 文件，CLI 负责配置、校验、状态、修复和辅助扫描。
7. **内容事实来源:** 页面文件。
8. **检索入口事实来源:** 分层索引。
9. **顶层索引定位:** `wiki/index.md` 是轻量 Routing Index，不列全量页面。
10. **分片索引定位:** `wiki/indexes/` 承载可增长的 Shard Indexes。
11. **热门入口:** 不人工维护，由 query usage 自动生成。
12. **所有 wiki skill:** 必须按分层索引协议调整。

## 3. 整体架构

OpenWiki 采用：

> File-first Wiki Runtime with CLI Guardrails

系统结构为：

```text
openwiki.toml
  ↓
wiki_root/
  ↓
Markdown 知识文件 + 分层索引
  ↓
AI skill + CLI guardrails
```

### 3.1 唯一运行时契约

`openwiki.toml` 是唯一运行时契约，负责声明：

- `wiki_root`
- 主语言/副语言
- source types
- index categories
- remote sync 配置
- 未来可扩展运行时参数

skill 和 CLI 都必须从 `openwiki.toml` 解析运行时状态，不得依赖 `WIKI.md`、`CLAUDE.md`、`.agents/skills`、`.claude/skills` 等路径推断。

### 3.2 项目本地优先

OpenWiki 的默认心智是：

```text
在一个项目目录中放置 openwiki.toml；
openwiki.toml 指向该项目的 wiki_root；
后续命令和 skill 从当前目录向上发现 openwiki.toml。
```

推荐结构：

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
    ├── entities/
    └── concepts/
```

全局配置 `~/.openwiki/openwiki.toml` 只作为 fallback，不是主要教学路径。

### 3.3 文件优先，CLI 兜底

AI skill 写内容时直接编辑文件：

- 新增/更新页面文件；
- 更新分层索引；
- 追加 `wiki/log.md`；
- 使用模板保证 frontmatter 和正文结构；
- 执行内容级判断，比如摘要、标签、scope、交叉引用、实体页、概念页。

CLI 主要负责：

- 配置发现；
- 配置读取和校验；
- 状态统计；
- lint/健康检查；
- index 检查、重建或修复；
- 辅助读取和扫描。

CLI 是 guardrail，不是所有内容写入的唯一通道。

## 4. 目录结构与配置发现

### 4.1 推荐项目结构

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

| 路径 | 角色 |
|---|---|
| `openwiki.toml` | 唯一运行时契约 |
| `openwiki/` | 默认 `wiki_root` |
| `openwiki/raw/` | 原始素材，保留来源 |
| `openwiki/wiki/pages/` | 普通知识页 |
| `openwiki/entities/` | 实体页，如人、组织、项目、工具 |
| `openwiki/concepts/` | 概念页、分析页、查询沉淀 |
| `openwiki/wiki/index.md` | 轻量 Routing Index |
| `openwiki/wiki/indexes/` | Shard Indexes |
| `openwiki/wiki/log.md` | 操作日志 |

### 4.2 `openwiki.toml` 配置结构

推荐配置结构：

```toml
wiki_root = "./openwiki"

[wiki]
primary_language = "zh"
secondary_language = "en"

[wiki.source_types]
types = ["docs", "urls", "code", "notes"]

[wiki.index]
categories = ["资料页", "实体页", "概念页", "适用范围", "快速导航"]

[remote]
sync_path = ""
auto_sync = false
```

关键约定：

- `wiki_root` 可以是相对路径或绝对路径；
- 相对路径必须基于 `openwiki.toml` 所在目录解析，而不是基于当前 shell 工作目录；
- `openwiki.toml` 是唯一契约。

### 4.3 配置发现顺序

当用户没有显式指定配置时，OpenWiki 按以下顺序发现配置：

```text
--config / -c 指定的 openwiki.toml
→ OPENWIKI_CONFIG 指定的 openwiki.toml
→ 从当前工作目录向上搜索 openwiki.toml
→ ~/.openwiki/openwiki.toml
```

来源标记：

| 来源 | source |
|---|---|
| `--config` / `-c` | `explicit` |
| `OPENWIKI_CONFIG` | `env` |
| CWD 向上搜索 | `local` |
| `~/.openwiki/openwiki.toml` | `global` |

local 优先于 global，避免在项目目录中误用全局 Wiki。

## 5. 初始化体验与生命周期命令

### 5.1 初始化主心智

`openwiki init` 面向极简项目本地使用。

默认行为：

```bash
openwiki init
```

在当前目录创建：

```text
./openwiki.toml
./openwiki/
```

初始化后的结构：

```text
<cwd>/
├── openwiki.toml
└── openwiki/
    ├── raw/
    ├── wiki/
    │   ├── index.md
    │   ├── log.md
    │   ├── pages/
    │   └── indexes/
    ├── entities/
    └── concepts/
```

### 5.2 自定义 wiki_root

用户可以通过位置参数指定数据目录：

```bash
openwiki init <wiki-root>
```

仍然在当前目录创建 `./openwiki.toml`，但配置中写入指定的 `wiki_root`。不再主推独立 `config-dir`。

### 5.3 重复初始化与 `--force`

如果当前目录已存在 `./openwiki.toml`，再次执行 `openwiki init` 应返回明确错误：

```text
WIKI_ALREADY_EXISTS
```

并提示：

```text
当前目录已经是 OpenWiki 项目。
如需继续使用，请运行 openwiki status。
如需覆盖，请使用 --force。
```

`openwiki init --force` 可以覆盖当前目录的 `openwiki.toml` 并补齐缺失结构，但不应默认删除已有 `wiki_root` 内容。

### 5.4 生命周期命令

配置类：

```bash
openwiki config path
openwiki config show
openwiki config validate
openwiki config get <key>
openwiki config set <key> <value>
```

状态类：

```bash
openwiki status
```

索引类：

```bash
openwiki index check
openwiki index rebuild
```

校验类：

```bash
openwiki lint
```

辅助读取类：

```bash
openwiki page list
openwiki page get <slug>
```

现有 `openwiki page create/update/delete` 可保留为人工或脚本便利命令，但不再是 AI skill 写内容的主路径。

## 6. 分层索引设计

### 6.1 为什么需要分层索引

`wiki/index.md` 对检索非常重要，但不能无限增长。如果它承载全量页面列表、所有 tag、所有 scope、所有实体、所有热门入口，会带来两个问题：

1. AI query 每次读取它时上下文消耗越来越大；
2. 顶层索引越来越难维护，最终失去快速路由作用。

因此 OpenWiki 采用：

```text
wiki/index.md              # 顶层 Routing Index，轻量、稳定
wiki/indexes/*.md          # Shard Indexes，可增长、可拆分
wiki/indexes/query-usage.jsonl
wiki/indexes/hot.md        # 根据 query 使用自动生成
```

目标是：

```text
index.md 负责路由；
indexes/ 负责展开；
页面文件负责内容。
```

### 6.2 顶层 `wiki/index.md` 是 Routing Index

`wiki/index.md` 不列全量页面，只包含：

1. Wiki 概览；
2. 稳定检索路由；
3. 分片索引目录；
4. 索引状态。

推荐模板：

```markdown
# Wiki 索引

## 概览

<这个 Wiki 覆盖什么、边界是什么、主要用于回答什么。>

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

- Last rebuilt: <date>
- Page count: <number>
- Index health: <ok|warning|stale>
- Query usage records: <number>
- Known gaps:
  - <gap>
```

硬约束：

- 不列全量页面；
- 不列具体主题大全；
- 不维护人工高频入口表；
- 目标控制在 200 行以内。

### 6.3 路由规则必须是稳定维度

`wiki/index.md` 的检索路由不能写成主题大全。它只能描述稳定维度：

- scope
- entity
- concept
- tag
- recent
- hot

具体主题映射放入分片索引，例如 `wiki/indexes/tags.md`、`wiki/indexes/scopes.md`、`wiki/indexes/scopes/<scope>.md`。

### 6.4 分片索引职责

| 文件 | 职责 |
|---|---|
| `indexes/scopes.md` | scope 概览，必要时指向 `indexes/scopes/<scope>.md` |
| `indexes/entities.md` | entity 页入口 |
| `indexes/concepts.md` | concept 页入口 |
| `indexes/tags.md` | tag → 页面入口 |
| `indexes/recent.md` | 最近更新页面 |
| `indexes/hot.md` | 根据查询使用记录生成的热门入口 |
| `indexes/query-usage.jsonl` | 每次 query 的结构化使用记录 |

当某个分片过大时，可以继续拆分：

```text
wiki/indexes/scopes/repo-openwiki.md
wiki/indexes/tags/config.md
wiki/indexes/tags/index.md
```

分片索引中的单条记录应保持短小：

```markdown
- [[slug]] — <title> | <type> | <scope> | <tags> | <updated> | <一句话摘要>
```

摘要建议限制在 80-120 中文字以内。

### 6.5 动态 Hot Index

高频入口不人工维护，而是由 query 使用记录生成。

每次 `wiki-query` 完成后，追加：

```text
wiki/indexes/query-usage.jsonl
```

记录格式：

```json
{
  "time": "2026-06-15T15:30:00+08:00",
  "query": "openwiki init 的设计是什么",
  "matched_indexes": ["indexes/scopes.md", "indexes/tags.md"],
  "read_pages": ["init-project-local-model", "config-discovery-order"],
  "cited_pages": ["init-project-local-model"],
  "intent_tags": ["init", "config", "cli"]
}
```

`indexes/hot.md` 由 query usage 自动生成：

```markdown
# 热门入口

> 自动生成自最近查询记录。

## 最近 30 天高频页面

| 页面 | 命中次数 | 最近命中 | 常见问题 |
|---|---:|---|---|
| [[config-discovery-order]] | 12 | 2026-06-15 | 配置发现、local/global 优先级 |

## 高频查询主题

| 主题 | 相关页面 | 命中次数 |
|---|---|---:|
| config | [[config-discovery-order]], [[openwiki-toml]] | 18 |
```

刷新策略：

- `wiki-query` 每次追加 usage；
- `openwiki index rebuild` 必定重建 `hot.md`；
- 后续可增加 `openwiki index refresh-hot`；
- 如果 usage 增长但 hot 未刷新，`index check` 应报告 stale warning。

### 6.6 index check / rebuild

CLI 应提供：

```bash
openwiki index check
openwiki index rebuild
```

`index check` 检查：

- 顶层 index 是否符合 Routing Index 模板；
- `wiki/indexes/` 必要文件是否存在；
- 分片索引是否覆盖所有页面；
- 分片索引是否引用不存在页面；
- scope/tag/entity/concept 分片是否与 frontmatter 一致；
- `hot.md` 是否落后于 `query-usage.jsonl`；
- index 状态是否过期或为 warning。

`index rebuild`：

- 扫描 `wiki/pages/`、`entities/`、`concepts/`；
- 读取 frontmatter；
- 提取/生成短摘要；
- 重建必要分片索引；
- 重建 `recent.md`；
- 根据 `query-usage.jsonl` 重建 `hot.md`；
- 更新顶层 `index.md` 的索引状态；
- 重建前备份旧索引文件。

### 6.7 query 渐进检索流程

`wiki-query` 的新流程：

```text
1. 读取 openwiki.toml
2. 读取 wiki/index.md
3. 使用 Routing Index 判断检索维度：
   - scope?
   - entity?
   - concept?
   - tag?
   - recent?
   - unknown?
4. 读取 1-3 个相关分片索引
5. 从分片索引选择候选页面
6. 读取候选页面全文
7. 回答并引用 [[slug]]
8. 追加 query-usage.jsonl
9. 必要时提示刷新 hot.md 或运行 index rebuild
```

如果顶层 index 或分片索引损坏：

- 不能假装正常；
- 必须提示索引异常；
- 可临时扫描页面文件补漏；
- 但要说明检索质量可能下降；
- 建议运行 `openwiki index rebuild`。

## 7. AI skill 与 CLI 职责边界

### 7.1 AI skill 的职责

AI skill 负责所有内容级工作：

- 阅读 raw/source；
- 提炼摘要；
- 生成页面标题；
- 生成 slug；
- 生成 tags；
- 判断 scope；
- 判断页面类型；
- 组织正文结构；
- 添加交叉引用；
- 保留来源；
- 直接写页面、分层索引和日志。

AI skill 可以直接写：

```text
wiki/pages/<slug>.md
entities/<slug>.md
concepts/<slug>.md
wiki/index.md
wiki/indexes/*.md
wiki/log.md
```

但必须遵守模板和分层索引协议。

### 7.2 CLI 的职责

CLI 负责确定性能力：

- 配置发现与校验；
- 状态扫描；
- 索引检查和重建；
- lint；
- 辅助读取。

CLI 不承担主要内容写入，但现有 page write 命令可保留为人工/脚本便利能力。

### 7.3 Page CRUD 命令重新定位

`openwiki page list` 和 `openwiki page get` 仍有价值，作为确定性读取和调试工具。它们应从文件系统读取，而不是依赖分片索引作为唯一来源。

`openwiki page create/update/delete` 可保留为人工或脚本便利命令，但必须遵守与 skill 相同的一致性规则：

- 写页面文件；
- 更新分层索引；
- 追加 log；
- index 更新失败时返回 partial failure/warning；
- 不允许静默破坏检索入口。

### 7.4 模板约束

推荐模板目录：

```text
skill/wiki-init/templates/
├── openwiki.toml
├── index.md
├── indexes/
│   ├── scopes.md
│   ├── entities.md
│   ├── concepts.md
│   ├── tags.md
│   ├── recent.md
│   └── hot.md
├── page.md
├── entity.md
├── concept.md
└── log-entry.md
```

如果多个 skill 共用模板，后续可抽出 shared template。

页面模板应至少包含：

```yaml
---
title: ""
summary: ""
tags: []
scope_level: ""
scope_code: ""
updated: "YYYY-MM-DD"
sources: []
---
```

entity 页面额外包含：

```yaml
entity_type: "person|org|project|tool"
```

concept 页面可以包含：

```yaml
concept_type: "design|analysis|decision|answer|report"
```

### 7.5 skill 写入工作流标准化

所有写入型 skill 都必须遵循统一流程：

```text
1. 解析 openwiki.toml
2. 校验 wiki_root 基础结构
3. 读取 Routing Index
4. 读取必要分片索引
5. 读取相关页面
6. 生成或修改页面文件
7. 更新相关分片索引
8. 更新 recent/hot 或标记 stale
9. 追加 log
10. 运行或建议运行校验
11. 向用户报告写入结果和 warning
```

`wiki-query` 必须遵循：

```text
1. 解析 openwiki.toml
2. 读取 Routing Index
3. 按路由维度选择分片索引
4. 读取 1-3 个相关分片索引
5. 选择候选页面
6. 读取候选页面全文
7. 回答并引用
8. 追加 query-usage.jsonl
9. 根据用户选择保存回答到 concepts/
10. 如果保存，则同步更新 concept/tags/scopes/recent 分片
```

### 7.6 对现有 skill 文档的必改项

必须同步更新：

- `skill/wiki-init/SKILL.md`
  - 移除 config-dir/WIKI.md 叙述；
  - 使用 `openwiki.toml`；
  - 初始化 `wiki/indexes/`；
  - 生成 Routing Index。

- `skill/wiki-ingest/SKILL.md`
  - 不再要求通过 `openwiki page create` 写页面；
  - 改为直接使用模板写 Markdown；
  - 写入后更新分片索引；
  - 必要时调用 CLI 校验。

- `skill/wiki-query/SKILL.md`
  - 不再默认完整读取全量 index；
  - 改为读取 Routing Index + 分片索引；
  - 查询后追加 query usage；
  - 保存答案时更新 concept 分片。

- `skill/wiki-update/SKILL.md`
  - 直接编辑页面；
  - 同步维护分片索引；
  - 每页 diff 后确认；
  - scope/tag/type 改变时处理旧分片和新分片。

- `skill/wiki-lint/SKILL.md`
  - 增加分层索引校验；
  - 检查 Routing Index、Shard Index、hot stale；
  - 建议或调用 `openwiki index rebuild`。

分层索引不是单个文件格式变化，而是所有 wiki skill 的协议升级。

## 8. 错误处理、校验与测试策略

### 8.1 错误处理原则

OpenWiki 区分三类失败：

```text
阻塞性失败；
部分成功；
非阻塞 warning。
```

### 8.2 阻塞性失败

以下情况应阻止继续执行：

- 找不到 `openwiki.toml`；
- TOML 解析失败；
- `wiki_root` 缺失；
- `wiki_root` 无法解析；
- `wiki_root` 不存在且当前命令不是 init；
- 必要目录缺失且无法创建；
- 页面 slug 非法；
- 目标页面已存在但当前操作是 create；
- 要更新的页面不存在；
- entity 页面缺少合法 `entity_type`；
- 页面 frontmatter 不符合模板且无法安全修复。

错误码示例：

```text
CONFIG_NOT_FOUND
CONFIG_PARSE_ERROR
CONFIG_MISSING_FIELD
CONFIG_INVALID_PATH
WIKI_LAYOUT_INVALID
PAGE_ALREADY_EXISTS
PAGE_NOT_FOUND
INVALID_SLUG
INVALID_FRONTMATTER
INVALID_ENTITY_TYPE
```

### 8.3 部分成功

典型情况：页面文件已写入，但索引更新失败。

处理原则：

- 不静默成功；
- 不默认回滚页面文件；
- 明确说明页面已写入；
- 明确说明索引未更新；
- 明确说明检索可能找不到；
- 给出修复命令。

文本输出示例：

```text
页面已写入：wiki/pages/foo.md
但分片索引更新失败：wiki/indexes/tags.md

该页面可能无法被 wiki-query 检索到。
建议运行：openwiki index rebuild
```

JSON 输出示例：

```json
{
  "success": false,
  "error": {
    "code": "INDEX_UPDATE_FAILED",
    "message": "页面已写入，但索引更新失败，检索可能找不到该页面",
    "details": {
      "content_written": true,
      "index_updated": false,
      "log_updated": false,
      "written_paths": ["wiki/pages/foo.md"],
      "failed_paths": ["wiki/indexes/tags.md"],
      "suggested_fix": "openwiki index rebuild"
    }
  }
}
```

### 8.4 非阻塞 warning

以下情况不阻止内容可用，但必须提示：

- `log.md` 追加失败；
- `hot.md` 未刷新；
- `query-usage.jsonl` 追加失败；
- `recent.md` 不是最新；
- index health 不是 ok；
- lint 有 yellow warning；
- 某些可选 metadata 缺失但不影响基本检索。

### 8.5 CLI 校验层级

Level 1：配置校验

```bash
openwiki config validate
```

Level 2：结构校验

```bash
openwiki status
```

Level 3：一致性校验

```bash
openwiki index check
openwiki lint
```

`index check` 偏结构化索引一致性；`lint` 偏知识库健康。

### 8.6 skill 校验策略

`wiki-init` 必须运行：

```bash
openwiki config validate
openwiki status
```

`wiki-ingest` 写入后至少确认：

- 写入文件存在；
- 相关分片索引包含新 slug；
- `recent.md` 有更新；
- `log.md` 有记录；
- 如果有 CLI 支持，可运行 `openwiki index check`。

`wiki-query` 查询前若发现 index 或关键分片缺失，必须提示异常；查询后追加 `query-usage.jsonl`。

`wiki-update` 写入前展示 diff 并逐页确认；写入后检查旧分片移除、新分片加入、recent 更新和 log 记录。

`wiki-lint` 输出问题列表、严重级别、影响、修复建议、可运行修复命令，以及是否建议 `openwiki index rebuild`。

### 8.7 测试策略

测试覆盖文档、CLI、skill 三层。

#### 单元测试

配置发现：

- explicit；
- env；
- local；
- global；
- local 优先于 global；
- 相对 `wiki_root` 基于配置文件目录解析。

配置校验：

- 缺失 `wiki_root`；
- 无效语言；
- wiki_root 不存在；
- 布局缺失；
- indexes 目录缺失。

索引生成：

- 从页面 frontmatter 生成分片索引；
- 生成 recent；
- 从 query usage 生成 hot；
- 顶层 Routing Index 状态更新。

#### 集成测试

初始化应创建：

```text
openwiki.toml
openwiki/raw/
openwiki/wiki/index.md
openwiki/wiki/indexes/
openwiki/wiki/log.md
openwiki/wiki/pages/
openwiki/entities/
openwiki/concepts/
```

状态检查覆盖：

- 空 wiki status 正常；
- 缺失 index 时报告 warning/error；
- 缺失分片索引时报告 warning；
- 页面存在但分片未覆盖时报告 index 不一致。

索引重建覆盖：

- 删除 index 后运行 `openwiki index rebuild` 能恢复；
- 新增页面后 rebuild 能加入分片；
- 删除页面后 rebuild 能移除死链；
- query usage 能生成 hot。

#### Skill 静态测试

读取所有相关 `SKILL.md`，断言：

- 不再将 `WIKI.md` 作为运行时契约；
- 不再要求 AI 写内容必须通过 `openwiki page create/update`；
- 明确使用 `openwiki.toml`；
- 明确分层索引；
- 明确 query usage；
- 明确 Routing Index + Shard Indexes；
- 明确写入后维护分片索引。

#### E2E 测试

最小 happy path：

```text
openwiki init
→ wiki-ingest 写一个资料页、一个 entity、一个 concept
→ 检查分片索引更新
→ wiki-query 读取 routing index + shard index，回答并记录 query usage
→ 保存 query answer 到 concepts
→ index rebuild
→ lint/status 通过
```

异常路径：

```text
页面存在但索引缺失
→ wiki-query 提示检索质量风险
→ openwiki index rebuild
→ query 正常命中
```

## 9. 验收标准

第一阶段架构改造完成时，应满足：

- 所有文档统一使用 `openwiki.toml`；
- `openwiki init` 创建项目本地结构；
- 默认 `wiki_root = ./openwiki/`；
- 配置发现顺序为 `explicit → env → local → global`；
- `wiki/index.md` 是 Routing Index；
- `wiki/indexes/` 存在并有基本分片；
- 所有 wiki skill 都遵守分层索引；
- AI skill 写文件，不依赖 CLI 写页面；
- CLI 能检查并重建索引；
- query 会记录 usage；
- hot index 能由 usage 生成；
- 测试覆盖初始化、发现、索引、skill 文档和最小 E2E。

## 10. 非目标

本设计不要求第一阶段完成以下内容：

- 复杂搜索引擎或向量检索；
- 多用户协作锁；
- 真正事务型文件写入；
- 完整废弃 `page create/update/delete`；
- 自动生成高质量长摘要；
- 所有历史 `openspec` 一次性完全重写。

