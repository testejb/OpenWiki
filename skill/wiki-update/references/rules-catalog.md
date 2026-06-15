# Lint 规则目录

本文档包含 wiki-lint 所有检查规则的详细定义。SKILL.md 正文仅保留规则名称和简要说明，完整定义见此处。

---

## Red Errors

### broken-links

**触发条件**: `wiki/pages/`、`entities/`、`concepts/` 中的页面存在 `[[slug]]` 交叉引用，但该 slug 在三类页面目录中均不存在。

**检查逻辑**:
1. 扫描 `wiki/pages/*.md`、`entities/*.md`、`concepts/*.md` 中的 `[[...]]` 引用
2. 对每个引用，检查 `wiki/pages/<slug>.md`、`entities/<slug>.md`、`concepts/<slug>.md` 是否存在
3. 三处均不存在的引用标记为断链

**修复建议**: 创建缺失的页面，或修正 slug 拼写，或移除无效引用。

---

### missing-frontmatter

**触发条件**: `wiki/pages/*.md`、`entities/*.md`、`concepts/*.md` 文件缺少 YAML frontmatter（`---...---` 包裹的元数据块）。

**检查逻辑**:
1. 读取每个页面文件的前 5 行
2. 检查是否以 `---` 开头
3. 若无 frontmatter，标记为缺失

**修复建议**: 添加包含 `title`、`tags`、`sources`、`updated`、`scope_level`、`scope_code` 的 frontmatter。

---

## Yellow Warnings

### orphan-pages

**触发条件**: 页面存在于 `wiki/pages/`、`entities/` 或 `concepts/` 中，但没有任何其他页面引用它，且没有出现在对应的 Shard Index 中。顶层 `wiki/index.md` 是 Routing Index，不作为页面收录依据。

**检查逻辑**:
1. 构建所有页面的被引用计数
2. 读取 `wiki/indexes/scopes.md`、`wiki/indexes/entities.md`、`wiki/indexes/concepts.md`、`wiki/indexes/tags.md`、`wiki/indexes/recent.md`
3. 标记被引用计数为 0 且未出现在适当 Shard Index 中的页面

**修复建议**: 在相关页面中添加交叉引用，更新对应 Shard Index，或运行 `openwiki index rebuild`。不要把页面注册到顶层 `wiki/index.md`。

---

### contradictions

**触发条件**: 两个或多个页面对同一概念给出了相互矛盾的描述。

**检查逻辑**:
1. 比较具有相同或相似标签的页面
2. 识别对同一主题的矛盾声明
3. 标注矛盾的具体内容

**修复建议**: 确认哪个描述是正确的，更新或合并矛盾页面。

---

### stale-claims

**触发条件**: 页面 `updated` 日期距今超过 6 个月，且内容可能已过时。

**检查逻辑**:
1. 解析每个页面的 `updated` 字段
2. 计算距今天数
3. 超过 180 天的标记为过时

**修复建议**: 审查页面内容，更新过时信息，或标记为历史存档。

---

### content-not-chinese-primary

**触发条件**: 页面正文中文占比低于 60%。

**排除内容**:
- 代码块（` ```...``` `）
- 行内代码（`` `code` ``）
- URL（`https://...`）
- YAML frontmatter（`---...---`）
- 术语首次标注括号内英文（`中文术语（English）`）

**检查逻辑**:
1. 提取页面正文（排除上述内容）
2. 统计中文字符占比
3. 低于 60% 时触发

**启用条件**: `openwiki.toml` 中 `primary_language` 为 `zh`。

**修复建议**: 将英文描述翻译为中文，或为英文内容添加中文摘要。

---

### missing-chinese-title

**触发条件**: 页面 h1 标题（第一个 `# ` 开头的行）不包含任何中文字符。

**检查逻辑**:
1. 提取页面的第一个 `# ` 标题
2. 检查是否包含 Unicode 中文字符（`\u4e00-\u9fff`）
3. 不包含时触发

**注意**: 仅检查 Markdown h1，不检查 frontmatter 中的 `title` 字段。

**启用条件**: `openwiki.toml` 中 `primary_language` 为 `zh`。

**修复建议**: 将 h1 标题翻译为中文，或在英文标题后附加中文翻译。

