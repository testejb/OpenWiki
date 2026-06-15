---
name: wiki-init
description: Use when bootstrapping a new project-local OpenWiki instance backed by openwiki.toml and layered indexes.
---
# Wiki Init

Bootstrap a project-local OpenWiki runtime.

## Runtime Contract

- `openwiki.toml` is the only canonical runtime contract.
- `WIKI.md` is not used as the runtime contract.
- `openwiki init` currently creates `openwiki.toml` inside the initialized wiki root.
- Use `--config <wiki-root>/openwiki.toml` when a command needs an explicit config path.
- Project-local root config (`<project>/openwiki.toml` pointing at `./openwiki/`) is the target direction, but do not promise that layout until the CLI semantics provide it.
- `wiki/index.md` is the lightweight Routing Index.
- `wiki/indexes/` contains Shard Indexes used for lookup and maintenance.


## CLI Index Command Guardrail

Before running `openwiki index check` or `openwiki index rebuild`, verify that the selected CLI supports index commands:

```bash
openwiki --help | grep -q "index"
```

If the global `openwiki` CLI is outdated or does not list `index`, use the repository-built CLI from the repository root instead:

```bash
go run ./cmd/openwiki --help | grep -q "index"
go run ./cmd/openwiki index check
go run ./cmd/openwiki index rebuild
```

If neither command exposes `index`, report that the OpenWiki CLI version is too old and do not imply that index commands are available.

## Process

### 1. Confirm target

If the user does not specify a wiki root, use a project-local wiki root:

```text
./openwiki/
```

With the current CLI, the expected config path is `<wiki-root>/openwiki.toml` after initialization. If `openwiki.toml` already exists in the chosen wiki root or a user-provided config path, treat it as an existing OpenWiki instance. Do not overwrite unless the user explicitly asks for force reinitialization.

When reusing an existing project:

- Read `openwiki.toml` first.
- Resolve `wiki_root` relative to the config file location when needed.
- Do not ask again for fields already present in `openwiki.toml`.
- Validate that the layered index layout exists before continuing.

### 2. Initialize with CLI

```bash
openwiki init --non-interactive --json
```

For a custom wiki root:

```bash
openwiki init <wiki-root> --non-interactive --json
```

For force repair or overwrite of config only:

```bash
openwiki init <wiki-root> --force --non-interactive --json
```

`--force` must not be treated as permission to delete existing wiki data.

### 3. Expected layout

```text
<wiki-root>/
├── openwiki.toml
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

A later project-local wrapper may place `<project>/openwiki.toml` beside `./openwiki/`; until then, document and use the actual CLI output path.

`wiki/index.md` must stay lightweight: it routes queries to shard indexes and must not contain a full all-page catalog.

`wiki/indexes/` contains Shard Indexes:

- `scopes.md` routes by `scope_level` and `scope_code`.
- `entities.md` routes entity pages under `entities/`.
- `concepts.md` routes concept/report pages under `concepts/`.
- `tags.md` routes by tags.
- `recent.md` routes recently updated content.
- `hot.md` routes frequently cited or queried content.
- `query-usage.jsonl` stores one query-usage JSON object per line.

**Critical:** `wiki/pages/` is flat. Summary pages live there as `<slug>.md`. Slugs are lowercase and hyphen-separated.

### 4. Validate

Run:

```bash
openwiki config validate
openwiki status
openwiki index check
```

If required shard indexes are missing or inconsistent, recommend:

```bash
openwiki index rebuild
```

### 5. Confirm

Tell the user:

- Configuration file: `<wiki-root>/openwiki.toml` for current CLI output, or the explicit `--config` path when supplied
- Wiki root: resolved `wiki_root`
- Routing index: `wiki/index.md`
- Shard indexes: `wiki/indexes/`
- Next steps: add sources to `raw/`, use `wiki-ingest`, use `wiki-query`, run `openwiki status` periodically.
