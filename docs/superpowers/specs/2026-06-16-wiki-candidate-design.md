# Wiki Candidate 候选知识审核设计

**日期:** 2026-06-16  
**主题:** wiki-candidate

## 背景

OpenWiki 当前已有 `wiki-ingest`，用于把用户提供的来源材料正式写入 wiki 页面、实体页、概念页，并更新分层索引与日志。现在希望新增一个前置能力：从 coding agent 的会话历史或记忆文件中发现有复用价值的知识候选，先生成飞书审核文档，由用户勾选确认后，再交给 `wiki-ingest` 入库。

首期候选来源是 code agent：

- `traex`：`/Users/bytedance/.trae/cli/history.jsonl`
- `trae-ide`：`/Users/bytedance/.trae-cn/memory/projects/**/session_memory_*.jsonl`

该能力不应局限于 codeagent，因此 skill 命名为 `wiki-candidate`。`codeagent` 是首期候选来源，未来可扩展 URL、文档集合、代码库、剪贴板等来源。

## 目标

- 新增 `wiki-candidate` skill，作为候选知识发现与审核入口。
- OpenWiki CLI 新增 `candidate codeagent` 子命令，负责 codeagent session 扫描、增量状态和 pending/commit。
- 从 codeagent records 中按平衡召回原则抽取候选知识。
- 生成用户可读、机器可解析的飞书候选审核文档。
- 用户通过候选卡片标题行的 checkbox 决定哪些候选允许入库。
- 更新 `wiki-ingest`，让它识别 Candidate Review Doc，并严格只处理已勾选候选。
- 对已勾选候选，`wiki-ingest` 视为用户已确认入库，不再进行二次确认，直接执行入库流程。

## 非目标

本期不做：

- 候选级去重。
- CLI 自动语义抽取。
- CLI 创建飞书文档。
- CLI 直接写入 wiki。
- 除 `traex`、`trae-ide` 外的 code agent。
- 批量更新已有飞书审核文档。
- 复杂飞书权限或分享设置管理。

## 总体架构

### 新增 skill

```text
skill/wiki-candidate/
  SKILL.md
  references/
    codeagent-extraction-rules.md
    review-doc-protocol.md
```

`wiki-candidate` 负责：

1. 调用 OpenWiki CLI 获取 codeagent 新增 records。
2. 读取抽取规则与审核文档协议。
3. 从新增 records 中提取候选知识。
4. 对候选进行分类、slug 生成、脱敏和证据整理。
5. 使用 `lark-doc` 创建飞书审核文档。
6. 保存本地候选快照。
7. 在飞书文档与快照都成功后调用 CLI commit 推进扫描状态。
8. 向用户报告飞书链接、候选数量、状态路径和后续入库方式。

### 新增 CLI

建议新增：

```bash
openwiki candidate codeagent scan --json
openwiki candidate codeagent commit --pending <pending.json> --review-doc-url <url> --snapshot <snapshot.json> --json
openwiki candidate codeagent status --json
```

CLI 负责确定性工作：

- 复用现有配置发现链。
- 读取 `[candidate]` 与 `[candidate.codeagent]` 配置。
- 发现 `traex` / `trae-ide` session 文件。
- 按状态增量扫描 JSONL records。
- 生成 pending scan 文件。
- 在 commit 阶段原子推进 `state.json`。
- 写入人类可读 `run.log`。

CLI 不负责语义抽取、飞书文档创建或正式入库。

### 修改 `wiki-ingest`

新增 Candidate Review Doc guardrail：当来源飞书文档包含：

```text
OPENWIKI_CANDIDATE_REVIEW_DOC v1
admission: ONLY_CHECKED_CANDIDATES
```

`wiki-ingest` 必须进入候选审核文档解析模式，只处理已勾选候选卡片。未勾选候选是用户明确拒绝入库，不得总结、引用、合并或作为背景材料使用。

## 配置设计

在 `openwiki.toml` 中新增可选配置：

```toml
[candidate]
state_dir = ""
run_log_path = ""
snapshot_dir = ""

[candidate.codeagent]
enabled = true
state_path = ""
pending_dir = ""
run_log_path = ""
snapshot_dir = ""
initial_days = 14
max_records_per_run = 500

[[candidate.codeagent.agents]]
name = "traex"
type = "traex-history"
paths = ["/Users/bytedance/.trae/cli/history.jsonl"]
enabled = true

[[candidate.codeagent.agents]]
name = "trae-ide"
type = "trae-ide-memory"
paths = ["/Users/bytedance/.trae-cn/memory/projects/**/session_memory_*.jsonl"]
enabled = true
```

### 默认值

