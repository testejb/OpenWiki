# OpenWiki — AI 驱动的个人知识库

**语言 / Language / 言語：** 中文（默认）｜ [English](README.en.md) ｜ [日本語](README.ja.md)

---

## 这是什么

OpenWiki 是一个面向 AI skill / agent 的文件优先个人知识库脚手架。Markdown 文件是主要写入界面，AI skills 直接编辑知识文件；CLI 不接管内容写入，而是提供配置发现、校验、状态查看、索引检查和索引重建等 guardrails。

**核心思路：**

- `openwiki.toml` 是唯一运行时契约，也是唯一 canonical runtime contract；
- 默认 `wiki_root` 是 `./openwiki/`；
- 项目本地运行模型优先：配置和 wiki 数据应随项目一起被发现和使用；
- `wiki/index.md` 是轻量 Routing Index，不列全量页面；
- `wiki/indexes/` 是 Shard Indexes，用于承载可增长索引。

---

## 运行模型

OpenWiki 使用项目本地运行模型：

```text
<project>/
├── openwiki.toml            # 唯一运行时契约；目标项目顶层契约
└── openwiki/                # 默认 wiki_root
    ├── raw/                 # 原始素材
    ├── wiki/
    │   ├── index.md         # 轻量 Routing Index
    │   ├── log.md           # 操作日志
    │   ├── pages/           # 普通知识页
    │   └── indexes/         # Shard Indexes / 分片索引
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

openwiki.toml 是唯一运行时契约。

- `openwiki.toml` 是唯一运行时契约；
- 默认 `wiki_root` 是 `./openwiki/`；
- 页面 Markdown 文件是内容事实来源；
- `wiki/index.md` 是轻量 Routing Index，只做检索路由，不列全量页面；
- `wiki/indexes/` 是 Shard Indexes，包括 `scopes.md`、`entities.md`、`concepts.md`、`tags.md`、`recent.md`、`hot.md`、`query-usage.jsonl`；
- AI skills 直接写 Markdown 文件；
- CLI 提供配置发现、校验、状态、`index check`、`index rebuild` 等 guardrails。

> 当前 CLI 的 `openwiki init` 会在目标 wiki root 内写入 `openwiki.toml`。若要采用上面的项目顶层契约模型，可以在项目顶层放置 `openwiki.toml` 并设置 `wiki_root = "./openwiki"`，或通过 `--config` / `-c` 指定该契约文件。

---

## 快速开始

### 前置条件

- 任意兼容 `skill.io` 或可读取本仓库 skills 的 AI agent / 工具
- （可选）[agent-browser](https://github.com/mediar-ai/agent-browser)：用于联网补充与查证

### 安装

```bash
git clone https://github.com/crabin/llm-wiki.git openwiki-project
cd openwiki-project
```

在你的 AI agent 中加载本仓库，并确保它能读取 `skill/` 中的公开 wiki skills。

### 初始化

当前 CLI 初始化一个 wiki root：

```bash
openwiki init ./openwiki/
```

该命令会创建 `./openwiki/`，并在该目录内写入 `openwiki.toml`、`wiki/index.md`、`wiki/indexes/`、`raw/`、`entities/`、`concepts/` 等文件和目录。

如果需要项目本地顶层契约，可在项目根目录使用如下 `openwiki.toml`：

```toml
wiki_root = "./openwiki"

[wiki]
primary_language = "zh"
secondary_language = "en"
```

之后可以显式指定：

```bash
openwiki --config ./openwiki.toml status
openwiki --config ./openwiki.toml index check
openwiki --config ./openwiki.toml index rebuild
```

运行时查找规则：

1. `--config` / `-c` 显式指定
2. `OPENWIKI_CONFIG`
3. 从当前工作目录向上搜索 `openwiki.toml`
4. `~/.openwiki/openwiki.toml`

---

## 目录结构

```text
openwiki-project/
├── skill/                    # 公开 wiki skill 目录
│   ├── wiki-init/
│   ├── wiki-ingest/
│   ├── wiki-query/
│   ├── wiki-lint/
│   ├── wiki-update/
│   └── agent-browser/
├── openwiki.toml             # 项目本地运行时契约（目标模型，可用 --config 指定）
├── openwiki/                 # 默认 wiki_root
│   ├── raw/
│   ├── wiki/
│   │   ├── index.md          # Routing Index
│   │   ├── log.md
│   │   ├── pages/
│   │   └── indexes/          # Shard Indexes
│   │       ├── scopes.md
│   │       ├── entities.md
│   │       ├── concepts.md
│   │       ├── tags.md
│   │       ├── recent.md
│   │       ├── hot.md
│   │       └── query-usage.jsonl
│   ├── entities/
│   └── concepts/
├── README.md
├── README.en.md
└── README.ja.md
```

---

## Skills 与 CLI 分工

### AI skills

- `wiki-init`：准备项目本地契约和 wiki 文件结构。
- `wiki-ingest`：读取素材，与用户确认重点后直接写入 Markdown 页面，并维护相关 Shard Indexes。
- `wiki-query`：先读 `wiki/index.md` Routing Index，再按需要读取 `wiki/indexes/` 分片和相关页面；可把有价值回答沉淀到 `concepts/`。
- `wiki-lint`：检查断链、孤立页面、矛盾、过期内容和索引健康。
- `wiki-update`：直接编辑已有 Markdown 页面，更新反向链接、日志和分片索引。
- `agent-browser`：联网抓取和查证，提供可引用的 URL 与页面内容。

### CLI guardrails

CLI 负责非内容写入的运行时护栏：

- 发现 `openwiki.toml` 并解析 `wiki_root`；
- 校验必要目录、`wiki/index.md` 和 `wiki/indexes/`；
- 输出状态和索引健康摘要；
- 运行 `openwiki index check` 检查 Routing Index 与 Shard Indexes；
- 运行 `openwiki index rebuild` 从 Markdown 页面和 `query-usage.jsonl` 重建索引。

---

## E2E 测试

- 快速 deterministic Artifact E2E：
  ```bash
  python3 -m unittest tests.test_wiki_skill_workflow_e2e -v
  ```
- 全量 fast 测试：
  ```bash
  python3 -m unittest discover -s tests -p "test_*.py"
  ```
- 慢速真实 agent smoke E2E：
  ```bash
  SKILL_AGENT_E2E=1 SKILL_AGENT_RUNNER=/path/to/compatible-agent-wrapper python3 -m unittest tests.test_agent_skill_smoke_e2e -v
  ```

说明：真实 runner 用例默认跳过，只有设置 `SKILL_AGENT_E2E=1` 后才执行。

---

## 设计理念

- **文件优先**：Markdown 文件是知识库内容的主要事实来源。
- **中立契约**：运行时只依赖 `openwiki.toml`，不依赖特定智能体命名。
- **分层索引**：Routing Index 保持轻量，Shard Indexes 承载增长。
- **AI 写入，CLI 护航**：AI skills 负责编辑内容，CLI 负责发现、校验和修复索引。
- **来源可追溯**：关键结论应绑定文件路径或 URL。

---

## License

MIT
