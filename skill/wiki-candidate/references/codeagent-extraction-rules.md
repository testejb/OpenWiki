# CodeAgent 候选抽取规则

这些规则用于从 `openwiki candidate codeagent scan --json` 生成的 pending 记录中抽取 OpenWiki 候选知识。不要直接扫描原始 codeagent 文件；pending 记录是唯一输入。

## 抽取目标

保持平衡召回：宁可保留可能有复用价值的候选供用户审核，也不要把低信号、重复或不可靠内容全部塞入审核文档。候选不是正式 wiki 内容，只有审核文档中被勾选的候选才能进入后续 ingest/update。

## 8 类候选

只允许使用以下 8 类，不要新增计划外类别：

1. **工具/产品使用说明**：工具、产品、插件、MCP、CLI 或平台能力的使用方法、限制和注意事项。
2. **工作流/流程规范**：跨任务可复用的操作流程、审核流程、发布流程、检查清单或协作步骤。
3. **项目规则/团队约定**：仓库规则、团队约定、提交规范、目录约定、运行约束或评审要求。
4. **可复用问题排查经验**：明确错误现象、根因、验证过的修复方法、诊断命令和避免复发的检查点。
5. **外部资料索引**：有长期价值的 external URL、Feishu URL、issue、PR/MR、文章或文档入口及上下文说明。
6. **设计决策/架构知识**：方案选择、取舍、边界、接口设计、数据模型、架构约束和后续影响。
7. **命令与配置片段**：可复用命令、配置字段、JSON/TOML/YAML 片段、协议头、脚本入口或参数组合。
8. **用户明确给出的知识材料**：用户直接提供、要求沉淀或可作为知识来源的文字、文档、会议纪要、素材或结论。

## Do Not Extract

不要抽取以下内容：

- 闲聊、寒暄、情绪表达、无行动价值的状态更新。
- 只对一次性本地临时路径、临时文件名、终端窗口状态有效的信息。
- 未验证的猜测、模型自我反思、没有证据的结论。
- 重复内容；仅保留最完整、来源最清晰的一条。
- 大段代码、日志或 diff；只提炼可复用规则，并保留可追溯来源。
- 与 OpenWiki、项目知识沉淀、工具流程无关的普通实现细节。
- 用户明确表示不要记录、不要外传、仅当前会话临时使用的信息。
- 任何密钥、令牌、Cookie、私钥、个人隐私或内部敏感凭据。

## Redaction

必须在写入 Feishu 审核文档、snapshot 或候选输出前脱敏。对以下内容使用 `[REDACTED]` 或保留结构后局部脱敏：

- token、password、private key、secret、cookie、auth header、Authorization header。
- 手机号、邮箱、个人账号、个人身份标识。
- 绝对本地路径中的用户名，例如 `/Users/zhangsan/...` 应改写为 `/Users/<user>/...` 或等价占位，不暴露真实用户名。
- 明显内部凭据、长随机 ID、访问票据、会话 ID、临时签名值。
- URL query 参数中名为或疑似 `token`、`key`、`signature`、`auth`、`password`、`secret`、`cookie`、`session` 的值。

脱敏时遵守：

- 保留判断上下文需要的非敏感结构，例如变量名、配置键名、错误类型。
- 不要脱敏普通来源链接本身，除非链接包含凭据或敏感查询参数。
- 明确保留 external URL 和 Feishu URL 的原始链接；如果链接含敏感参数，只脱敏敏感参数并说明已脱敏。

## Candidate Field Requirements

每个候选必须使用以下计划字段：

- `candidate_id`：稳定 ID，例如 `CAND-001`；同一审核文档内唯一。
- `slug`：英文小写短横线 slug，例如 `openwiki-runtime-discovery`；用于候选标题行和后续定位。
- `title`：简洁中文标题，说明候选知识点。
- `category`：只能使用上述 8 类之一。
- `target_wiki_area`：建议进入的 wiki 区域，例如 `wiki/pages`、`entities`、`concepts`、`indexes`、`unknown`。
- `reason`：为什么值得进入审核，说明复用价值或沉淀原因。
- `proposed_content`：建议写入 wiki 的候选内容草稿，使用中文，避免未经审核的绝对化表述。
- `evidence`：来自 pending 记录的证据摘要或引用定位，避免粘贴大段原文。
- `risk_and_redaction`：风险、敏感信息处理、脱敏说明；无风险也写 `none`。
- `original_links`：原样保留 external URL 和 Feishu URL 原始链接；没有则为空列表。

## 链接保留要求

- external URL 必须保留原始链接，不改写、不短链化、不翻译。
- Feishu URL 必须保留原始链接，不替换为标题或截图。
- 同一候选有多个来源链接时全部列出，除非完全重复。
- 如果 URL query 含敏感参数，只脱敏敏感参数值，保留 URL 的来源可识别性。
