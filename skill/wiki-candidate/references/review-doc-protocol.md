# Review Doc Protocol

Feishu 审核文档必须可由后续工具或人工按固定协议解析。候选文档不是正式 wiki；它只表达哪些候选被用户允许进入后续 ingest/update。

## 固定标题

Feishu 文档标题必须是：

```text
OpenWiki 候选知识审核 - CodeAgent
```

## 协议 block

文档开头标题后必须包含以下三行协议 block，保持原文大小写：

```text
OPENWIKI_CANDIDATE_REVIEW_DOC v1
source: codeagent
admission: ONLY_CHECKED_CANDIDATES
```

`ONLY_CHECKED_CANDIDATES` 表示只有标题行被勾选的候选可以进入后续正式写入流程。Unchecked 是 explicit user rejection，不是“待定接受”。

## Layout

文档必须按以下布局组织：

1. 固定标题：`OpenWiki 候选知识审核 - CodeAgent`。
2. 协议 block：包含 `OPENWIKI_CANDIDATE_REVIEW_DOC v1`、`source: codeagent`、`admission: ONLY_CHECKED_CANDIDATES`。
3. 扫描摘要：pending path/ID、state path（如已知）、扫描记录数、候选数、生成时间、snapshot path。创建前摘要不含 review doc URL；创建后可在 snapshot 和 final report 记录 URL，若要写回文档则作为可选后续更新，不作为必需。
4. How to review：说明只勾选允许进入 OpenWiki 的候选；未勾选就是拒绝；不要删除协议 block；如复选框不可用，使用标题行 `[x]` fallback。
5. 候选总览：候选 ID、slug、title、category、target_wiki_area 的表格或列表。
6. 分类章节：按 8 类候选分组；没有候选的分类可以省略。
7. 候选卡片：每个候选使用固定 candidate card，标题行 checkbox 是唯一 admission signal。
8. Next step：用户完成勾选后，后续 ingest/update 只读取 checked candidates，并继续保留原始链接和脱敏记录。

## Candidate Card

每个候选卡片必须包含稳定 ID、slug、标题和可解析勾选状态。标题行 checkbox 是唯一 admission signal；body checkbox is ignored。正文不能使用 `- [ ] admit`、`- [x] admit` 或任何正文 admit 字段作为准入信号。

Feishu 原生复选框标题行格式：

```markdown
☐ CAND-001｜openwiki-runtime-discovery｜OpenWiki 配置发现规则
```

正文使用计划字段：

```markdown
- Category: 工作流/流程规范
- Target wiki area: wiki/pages
- Reason: 该规则影响所有 OpenWiki 技能的运行时发现，复用价值高。
- Proposed content: 使用 openwiki.toml 作为 runtime contract，先执行 config path/show。
- Evidence: pending 记录中的配置发现流程说明。
- Risk and redaction: none
- Original links:
  - https://example.com/original
```

标题行处于 checked 状态才表示准入；正文中的评论、文字“同意”、风险说明或其他勾选项都不能作为准入信号。

## Parse Contract

解析器只接受满足以下条件的候选：

- 文档标题为 `OpenWiki 候选知识审核 - CodeAgent`。
- 文档包含协议头 `OPENWIKI_CANDIDATE_REVIEW_DOC v1`。
- `source: codeagent`。
- `admission: ONLY_CHECKED_CANDIDATES`。
- 候选卡片标题行包含 `CAND-<number>｜<slug>｜<title>`。
- 候选卡片标题行的 Feishu checkbox 为 checked，或 fallback 标题行为 `[x] CAND-002｜slug｜title`。

解析器必须拒绝或忽略以下内容：

- 未勾选候选；unchecked 是 explicit user rejection。
- 无固定标题、无协议头、协议头被改写、source 不匹配或 admission 不是 `ONLY_CHECKED_CANDIDATES` 的文档。
- 没有稳定 `candidate_id` 或 `slug` 的候选。
- 被用户删除的候选。
- 正文 `- [ ] admit`、`- [x] admit`、任何正文 admit 字段或任何 body checkbox。
- 看似同意但没有标题行 checkbox checked 或标题行 `[x]` fallback 的自然语言评论。

## Fallback `[x]`

当 Feishu 标题行复选框无法由 API 稳定读取时，使用纯文本标题行 `[x] CAND-002｜slug｜title` 作为唯一接受标记。例如：

```markdown
[x] CAND-002｜openwiki-cli-config｜OpenWiki CLI 配置命令
```

未勾选 fallback 使用 `[ ] CAND-002｜slug｜title`。不要接受正文 `[x] admit`、`yes`、`Y`、`同意`、`通过`、emoji 或评论回复作为替代标记。

## Admission Source Rule

Snapshot is not an admission source. It only records what was sent to Feishu for traceability. Admission must be parsed from checked title lines in the Feishu review document; body checkbox markers are ignored, title checkbox only. Unchecked candidates are explicit rejection.