---

### missing-term-glossary

**触发条件**: 英文多词术语（2 个及以上单词）在页面中首次出现时未附中文解释。

**支持格式**:
- `中文术语（English Term）`
- `English Term（中文术语）`

**豁免**: 单字术语（如 "Go"、"Rust"）不触发。

**检查逻辑**:
1. 扫描页面正文中的英文多词短语
2. 检查首次出现时是否附带中文解释
3. 未附带时触发

**启用条件**: `openwiki.toml` 中 `primary_language` 为 `zh`。

**修复建议**: 在术语首次出现时添加中文解释，如 `依赖注入（Dependency Injection）`。

---

### missing-bilingual-tags

**触发条件**: frontmatter 中 `tags` 仅有英文标签，无中文标签。

**检查逻辑**:
1. 解析 frontmatter 的 `tags` 字段
2. 检查是否存在中文字符的标签
3. 无中文标签时触发

**启用条件**: `openwiki.toml` 中 `primary_language` 为 `zh` 且 `secondary_language` 为 `en`。

**修复建议**: 为每个英文标签添加对应的中文标签。

---

### missing-scope-fields

**触发条件**: 页面 frontmatter 缺少 `scope_level` 或 `scope_code` 字段。

**检查逻辑**:
1. 解析每个页面的 frontmatter
2. 检查 `scope_level` 和 `scope_code` 是否存在
3. 任一缺失时触发

**注意**: 向后兼容，Yellow Warning 级别。

**修复建议**: 添加 `scope_level`（repo/domain/company/industry/wisdom）和 `scope_code`（slug 格式）。

---

### invalid-scope-level

**触发条件**: `scope_level` 不在合法枚举值中。

**合法值**: `repo`、`domain`、`company`、`industry`、`wisdom`

**检查逻辑**:
1. 解析 frontmatter 的 `scope_level`
2. 检查是否在合法枚举值中
3. 不在时触发

**修复建议**: 将 `scope_level` 修正为合法值之一。

---

### invalid-scope-code-format

**触发条件**: `scope_code` 不符合 slug 规则。

**规则**: 全小写、连字符分隔、无特殊字符，中文代号须翻译为英文。

**检查逻辑**:
1. 解析 frontmatter 的 `scope_code`
2. 检查是否符合 slug 规则
3. 不符合时触发

**修复建议**: 将 `scope_code` 修正为符合 slug 规则的格式。

---

### scope-level-code-mismatch

**触发条件**: `scope_level` 为 `wisdom` 但 `scope_code` 不为 `"wisdom"`。

**检查逻辑**:
1. 解析 frontmatter 的 `scope_level` 和 `scope_code`
2. 若 `scope_level == "wisdom"` 且 `scope_code != "wisdom"`，触发

**修复建议**: 将 `scope_code` 设为 `"wisdom"`。

---

## Blue Info

### missing-concept-pages

**触发条件**: 页面中引用了某个概念，但该概念没有对应的 wiki 页面。

**检查逻辑**:
1. 扫描所有页面中的概念引用
2. 检查是否有对应的页面存在
3. 不存在的标记为缺失概念页

**修复建议**: 为缺失的概念创建页面，或使用 wiki-ingest 摄入相关源。

---

### missing-cross-references

**触发条件**: 两个页面讨论相关主题，但彼此没有交叉引用。

**检查逻辑**:
1. 比较页面的标签和内容相似度
2. 识别相关但未互链的页面对
3. 标记缺失的交叉引用

**修复建议**: 在相关页面中添加 `[[slug]]` 交叉引用。

---

### hardcoded-or-literal-today

**触发条件**: 生成文件中残留字面量 `<today>` 或使用明显非当前日期的硬编码日期。

**检查范围**: `wiki/pages/*.md`、`entities/*.md`、`concepts/*.md`、`wiki/index.md`、`wiki/indexes/*.md`、`wiki/indexes/query-usage.jsonl`、`wiki/log.md`

**检查逻辑**:
1. 扫描生成文件内容
2. 检查是否包含字面量 `<today>`
3. 检查日期字段是否与当前日期偏差过大

**修复建议**: 将 `<today>` 替换为实际当前日期，更新过时日期。
