---
name: wiki-ingest
description: Use when adding a new source to a wiki — a paper, article, URL, file, transcript, or any document. One ingest may touch 5-15 wiki pages.
---
# Wiki Ingest

Add a source to the wiki. Read it, discuss with the user, write or update Markdown pages directly, update shard indexes, and log the operation.

## Runtime Contract

- Use `openwiki.toml` as the runtime contract.
- Do not infer paths from `cwd`, legacy agent-specific files, or compatibility directories.
- `wiki/index.md` is the lightweight Routing Index.
- `wiki/indexes/` contains Shard Indexes.
- AI writes Markdown files directly using templates. Do not require `openwiki page create` for content writes.

## Pre-condition

Discover and read `openwiki.toml`:

1. If the user explicitly provides a config path or project directory, use that `openwiki.toml`.
2. Otherwise, search upward from the current working directory for `openwiki.toml`.
3. If not found, ask the user for the project path or tell them to run `wiki-init` first.

Resolve `wiki_root` from `openwiki.toml`, then locate:

- `raw/`
- `wiki/index.md`
- `wiki/log.md`
- `wiki/pages/`
- `wiki/indexes/`
- `entities/`
- `concepts/`
- optional `remote_sync_path` and `auto_sync`

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

### 1. Accept the source

The source can be:

- **File path** — read it directly from `raw/` or another user-provided local path.
- **URL** — use the `agent-browser` skill to fetch it; snapshot to `raw/` if needed.
- **Pasted text** — use what the user provided.
- **当前会话上下文** — discussion history so far.

### 2. Read the source in full

Read all content. For long sources, read in sections. Do not skip.

### 3. Surface takeaways before writing anything

Tell the user:

- 3-5 bullet points of key takeaways.
- what entities or concepts this introduces or updates.
- whether it contradicts anything already in the wiki. Read `wiki/index.md`, 1-3 relevant shard indexes, and relevant pages to check.
- suggested `scope_level` and `scope_code` when the page template requires them.

Ask: **"Anything specific you want me to emphasize or de-emphasize? 适用范围是否合适？"**

Wait for the user's response before proceeding.

### 4. Network supplement when useful

For core concepts or key claims, use `agent-browser` to fetch current authoritative sources. Prioritize official or authoritative sites for the topic. External facts must preserve the source URL or a saved raw snapshot path.

### 5. Generate slugs

Use `references/slug-rules.md` for slug generation. Core rule: lowercase, hyphen-separated slugs with no special characters. For Chinese titles, translate the title meaning to English and slugify it; do not use pinyin by default.

### 6. Write or update wiki pages directly

Before writing summary, entity, or concept pages, read `references/page-template.md`. Write Markdown files directly using that template guidance and the existing page style:

- Summary pages: `wiki/pages/<slug>.md`
- Entity pages: `entities/<slug>.md`
- Concept pages: `concepts/<slug>.md`

Required frontmatter should include the fields used by the repository templates, including title, tags, updated date, and scope fields when applicable.

After writing, read the files back and check:

- frontmatter contains all required fields.
- `[[slug]]` cross-references point to existing or newly created pages.
- summary, tags, type, scope, and updated date match the intended page metadata.

### 7. Update related entity or concept pages

For each entity or concept touched by this source:

- **Page exists:** read it, update the relevant section, update `updated`.
- **Page does not exist:** create it with the same frontmatter format.
- Keep source-backed claims cited or traceable to the ingested source. External facts must preserve a URL or raw snapshot path so later readers can verify provenance.

### 8. Backlink audit

Scan existing pages in `wiki/pages/`, `entities/`, and `concepts/` for mentions of the new source's entities or concepts. Add `[[new-slug]]` references where appropriate and confirmed.

## Layered Index Write Protocol

After writing or updating a page, update the relevant shard indexes:

- Summary page in `wiki/pages/` → update `wiki/indexes/scopes.md`, `wiki/indexes/tags.md`, and `wiki/indexes/recent.md`.
- Entity page in `entities/` → update `wiki/indexes/entities.md`, `wiki/indexes/tags.md`, and `wiki/indexes/recent.md`.
- Concept page in `concepts/` → update `wiki/indexes/concepts.md`, `wiki/indexes/tags.md`, and `wiki/indexes/recent.md`.

Guidelines:

- Use the page title, one-line summary, tags, scope, type, path, and updated date in shard entries.
- Remove duplicate stale entries for the same slug before adding the new entry.
- Keep `wiki/indexes/recent.md` ordered by latest update where practical.
- Do not append all page rows to `wiki/index.md`. `wiki/index.md` is a Routing Index and must stay lightweight.
- Do not use any category-based top-level index sections as the source of truth.

If shard updates fail or are uncertain, warn the user that the page may not be discoverable and recommend:

```bash
openwiki index rebuild
```

### 9. Append to `wiki/log.md`

Append a concise operation record directly to `wiki/log.md` or with the log helper if available:

```text
ingest | <source title> - Created/Updated pages: xxx, yyy
```

### 10. Verify

After writing files, verify:

```bash
openwiki index check
openwiki status
```

If verification reports index drift, recommend or run:

```bash
openwiki index rebuild
```

### 11. Report to the user

- Summary pages created or updated: <list>
- Entity or concept pages created or updated: <list>
- Pages that received backlinks: <list>
- Shard indexes updated under `wiki/indexes/`
- Any verification warnings and suggested next steps

### 12. Cloud Sync

Read `references/cloud-sync.md` before syncing. If `remote_sync_path` and `auto_sync` are configured in `openwiki.toml`, sync `wiki_root` to the remote location using the configured workflow. Sync failure does not block ingest; report it separately.
