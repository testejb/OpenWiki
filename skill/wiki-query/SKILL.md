---
name: wiki-query
description: Use when asking a question against a personal wiki built with wiki-init and wiki-ingest. Do not answer from general knowledge — always read the wiki pages first.
---
# Wiki Query

Ask a question. Read the wiki through the Routing Index and Shard Indexes. Synthesize with citations. Offer to file the answer back.

## Runtime Contract

- Use `openwiki.toml` as the runtime contract.
- `wiki/index.md` is the lightweight Routing Index.
- `wiki/indexes/` contains Shard Indexes.
- Read local wiki evidence before answering from general knowledge.
- Append query usage to `wiki/indexes/query-usage.jsonl` after each query.

## Pre-condition

Discover and read `openwiki.toml`:

1. If the user explicitly provides a config path or project directory, use that `openwiki.toml`.
2. Otherwise, search upward from the current working directory for `openwiki.toml`.
3. If not found, ask the user for the project path or tell them to run `wiki-init` first.

Resolve `wiki_root` from `openwiki.toml`, then locate:

- `wiki/index.md`
- `wiki/log.md`
- `wiki/pages/`
- `wiki/indexes/`
- `entities/`
- `concepts/`

Do not depend on legacy agent-specific files or compatibility directories.

> **日期占位符说明：** 本文档中的 `<today>` 在执行时必须替换为实际当前日期，格式为 YYYY-MM-DD（如 `2026-05-26`）。

## Query Flow

1. Read `openwiki.toml` and resolve `wiki_root`.
2. Read `wiki/index.md` as the lightweight Routing Index.
3. Use the routing dimensions to choose shard indexes:
   - Scope clue → `wiki/indexes/scopes.md`
   - Entity clue → `wiki/indexes/entities.md`
   - Concept clue → `wiki/indexes/concepts.md`
   - Tag clue → `wiki/indexes/tags.md`
   - Recent/current clue → `wiki/indexes/recent.md`
   - Unclear query → `wiki/indexes/hot.md` and `wiki/indexes/recent.md`
4. Read 1-3 relevant shard indexes.
5. Select candidate pages.
6. Read candidate pages in full from `wiki/pages/`, `entities/`, or `concepts/`.
7. Follow one level of `[[slug]]` links if they are clearly relevant.
8. Answer with `[[slug]]` citations.
9. Append a JSON line to `wiki/indexes/query-usage.jsonl`.

Append query usage as one JSON object per line:

```json
{"time":"2026-06-15T15:30:00+08:00","query":"用户问题","matched_indexes":["indexes/tags.md"],"read_pages":["slug"],"cited_pages":["slug"],"intent_tags":["tag"]}
```

Use the actual timestamp, original user query, matched shard index paths, pages read, pages cited, and inferred intent tags.

## Outside supplement if needed

When local wiki evidence is insufficient, supplement with external sources after the Routing Index and selected shard indexes have been checked.

For ByteDance-internal or organization-specific questions, search relevant internal documentation when available. For public facts, prefer official or authoritative sources. Cite URLs for external sources and clearly separate them from local wiki citations.

## Synthesize the answer

Write a response that:

- is grounded in the wiki pages you read.
- cites inline using `[[slug]]` for local pages and URLs for web sources.
- notes agreements and disagreements between pages.
- flags gaps such as "The wiki has no page on X".
- suggests follow-up sources to ingest or questions to investigate.

## Always offer to save

After answering, say:

> "Worth saving to `concepts/<suggested-slug>.md`?"

If yes:

- write the concept page directly as Markdown with frontmatter such as `tags: [query, analysis]`, source count, and `updated: <today>`.
- update `wiki/indexes/concepts.md`, `wiki/indexes/tags.md`, and `wiki/indexes/recent.md`.
- keep `wiki/index.md` lightweight as the Routing Index.
- append a `query` record to `wiki/log.md`.

If no:

- still append a `query` record to `wiki/log.md` noting the pages read and whether outside verification was used.

## Common Mistakes

- **Answering from memory** — always read local wiki files first.
- **Reading every page by default** — route through `wiki/index.md` and 1-3 shard indexes instead.
- **Skipping query usage** — always append `wiki/indexes/query-usage.jsonl`.
- **Skipping the save offer** — always offer.
- **No citations** — every factual claim should trace back to a `[[slug]]` or URL.
