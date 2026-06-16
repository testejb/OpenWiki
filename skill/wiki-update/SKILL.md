---
name: wiki-update
description: Use when revising existing wiki pages because knowledge has changed, a new piece of information updates or contradicts existing content, or the user wants to directly edit wiki content with LLM assistance.
composes: [wiki-ingest, wiki-lint, wiki-init]
---
# Wiki Update

Revise existing wiki pages. Always show diffs before writing. Always log. Always cite the source of new information.

## Runtime Contract

- Use `openwiki.toml` as the runtime contract.
- `wiki/index.md` is the lightweight Routing Index.
- `wiki/indexes/` contains Shard Indexes.
- AI edits Markdown files directly. Do not require CLI page update commands for content writes.

## Pre-condition

Use the OpenWiki CLI as the source of truth for runtime discovery. Do not reimplement the config discovery chain inside this skill.

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

If `config path` reports `source = "global"`, tell the user explicitly that this workflow is using the global config at `~/.openwiki/openwiki.toml`.

If the user provides an explicit config path, pass it to the CLI:

```bash
openwiki --config /path/to/openwiki.toml config path --json
openwiki --config /path/to/openwiki.toml config show --json
```

If the user provides a project directory and `<project>/openwiki.toml` exists, pass that file with `--config`. If it does not exist, run CLI discovery from that directory or ask the user for the exact config path.

If the global `openwiki` command is unavailable or too old, and this is the OpenWiki repository, use the repository-built CLI from the repository root:

```bash
go run ./cmd/openwiki config path --json
go run ./cmd/openwiki config show --json
```

If neither CLI path works, ask the user to install/update OpenWiki CLI or provide an explicit `openwiki.toml` path.

Resolve `wiki_root` from `openwiki.toml`, then locate:

- `wiki/index.md`
- `wiki/log.md`
- `wiki/pages/`
- `wiki/indexes/`
- `entities/`
- `concepts/`

Do not depend on legacy agent-specific files or compatibility directories.


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

### 1. Identify what to update

The user may provide:

- specific page names.
- new information.
- a lint report.
- a requested metadata change such as title, summary, tags, scope, type, or updated date.

### 2. Read current content

Read the target Markdown files directly from `wiki/pages/`, `entities/`, or `concepts/`. Use `wiki/index.md` and relevant shard indexes under `wiki/indexes/` to locate candidates when the slug is unclear.

For each proposed change, show:

> **Current:** `<quote the existing text>`  
> **Proposed:** `<replacement text>`  
> **Reason:** `<why this change is warranted>`  
> **Source:** `<URL, raw/ path, or other source>`

Ask for confirmation before writing each page unless the user has explicitly authorized the full batch.

### 3. Write Markdown files directly

Edit the page files directly. Preserve frontmatter structure, citations, cross-references, and local style. Update `updated` whenever content or important metadata changes.

### 4. Check downstream effects

After identifying the primary pages to update, search for `[[slug]]` references across `wiki/pages/`, `entities/`, and `concepts/`. Flag linked pages that may also need updating.

### 5. Contradiction sweep

If the new information contradicts existing wiki content, search all pages for the contradicted claim and update all affected occurrences after confirmation.

## Layered Index Update Protocol

When changing page title, summary, tags, scope, type, or updated date:

1. Remove stale entries from old shard indexes.
2. Add updated entries to new shard indexes.
3. Update `wiki/indexes/recent.md`.
4. Keep `wiki/index.md` lightweight; do not add full page rows to it.
5. Append `wiki/log.md`.
6. If unsure whether all shard indexes were updated correctly, run or recommend:

```bash
openwiki index check
openwiki index rebuild
```

Shard placement:

- Summary pages in `wiki/pages/` belong in `wiki/indexes/scopes.md`, `wiki/indexes/tags.md`, and `wiki/indexes/recent.md`.
- Entity pages in `entities/` belong in `wiki/indexes/entities.md`, `wiki/indexes/tags.md`, and `wiki/indexes/recent.md`.
- Concept pages in `concepts/` belong in `wiki/indexes/concepts.md`, `wiki/indexes/tags.md`, and `wiki/indexes/recent.md`.

### 6. Append to `wiki/log.md`

Append a concise update record:

```text
update | <slug> - <reason>
```

### 7. Verify

Run or recommend:

```bash
openwiki index check
openwiki status
```

## Common Mistakes

- updating without a source.
- skipping downstream checks.
- skipping `wiki/log.md`.
- batch-writing without clear authorization.
- updating page metadata but leaving stale shard index entries under `wiki/indexes/`.
- adding full page catalogs to `wiki/index.md` instead of keeping it as the Routing Index.