如果 `[candidate]` 未配置：

```text
candidate.state_dir     = <wiki_root>/candidate
candidate.run_log_path  = <wiki_root>/candidate/run.log
candidate.snapshot_dir  = <wiki_root>/candidate/reviews
```

codeagent 默认使用独立子目录：

```text
state_path    = <wiki_root>/candidate/codeagent/state.json
pending_dir   = <wiki_root>/candidate/codeagent/pending
run_log_path  = <wiki_root>/candidate/codeagent/run.log
snapshot_dir  = <wiki_root>/candidate/codeagent/reviews
```

如果未显式配置 `candidate.codeagent.agents`，CLI 内置首期两个默认来源：`traex` 与 `trae-ide`。如果显式配置 agents，则以配置为准，不追加内置来源，避免重复扫描。

所有 candidate 路径若为相对路径，均相对于 `wiki_root` 解析，不相对于当前工作目录。

## 增量扫描状态

默认状态文件：

```text
<wiki_root>/candidate/codeagent/state.json
```

结构：

```json
{
  "version": 1,
  "source": "codeagent",
  "updated_at": "2026-06-16T17:00:00+08:00",
  "files": {
    "/Users/bytedance/.trae/cli/history.jsonl": {
      "agent": "traex",
      "type": "traex-history",
      "file_id": "dev:inode 或 fallback",
      "size": 50731,
      "mtime": "2026-06-16T16:00:00+08:00",
      "processed_lines": 128,
      "processed_bytes": 50731,
      "tail_hash": "sha256:last-5-lines",
      "last_scanned_at": "2026-06-16T17:00:00+08:00"
    }
  }
}
```

首期不记录候选 hash，不做候选级去重。

### Pending scan

`scan` 不直接推进状态，而是创建 pending 文件：

```text
<wiki_root>/candidate/codeagent/pending/2026-06-16-173000-scan.json
```

pending 文件包含：

- 配置路径、`wiki_root`、状态路径、日志路径、快照目录。
- 扫描限制。
- 本轮新增 records。
- 拟推进的 `state_updates`。

`state_updates` 固化在 pending 中，commit 不重新扫描文件，避免 scan 后文件继续 append 导致推进过头。

### Commit

`wiki-candidate` 只有在飞书审核文档创建成功且本地候选快照保存成功后，才调用：

```bash
openwiki candidate codeagent commit --pending <pending.json> --review-doc-url <url> --snapshot <snapshot.json> --json
```

commit 必须：

- 校验 pending 存在且状态为 pending。
- 校验 `review_doc_url` 非空。
- 校验 snapshot 文件存在。
- 将 pending 中的 `state_updates` 原子合并到 `state.json`。
- 将 pending 标记为 committed。
- 追加 `run.log`。

如果飞书文档或快照失败，不得推进状态，避免丢失未审核 records。

## 文件变化处理

- 文件变大：从 `processed_bytes` 续扫。
- 文件不变：跳过。
- 文件变小：视为 truncate 或轮转，从头扫描，受首次运行限制约束，并记录 warning。
- `mtime` 变化且 `tail_hash` 不一致：保守重新扫描该文件，受首次运行限制约束，并记录 warning。
- `file_id` 改变：视为新文件，从头扫描，受首次运行限制约束。

## 首次运行限制

如果 `state.json` 不存在，或某个文件未出现在 state 中，视为首次扫描该文件。

默认限制：

```text
initial_days = 14
max_records_per_run = 500
```

含义：

- 只选 timestamp 在最近 14 天内的 records。
- 如果超过 500 条，只取最新 500 条。
- 对无 timestamp 的记录，如果文件 mtime 在最近 14 天内可纳入，否则跳过并记录 warning。

CLI 可提供参数覆盖，例如：

```bash
openwiki candidate codeagent scan --initial-days 30 --max-records 1000 --json
```

## Run log 与候选快照

默认 run log：

```text
<wiki_root>/candidate/codeagent/run.log
```

示例：

```text
2026-06-16T17:30:00+08:00 | scan_start | source=codeagent agents=traex,trae-ide
2026-06-16T17:30:01+08:00 | file_scanned | agent=traex path=/Users/.../history.jsonl from_line=129 to_line=146 records=18
2026-06-16T17:30:02+08:00 | pending_created | path=/.../pending/2026-06-16-173000-scan.json records=86
2026-06-16T17:38:12+08:00 | committed | pending=/.../scan.json review_doc=https://... snapshot=/.../reviews/2026-06-16-173000-candidates.json
```

候选快照默认放在：

```text
<wiki_root>/candidate/codeagent/reviews/2026-06-16-173000-candidates.json
```

