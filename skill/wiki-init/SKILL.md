---
name: wiki-init
description: Use when bootstrapping a new project-local OpenWiki instance backed by openwiki.toml and layered indexes.
---
# Wiki Init

Bootstrap a project-local OpenWiki runtime.

## Runtime Contract

- `openwiki.toml` is the only canonical runtime contract.
- `WIKI.md` is not used as the runtime contract.
- The default config location is the current project directory.
- The default wiki root is `./openwiki/`.
- `wiki/index.md` is the lightweight Routing Index.
- `wiki/indexes/` contains Shard Indexes used for lookup and maintenance.

## Process

### 1. Confirm target

If the user does not specify a wiki root, use:

```text
./openwiki/
```

If `openwiki.toml` already exists in the current directory, treat the directory as an existing OpenWiki project. Do not overwrite unless the user explicitly asks for force reinitialization.

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
<project>/
├── openwiki.toml
└── openwiki/
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

- Configuration file: `openwiki.toml`
- Wiki root: resolved `wiki_root`
- Routing index: `wiki/index.md`
- Shard indexes: `wiki/indexes/`
- Next steps: add sources to `raw/`, use `wiki-ingest`, use `wiki-query`, run `openwiki status` periodically.
