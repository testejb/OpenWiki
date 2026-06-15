---
name: wiki-init
description: Use when bootstrapping a new personal wiki for any knowledge domain. Initialize a neutral `skill.io`-compatible wiki contract with a configurable `config-dir` and `wiki-root`.
---
# Wiki Init

Bootstrap a new LLM-maintained wiki using a neutral runtime contract.

## Pre-flight

Check whether an `openwiki.toml` already exists in the target configuration directory.

- If the user explicitly provides a `config-dir` and `<config-dir>/openwiki.toml` exists, reuse the existing config and treat the directory as an existing wiki instance rather than reinitializing.
- In that continue path, do not rewrite `openwiki.toml` unless the user explicitly asks to reinitialize.
- If `openwiki.toml` exists but the user did not explicitly provide the `config-dir`, ask the user whether to reinitialize or continue with the existing wiki instance.

## Process

### 1. Gather configuration (one question at a time)

If the workflow is reusing an existing `openwiki.toml`:

- Read the existing contract first.
- skip asking for `wiki_root`, `domain`, `source_types`, `index_categories`, `remote_sync_path`, `auto_sync`, `primary_language`, and `secondary_language` when they are already present in `openwiki.toml`.
- only ask for fields that are still missing from the existing contract.

If the workflow is creating a new wiki instance:

If the user does not provide a `config-dir`, use project-local `openwiki.toml` in the wiki root created by `openwiki init`.

Ask:

1. **Where should the wiki root directory live?** (absolute path)
2. **What is the domain/purpose?** (one sentence)
3. **What are the primary and secondary languages?** (e.g. `zh` / `en`, defaults: `zh` / `en`)
4. **What types of sources will you add?** (papers, URLs, code files, transcripts, etc.)
5. **Do you need any initial scope hints for the layered indexes?** If none, keep the generated shard indexes empty.

### 2. Initialize with CLI

Use the `openwiki` CLI to initialize the wiki:

```bash
openwiki init <wiki-root> --non-interactive --json
```

If the user wants to force overwrite an existing instance:

```bash
openwiki init <wiki-root> --force --non-interactive --json
```

### 3. Validate paths

- The wiki root directory must be an absolute path.
- The wiki root target must be writable or creatable.

If the workflow is reusing an existing `openwiki.toml`, fail fast when:

- the existing contract is missing `wiki_root`
- `wiki_root` is not absolute
- the required wiki layout is missing under `wiki_root`, including `wiki/index.md`, `wiki/log.md`, `wiki/pages/`, `wiki/indexes/`, or `entities/`

In that failure path:

- do not rewrite `openwiki.toml`
- do not create a replacement layout
- ask the user to fix the config or continue only if the user explicitly chooses `reinitialize`

### 4. Write `openwiki.toml`

The `openwiki init` command creates a project-local `openwiki.toml` in the wiki root directory. If a separate config directory is needed, copy the generated `openwiki.toml` to the config directory and update `wiki_root` to point to the wiki root.

Use the local starter template at `skill/wiki-init/templates/openwiki.toml` as reference. The starter config uses a layered index contract, not category-based `index.md` sections.

### 5. Verify wiki data layout

The CLI creates this structure under `wiki_root`:

```text
<wiki-root>/
├── raw/              ← immutable source documents
├── wiki/
│   ├── index.md      ← Routing Index: routes lookup to shard indexes
│   ├── indexes/
│   │   ├── scopes.md
│   │   ├── entities.md
│   │   ├── concepts.md
│   │   ├── tags.md
│   │   ├── recent.md
│   │   ├── hot.md
│   │   └── query-usage.jsonl
│   ├── log.md        ← append-only operation log
│   └── pages/        ← flat topic pages, one slug per file
├── entities/         ← entity pages (people, orgs, projects, tools)
└── concepts/         ← generated reports, analyses, and answers
```

`wiki/index.md` is only the Routing Index. Do not use it as a full content catalog. Put lookup entries in the shard indexes under `wiki/indexes/`; keep `query-usage.jsonl` for query usage records.

**Critical:** `wiki/pages/` is flat. All pages live here as `<slug>.md`. No subdirectories. Slugs are lowercase and hyphen-separated.

### 6. Confirm

Tell the user:

- If an existing config was reused, say the workflow is connected to the existing wiki.
- Show the resolved `wiki_root`, plus any available `domain`, `source_types`, layered index paths, `remote_sync_path`, and `auto_sync` from `openwiki.toml`.
- Tell the user they can keep using the same `config-dir` with `wiki-query`, `wiki-ingest`, `wiki-lint`, and `wiki-update`.
- Configuration initialized at `<config-dir>/openwiki.toml`
- Wiki data initialized under `<wiki-root>`
- Add sources to `raw/` manually, or run `wiki-ingest` with a file path, URL, or pasted text
- Run `wiki-lint` periodically to keep the wiki healthy
- `skill/` is the canonical public skill directory for this repository
