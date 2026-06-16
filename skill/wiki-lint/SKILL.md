---
name: wiki-lint
description: Use when auditing a wiki for health issues — contradictions between pages, orphan pages, broken cross-references, stale claims, missing pages, or coverage gaps.
---
# Wiki Lint

Audit the wiki. Produce a categorized report. Offer concrete fixes. Log the operation.

## Runtime Contract

- Use `openwiki.toml` as the runtime contract.
- `wiki/index.md` is the lightweight Routing Index.
- `wiki/indexes/` contains Shard Indexes.
- Audit Markdown files directly under the resolved `wiki_root`.

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
- `primary_language`
- `secondary_language`

Do not depend on legacy agent-specific files or compatibility directories.

> **日期占位符说明：** 本文档中的 `<today>` 在执行时必须替换为实际当前日期，格式为 YYYY-MM-DD（如 `2026-05-26`）。


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

### 1. Build the page inventory

Read `wiki/index.md`, shard indexes under `wiki/indexes/`, and all Markdown files in:

- `wiki/pages/`
- `entities/`
- `concepts/`

Build a map of:

- all existing slugs and file paths.
- all `[[slug]]` references.
- all frontmatter fields.
- all tags, scopes, page types, summaries, and updated dates.
- all entries found in shard indexes.

### 2. Run content health checks

Apply the repository lint rules where available. Categorize findings by severity:

**Red Errors**: broken links, missing required frontmatter, unreadable page files.

**Yellow Warnings**: orphan pages, contradictions, stale claims, language/style mismatches, missing bilingual terms, missing or invalid scope fields, invalid entity type, weak summaries.

**Blue Info**: missing concept pages, missing cross-references, hardcoded date placeholders, coverage gaps.

Language-specific rules should follow `primary_language` and `secondary_language` in `openwiki.toml`.

### 2.1 Layered Index Checks

Audit:

- `wiki/index.md` exists and is a Routing Index.
- `wiki/index.md` does not contain full all-page tables.
- Required shard indexes exist under `wiki/indexes/`.
- Every page in `wiki/pages/`, `entities/`, and `concepts/` appears in at least one appropriate shard index.
- Shard indexes do not link to missing pages.
- `wiki/indexes/hot.md` is not stale relative to `wiki/indexes/query-usage.jsonl`.

Required shard indexes:

- `wiki/indexes/scopes.md`
- `wiki/indexes/entities.md`
- `wiki/indexes/concepts.md`
- `wiki/indexes/tags.md`
- `wiki/indexes/recent.md`
- `wiki/indexes/hot.md`
- `wiki/indexes/query-usage.jsonl`

If index inconsistencies are found, recommend:

```bash
openwiki index rebuild
```

### 3. Verify output completeness

Check that:

- every page file was scanned.
- every Red Error has a concrete fix suggestion.
- every Yellow Warning has an explanation.
- every index inconsistency names the affected shard index and slug/path.

### 4. Write the lint report

Write `concepts/lint-<today>.md` directly as Markdown and summarize all findings with concrete remediation suggestions.

Update `wiki/indexes/concepts.md`, `wiki/indexes/tags.md`, and `wiki/indexes/recent.md` for the lint report. Keep `wiki/index.md` lightweight.

### 5. Offer concrete fixes

For each fixable category, offer precise edits and show diffs before writing. If the fix is purely index drift, recommend `openwiki index rebuild` before manual edits.

### 6. Append to `wiki/log.md`

Always append a `lint` entry with issue counts, index health summary, and any fixes applied.