快照包含：

- `review_doc_url`
- `pending_path`
- 全量候选字段
- 文档协议版本
- 创建时间

快照仅用于审计和排障，不作为入库准入凭证。准入凭证只有飞书文档中的已勾选候选。

## 候选抽取规则

首期采用平衡召回：候选不必已经确定入库，但必须有明确复用价值，并说明原因、证据、建议分类、风险与脱敏情况。

默认 8 类候选：

1. 工具/产品使用说明。
2. 工作流/流程规范。
3. 项目规则/团队约定。
4. 可复用问题排查经验。
5. 外部资料索引。
6. 设计决策/架构知识。
7. 命令与配置片段。
8. 用户明确给出的知识材料。

不建议抽取：

- 一次性任务状态。
- 临时计划或临时 TODO。
- 未验证猜测。
- 纯操作流水。
- 纯寒暄。
- 大段代码 diff。
- 与当前项目强绑定且无复用价值的细节。
- 含敏感信息且无法有效脱敏的内容。
- agent 自我评价或笼统总结。
- 只描述“做了什么”但没有可复用知识的 session 记忆。

## 脱敏规则

采用中等脱敏。

必须遮罩：

- token、password、private key、secret、cookie、authorization header。
- 手机号、邮箱、个人账号等个人信息。
- 绝对本地路径中的用户名，例如 `/Users/bytedance/...` 改为 `/Users/<user>/...`。
- 明显内部密钥、长随机 ID、访问凭据。
- URL query 中的 token、key、signature 等敏感参数。

可以保留：

- 外部 URL 原始链接。
- 飞书文档 URL 原始链接。
- 公开 API 文档链接。
- 通用命令名、参数名、配置字段名。
- 项目相对路径。
- 非敏感错误信息和日志路径模式。

内部 URL 不一刀切删除。保留必要上下文，遮罩敏感 query 和过长 ID。如果链接本身是凭据或临时授权链接，则整条替换为 `<redacted-url>`。

## 候选字段

每条候选至少包含：

```yaml
candidate_id: CAND-001
slug: openwiki-runtime-discovery
title: OpenWiki 配置发现规则
category: 工具/产品使用说明
target_wiki_area: wiki/pages 或 concepts 或 entities
reason: 为什么值得沉淀
proposed_content: 拟入库内容摘要
evidence:
  - agent: traex
    source_file: /Users/<user>/.trae/cli/history.jsonl
    line_start: 129
    session_id: 019e...
    message_id: ""
    timestamp: 2026-06-16T16:20:00+08:00
    quote: 原文片段
risk_and_redaction:
  - 已遮罩本地用户名
original_links:
  - https://example.com/...
```

slug 规则复用 `wiki-ingest/references/slug-rules.md`：lowercase、hyphen-separated、无特殊字符；中文标题翻译语义后 slugify，不默认拼音。

## 飞书审核文档协议

文档顶部必须包含：

```text
OPENWIKI_CANDIDATE_REVIEW_DOC v1
source: codeagent
admission: ONLY_CHECKED_CANDIDATES
```

并醒目说明：

> 只有已勾选候选项允许被 `wiki-ingest` 入库。未勾选候选项表示用户明确拒绝入库，`wiki-ingest` 不得总结、合并、引用或作为背景材料使用。

展示形式采用分组卡片式：

```text
# OpenWiki 候选知识审核 - CodeAgent
协议说明
本次扫描摘要
如何审核
候选总览
分类章节 1
  候选卡片
分类章节 2
  候选卡片
下一步
```

每条候选是独立卡片，标题行有唯一准入 checkbox：

```text
☐ CAND-001｜openwiki-runtime-discovery｜OpenWiki 配置发现规则
```

卡片包含：

- 建议分类。
- 建议入库位置。
- 拟 slug。
- 为什么值得沉淀。
- 拟入库内容。
- 证据。
- 原始链接。
- 风险与脱敏。

解析约束：

- 只有候选卡片标题行的 checkbox 是准入信号。
- 卡片内部 checkbox 必须忽略。
- 候选块从标题行开始，到下一个候选标题行或下一个分类标题前结束。
- `CAND-xxx` 必须唯一。
- slug 必须明确出现。
- `wiki-ingest` 只读取 checked 候选块。

优先使用 `lark-doc` v2 DocxXML 创建真实 checkbox block。如果 checkbox XML 表达不稳定，可降级为 `[ ]` / `[x]` 文本标记，但首选真实 checkbox。

## `wiki-ingest` 协作契约

当 `wiki-ingest` 收到飞书文档链接时，应先用 `lark-doc` 读取足够内容以检测协议标记。

