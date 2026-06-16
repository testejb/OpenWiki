# Review Doc Protocol

Feishu 审核文档必须可由后续工具或人工按固定协议解析。候选文档不是正式 wiki；它只表达哪些候选被用户允许进入后续 ingest/update。

## 协议头

文档开头必须包含以下三行，保持原文大小写：

```text
OPENWIKI_CANDIDATE_REVIEW_DOC v1
source: codeagent
admission: ONLY_CHECKED_CANDIDATES
```

`ONLY_CHECKED_CANDIDATES` 表示只有被勾选的候选可以进入后续正式写入流程。Unchecked 是 explicit user rejection，不是“待定接受”。

## Layout

推荐结构：

1. 协议头。
2. 简短说明：请用户勾选允许进入 OpenWiki 的候选；未勾选将视为拒绝。
3. 扫描摘要：pending path/ID、扫描记录数、候选数、生成时间、snapshot path。
4. 候选列表：每个候选使用独立 candidate card。
5. 解析说明：不要删除协议头；如复选框不可用，可使用 `[x]` fallback。

## Candidate Card

每个候选卡片必须包含稳定 ID 和可解析勾选状态。推荐 Markdown 形态：

```markdown
### CANDIDATE <id>: <title>

- [ ] admit
- Category: <one of 8 categories>
- Confidence: high|medium|low
- Suggested destination: <path-or-unknown>
- Source ref: <pending/source ref>
- Source URLs:
  - <original external URL or Feishu URL>
- Redactions: none|<redaction summary>

摘要：<1-3 sentence Chinese summary>

证据：<short evidence summary>
```

如果 Feishu 原生 checkbox 无法稳定导出，使用文本 fallback：

```markdown
- [x] admit
```

其中 `[x]` 表示用户明确勾选，`[ ]`、缺失、删除、空白或任何非 `[x]` 状态都视为 unchecked。

## Parse Contract

解析器只接受满足以下条件的候选：

- 文档包含协议头 `OPENWIKI_CANDIDATE_REVIEW_DOC v1`。
- `source: codeagent`。
- `admission: ONLY_CHECKED_CANDIDATES`。
- 候选卡片包含 `CANDIDATE <id>`。
- 候选卡片的 admit 状态为 Feishu checkbox checked，或 fallback 文本为 `[x] admit`。

解析器必须拒绝或忽略以下内容：

- 未勾选候选；unchecked 是 explicit user rejection。
- 无协议头、协议头被改写、source 不匹配或 admission 不是 `ONLY_CHECKED_CANDIDATES` 的文档。
- 没有稳定 ID 的候选。
- 被用户删除的候选。
- 看似同意但没有 checkbox checked 或 `[x] admit` 的自然语言评论。

## Fallback `[x]`

当 Feishu 复选框无法由 API 稳定读取时，使用纯文本 `[x] admit` 作为唯一接受标记。不要接受 `yes`、`Y`、`同意`、`通过`、emoji 或评论回复作为替代标记。
