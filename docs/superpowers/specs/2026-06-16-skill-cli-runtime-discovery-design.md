# Skill 统一委托 CLI 发现运行时设计

**日期:** 2026-06-16  
**主题:** skill-cli-runtime-discovery

## 背景

OpenWiki CLI 已经实现统一配置发现顺序：

```text
--config / -c
→ OPENWIKI_CONFIG
→ 从当前工作目录向上搜索 openwiki.toml
→ ~/.openwiki/openwiki.toml
```

CLI 找到 `openwiki.toml` 后，再通过 `config.Load` 解析 `wiki_root`。如果 `wiki_root` 是相对路径，则相对于 `openwiki.toml` 所在目录解析。

但当前 wiki skills 的 Pre-condition 文档仍在各自手写发现逻辑，其中 `wiki-ingest`、`wiki-query`、`wiki-update`、`wiki-lint` 只描述了显式配置和向上搜索，没有描述 global fallback。这导致当当前目录和父目录都没有 `openwiki.toml` 时，agent 可能不会继续读取 `~/.openwiki/openwiki.toml`，即使 CLI 本身可以正确发现 global 配置。

## 目标

将所有 wiki skill 的运行时发现逻辑统一改为：

```text
skill 不自行实现配置发现链；
skill 委托 OpenWiki CLI 发现 active config；
skill 使用 CLI 输出解析 wiki_root 和 config source。
```

这样配置发现规则只有一个事实来源：CLI。

## 非目标

本设计不改变：

- `internal/config/discovery.go` 的发现顺序；
- `openwiki.toml` 的配置结构；
- `wiki_root` 的相对路径解析规则；
- AI skill 直接写 Markdown 文件的 file-first 模型；
- 分层索引协议。

## 设计

### 1. CLI 是 runtime discovery 的唯一事实来源

所有 wiki skill 在执行前应先调用：

```bash
openwiki config path --json
```

用于获取当前 active config 路径和来源：

```json
{
  "success": true,
  "data": {
    "path": "/Users/example/.openwiki/openwiki.toml",
    "source": "global"
  }
}
```

然后调用：

```bash
openwiki config show --json
```

用于读取解析后的 runtime config，包括 `wiki_root`。

skill 不应自行复制以下发现顺序：

```text
explicit → env → local → global
```

而是应引用 CLI 当前规则，并将 CLI 输出作为准。

### 2. 显式 config 的处理

如果用户显式给出 `openwiki.toml` 路径，skill 应将其传给 CLI：

```bash
openwiki --config /path/to/openwiki.toml config path --json
openwiki --config /path/to/openwiki.toml config show --json
```

如果用户给出项目目录，skill 可优先检查：

```text
<project>/openwiki.toml
```

如果存在，则调用：

```bash
openwiki --config <project>/openwiki.toml config path --json
openwiki --config <project>/openwiki.toml config show --json
```

如果不存在，则在该目录下运行 CLI discovery，或要求用户明确 config 路径。

### 3. global config 的用户提示

如果 `openwiki config path --json` 返回：

```json
"source": "global"
```

skill 必须明确告诉用户当前使用的是 global config：

```text
当前使用 global OpenWiki config: ~/.openwiki/openwiki.toml
```

目的：避免用户误以为当前 workflow 正在使用项目本地 wiki。

### 4. CLI 可用性 guardrail

因为 skill 依赖 CLI 做 runtime discovery，执行前应确认 CLI 可用：

```bash
openwiki config path --json
```

如果全局 `openwiki` 不可用或版本过旧，而当前工作区是 OpenWiki 仓库，则可使用 repo-built CLI：

```bash
go run ./cmd/openwiki config path --json
go run ./cmd/openwiki config show --json
```

如果两者都不可用，skill 不应自行猜测配置路径；应提示用户安装/更新 OpenWiki CLI 或显式提供 `openwiki.toml` 路径。

### 5. 适用 skill

需要统一更新：

- `skill/wiki-init/SKILL.md`
- `skill/wiki-ingest/SKILL.md`
- `skill/wiki-query/SKILL.md`
- `skill/wiki-update/SKILL.md`
- `skill/wiki-lint/SKILL.md`

`wiki-init` 是特殊情况：

- 新建 wiki 时仍负责初始化；
- 复用已有配置、检查当前 runtime、继续已有 wiki 时，也应通过 CLI discovery 获取 active config。

## 推荐 Pre-condition 模板

```markdown
## Pre-condition

Use the OpenWiki CLI as the source of truth for runtime discovery.

Resolve the active config:

```bash
openwiki config path --json
```

Read the runtime config:

```bash
openwiki config show --json
```

The CLI discovery order is:

1. `--config` / `-c`
2. `OPENWIKI_CONFIG`
3. Search upward from the current working directory for `openwiki.toml`
4. `~/.openwiki/openwiki.toml`

Do not reimplement this discovery chain inside the skill.

If the CLI reports `source = "global"`, tell the user explicitly that the workflow is using the global config at `~/.openwiki/openwiki.toml`.

If the global `openwiki` command is unavailable or too old, and this is the OpenWiki repository, use:

```bash
go run ./cmd/openwiki config path --json
go run ./cmd/openwiki config show --json
```

If no CLI path works, ask the user to install/update OpenWiki CLI or provide an explicit `openwiki.toml` path.
```

## Testing Strategy

### Static checks

Read the five `SKILL.md` files and assert:

- each contains `openwiki config path --json`;
- each contains `openwiki config show --json`;
- each says not to reimplement discovery inside the skill;
- each mentions `~/.openwiki/openwiki.toml` as CLI global fallback;
- each mentions warning the user when `source = "global"`.

### Manual behavior check

From a directory without local `openwiki.toml`, verify CLI returns global when global exists:

```bash
openwiki config path --json
```

Expected:

```json
{
  "success": true,
  "data": {
    "source": "global",
    "path": "<home>/.openwiki/openwiki.toml"
  }
}
```

## Acceptance Criteria

- All wiki skills delegate runtime discovery to CLI.
- No wiki skill contains a hand-maintained partial discovery chain that omits global fallback.
- Global config usage is explicit to the user.
- CLI remains the single source of truth for config discovery.
