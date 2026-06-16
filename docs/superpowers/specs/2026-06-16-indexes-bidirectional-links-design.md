# index.md 分片索引双向链接设计

## 目标

生成的 `wiki/index.md` 中，对 `wiki/indexes/` 下分片索引页面的引用必须使用双向链接，格式为 `[[indexes/<name>]]`。

## 范围

本次只调整顶层 `wiki/index.md` 中指向分片索引的引用展示：

- `indexes/scopes.md` → `[[indexes/scopes]]`
- `indexes/entities.md` → `[[indexes/entities]]`
- `indexes/concepts.md` → `[[indexes/concepts]]`
- `indexes/tags.md` → `[[indexes/tags]]`
- `indexes/recent.md` → `[[indexes/recent]]`
- `indexes/hot.md` → `[[indexes/hot]]`

## 修改点

- `internal/wiki/init.go`：更新 `indexTemplate`，保证 `openwiki init` 新建的 `wiki/index.md` 使用双向链接。
- `internal/wiki/index.go`：更新 `buildRoutingIndex`，保证 `openwiki index rebuild` 重建的 `wiki/index.md` 使用双向链接。
- `internal/wiki/init_test.go` 与 `internal/wiki/index_test.go`：增加或调整测试，验证初始化和重建输出都使用 `[[indexes/<name>]]`，且不再输出反引号包裹的 `indexes/*.md` 路径。

## 不修改

- 不改分片索引目录结构和文件名。
- 不改普通页面链接格式 `[[slug]]`。
- 不改 `CheckIndex` 的覆盖率检查逻辑。
- 不改 `wiki/indexes/*.md` 内部页面索引生成逻辑。

## 测试策略

采用测试先行：

1. 先为 `Init` 的 `wiki/index.md` 输出添加断言，确认需要的双向链接存在、旧路径格式不存在。
2. 运行测试确认失败。
3. 修改初始化模板。
4. 运行测试确认通过。
5. 再为 `RebuildIndex` 的 `wiki/index.md` 输出添加同类断言。
6. 运行测试确认失败。
7. 修改重建模板。
8. 运行相关测试和完整 Go 测试。
