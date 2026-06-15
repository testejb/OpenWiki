# OpenWiki — AI-Powered Personal Knowledge Base

**Language / 语言 / 言語：** [中文](README.md) ｜ English（default）｜ [日本語](README.ja.md)

---

## What is this?

OpenWiki is a file-first personal knowledge-base scaffold for AI skills and agents. Markdown files are the primary write interface, and AI skills edit knowledge files directly. The CLI does not own content authoring; it provides guardrails for config discovery, validation, status, index checks, and index rebuilds.

**Core idea:**

- `openwiki.toml` is the only canonical runtime contract.
- The default `wiki_root` is `./openwiki/`.
- OpenWiki prefers a project-local runtime model: config and wiki data are discovered and used with the project.
- `wiki/index.md` is a lightweight Routing Index and does not list every page.
- `wiki/indexes/` contains Shard Indexes for growing indexes.

---

## Runtime Model

OpenWiki uses a project-local runtime model:

```text
<project>/
├── openwiki.toml            # only canonical runtime contract; target project-level contract
└── openwiki/                # default wiki_root
    ├── raw/                 # source material
    ├── wiki/
    │   ├── index.md         # lightweight Routing Index
    │   ├── log.md           # operation log
    │   ├── pages/           # regular knowledge pages
    │   └── indexes/         # Shard Indexes
    │       ├── scopes.md
    │       ├── entities.md
    │       ├── concepts.md
    │       ├── tags.md
    │       ├── recent.md
    │       ├── hot.md
    │       └── query-usage.jsonl
    ├── entities/            # entity pages
    └── concepts/            # concepts, analyses, saved answers
```

Core principles:

- `openwiki.toml` is the only canonical runtime contract.
- The default `wiki_root` is `./openwiki/`.
- Markdown page files are the source of truth for content.
- `wiki/index.md` is a lightweight Routing Index for routing only; it does not list every page.
- `wiki/indexes/` contains Shard Indexes: `scopes.md`, `entities.md`, `concepts.md`, `tags.md`, `recent.md`, `hot.md`, and `query-usage.jsonl`.
- AI skills write Markdown files directly.
- The CLI provides guardrails for config discovery, validation, status, `index check`, and `index rebuild`.

> The current CLI implementation of `openwiki init` writes `openwiki.toml` inside the target wiki root. To use the project-level contract model above, place `openwiki.toml` at the project root with `wiki_root = "./openwiki"`, or pass that file with `--config` / `-c`.

---

## Quick Start

### Prerequisites

- Any `skill.io`-compatible agent or any AI agent/tool that can read this repository's skills
- (Optional) [agent-browser](https://github.com/mediar-ai/agent-browser) for web-augmented research

### Installation

```bash
git clone https://github.com/crabin/llm-wiki.git openwiki-project
cd openwiki-project
```

Load this repository into your AI agent and ensure it can read the public wiki skills under `skill/`.

### Initialization

The current CLI initializes a wiki root:

```bash
openwiki init ./openwiki/
```

This command creates `./openwiki/` and writes `openwiki.toml`, `wiki/index.md`, `wiki/indexes/`, `raw/`, `entities/`, and `concepts/` inside that directory.

For a project-local top-level contract, use this `openwiki.toml` at the project root:

```toml
wiki_root = "./openwiki"

[wiki]
primary_language = "zh"
secondary_language = "en"
```

Then pass it explicitly when needed:

```bash
openwiki --config ./openwiki.toml status
openwiki --config ./openwiki.toml index check
openwiki --config ./openwiki.toml index rebuild
```

Discovery order:

1. `--config` / `-c`
2. `OPENWIKI_CONFIG`
3. Search upward from the current working directory for `openwiki.toml`
4. `~/.openwiki/openwiki.toml`

---

## Repository Layout

```text
openwiki-project/
├── skill/                    # public wiki skills
│   ├── wiki-init/
│   ├── wiki-ingest/
│   ├── wiki-query/
│   ├── wiki-lint/
│   ├── wiki-update/
│   └── agent-browser/
├── openwiki.toml             # project-local runtime contract (target model; may be passed with --config)
├── openwiki/                 # default wiki_root
│   ├── raw/
│   ├── wiki/
│   │   ├── index.md          # Routing Index
│   │   ├── log.md
│   │   ├── pages/
│   │   └── indexes/          # Shard Indexes
│   │       ├── scopes.md
│   │       ├── entities.md
│   │       ├── concepts.md
│   │       ├── tags.md
│   │       ├── recent.md
│   │       ├── hot.md
│   │       └── query-usage.jsonl
│   ├── entities/
│   └── concepts/
├── README.md
├── README.en.md
└── README.ja.md
```

---

## Skills and CLI Responsibilities

### AI skills

- `wiki-init`: prepares the project-local contract and wiki file layout.
- `wiki-ingest`: reads source material, confirms key takeaways with the user, writes Markdown pages directly, and maintains related Shard Indexes.
- `wiki-query`: reads the `wiki/index.md` Routing Index first, then relevant `wiki/indexes/` shards and pages; valuable answers can be saved under `concepts/`.
- `wiki-lint`: checks broken links, orphan pages, contradictions, stale content, and index health.
- `wiki-update`: edits existing Markdown pages directly and updates backlinks, logs, and shard indexes.
- `agent-browser`: performs web retrieval and fact-checking, supplying citable URLs and page content.

### CLI guardrails

The CLI handles runtime guardrails that do not own content authoring:

- discover `openwiki.toml` and resolve `wiki_root`;
- validate required directories, `wiki/index.md`, and `wiki/indexes/`;
- report status and index health;
- run `openwiki index check` to check the Routing Index and Shard Indexes;
- run `openwiki index rebuild` to rebuild indexes from Markdown pages and `query-usage.jsonl`.

---

## E2E Testing

- Fast deterministic artifact E2E:
  ```bash
  python3 -m unittest tests.test_wiki_skill_workflow_e2e -v
  ```
- Full fast test suite:
  ```bash
  python3 -m unittest discover -s tests -p "test_*.py"
  ```
- Slow real-agent smoke E2E:
  ```bash
  SKILL_AGENT_E2E=1 SKILL_AGENT_RUNNER=/path/to/compatible-agent-wrapper python3 -m unittest tests.test_agent_skill_smoke_e2e -v
  ```

The real runner scenario is skipped by default and only runs when `SKILL_AGENT_E2E=1` is set.

---

## Design Principles

- **File-first**: Markdown files are the primary source of truth for knowledge content.
- **Neutral runtime**: runtime behavior depends on `openwiki.toml`, not agent-specific file names.
- **Layered indexing**: the Routing Index stays lightweight, while Shard Indexes carry growth.
- **AI writes, CLI guards**: AI skills edit content; the CLI discovers, validates, and repairs indexes.
- **Traceable sources**: important claims should point to file paths or URLs.

---

## License

MIT