如果检测到：

```text
OPENWIKI_CANDIDATE_REVIEW_DOC v1
admission: ONLY_CHECKED_CANDIDATES
```

必须读取 `skill/wiki-ingest/references/candidate-review-doc.md` 并严格执行：

1. 使用 DocxXML 判断候选标题行 checkbox 状态。
2. 只提取 checked 候选卡片。
3. 未 checked 候选不得总结、合并、引用或作为背景材料。
4. 如果没有 checked 候选，停止，不写 wiki。
5. checked 候选代表用户已确认入库，`wiki-ingest` 不再执行“是否确认写入”的二次确认。
6. 对 checked 候选直接继续现有入库流程：检查相关 wiki 页面、生成或复用 slug、写入/更新页面、更新分层索引、记录日志、验证。

如果文档没有协议标记，则按普通来源继续现有 ingest 流程。

如果协议不完整，例如只有 `OPENWIKI_CANDIDATE_REVIEW_DOC v1` 但没有 `ONLY_CHECKED_CANDIDATES`，必须停止，不得按普通文档 ingest。

如果 checkbox 状态无法可靠判断，必须停止，不得入库。

## 错误处理

### `wiki-candidate`

- 配置不存在：复用 CLI `CONFIG_NOT_FOUND`，提示用户提供 `--config` 或先运行 `wiki-init`。
- 没有新增 records：不创建飞书文档，提示可扩大范围。
- 扫描文件不存在：记录 warning；全部路径不存在时返回 0 records 和 warnings。
- 单行 JSONL 解析失败：跳过该行，记录 warning，不中断整体扫描。
- 大量 JSONL 失败：degraded success，skill 提醒用户。
- 飞书文档创建失败：不得 commit pending，报告 pending 路径和错误。
- 候选快照保存失败：不得 commit pending。
- commit 失败：报告飞书文档已创建、快照已保存、状态未推进；提示可手工重试 commit。

### `wiki-ingest`

- 有协议但没有 checked 候选：停止并说明没有内容会进入知识库。
- 协议不完整：停止并要求重新生成或提供完整审核文档。
- checkbox 解析不确定：停止并提示改用文本 `[x]` 标记版本或重新生成。
- checked 候选缺少 slug：可重新生成 slug，并在报告中说明。
- checked 候选缺少 evidence：可入库但标记证据不足；由于 checkbox 已代表确认，不再二次询问是否继续。
- checked 候选缺少 proposed content：停止处理该候选。

## 测试策略

### Go CLI 单元测试

覆盖：

1. candidate 配置默认值解析。
2. candidate 配置覆盖。
3. traex history JSONL 解析。
4. trae-ide session_memory JSONL 解析。
5. 首次运行 `initial_days` 限制。
6. `max_records_per_run` 限制。
7. append 文件从 offset 续扫。
8. truncate 文件重新扫描。
9. pending 文件包含 `state_updates`。
10. commit 原子推进状态。
11. commit 要求 `review_doc_url` 和 `snapshot`。
12. warning 不导致整体失败。

### Go CLI 集成测试

使用临时目录模拟：

```text
tmp/
  openwiki.toml
  wiki_root/
  sessions/
    history.jsonl
    projects/.../session_memory_x.jsonl
```

执行：

```bash
go run ./cmd/openwiki --config tmp/openwiki.toml candidate codeagent scan --json
go run ./cmd/openwiki --config tmp/openwiki.toml candidate codeagent commit --pending ... --review-doc-url ... --snapshot ... --json
```

验证 pending 创建、scan 阶段 state 不推进、commit 后 state 推进、run.log 追加。

### Skill 文档验证

`wiki-candidate` 验证：

- 是否强制调用 CLI 而不是手动扫描。
- 是否读取 extraction rules。
- 是否生成协议标记。
- 是否保留原始外部 URL / 飞书 URL。
- 是否只有飞书文档和快照成功后才 commit。

`wiki-ingest` 验证：

- 遇到 Candidate Review Doc 时不走普通全文 ingest。
- 只处理 checked 候选。
- 未 checked 候选不得作为背景材料。
- checked 候选不触发二次入库确认。

### 手工端到端验证

1. 创建临时 `history.jsonl`，包含工具说明、外部 URL、一次性任务进展、含 token 敏感片段。
2. 运行 `wiki-candidate`。
3. 确认飞书文档有协议标记、候选按分类分组、外部 URL 保留、token 被遮罩、一次性任务没有候选。
4. 勾选一条候选。
5. 把飞书链接交给 `wiki-ingest`。
6. 确认只有勾选候选入库，未勾选候选未被引用，且不进行二次确认。
