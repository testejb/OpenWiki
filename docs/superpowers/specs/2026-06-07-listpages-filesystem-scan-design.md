# ListPages 改为文件系统扫描 设计文档

**日期:** 2026-06-07
**问题:** `openwiki status` 只解析 `index.md` 表格，当 index.md 格式不匹配时返回 0 页，而非反映实际存在的页面文件数量。

## 目标

将 `wiki.ListPages` 从解析 `index.md` 表格改为扫描文件系统目录（`wiki/pages/`、`entities/`、`concepts/`），读取每个 `.md` 文件的 frontmatter 提取元信息。

## 架构

修改仅涉及一处：

- `internal/wiki/page.go`：重写 `ListPages` 函数，新增 `listPagesFromFS` 实现

## 详细设计

### ListPages 新逻辑

```
ListPages(fs, root):
  1. 遍历 pageDirs 中的三个目录: wiki/pages, entities, concepts
  2. 对每个目录，用 fs.ReadDir 列出所有 .md 文件
  3. 对每个 .md 文件:
     a. 读取文件内容
     b. 解析 YAML frontmatter（复用已有的 parsePage 或内联解析）
     c. 提取 title, tags, scope_level, scope_code, updated
     d. 构造 PageMeta：slug=文件名去.md, type=目录对应类型
  4. 返回 []PageMeta
```

### 错误处理

- 目录不存在 → 跳过该目录，不返回错误
- 单个文件读取失败或 YAML 解析失败 → 跳过该文件，继续处理其他文件
- 所有目录都不存在 → 返回空数组 + nil（不是错误）

### frontmatter 字段映射

| frontmatter 字段 | PageMeta 字段 | 类型 |
|---|---|---|
| `title` | `Title` | string |
| `tags` | `Tags` | `[]string`（从 YAML 数组转换） |
| `scope_level` | `ScopeLevel` | string |
| `scope_code` | `ScopeCode` | string |
| `updated` | `Updated` | string（保持原始格式，用于过期检测） |

### 函数签名不变

`ListPages(fs FS, root string) ([]PageMeta, error)` 保持完全不变，所有调用方（`status.go`、`page.go` 的 `runPageList`）无需修改。

## 影响分析

- **优点**: 不依赖 index.md 格式，始终反映实际文件数量；index 与文件不一致时以文件为准
- **缺点**: 需遍历所有文件，对百/千级页面可忽略
- **兼容性**: 所有现有测试需要更新 — 测试不再需要在 MemFS 中构建 index.md 表格，只需创建 `.md` 文件即可

## 测试策略

- 更新 `TestListPages`：不再依赖 index.md 内容，改为在 `wiki/pages/` 目录下创建 `.md` 文件
- 更新 `TestListPagesWithType`：在三个目录下分别创建文件
- 新增 `TestListPagesEmptyDir`：目录不存在时返回空
- 新增 `TestListPagesSkipsBadFiles`：单个文件损坏时跳过
- 更新 `cli/status_test.go` 的 `setupTestWiki`：去除 index.md 写入，保留文件创建