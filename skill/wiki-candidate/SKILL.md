---
name: wiki-candidate
description: Use when discovering candidate knowledge for OpenWiki review before ingest, especially from code agent sessions, conversation history, or agent memory files.
---
# Wiki Candidate

Discover candidate knowledge for human review before OpenWiki ingest. This skill creates review material only; it never writes accepted knowledge into formal wiki pages.

## Runtime Contract

- Use `openwiki.toml` as the runtime contract.
- Use the OpenWiki CLI candidate commands as the source of truth. Do not manually scan CodeAgent, conversation history, agent memory files, or other codeagent files.
- Candidate files are stored under `<wiki_root>/candidate/` by default.
- This skill only creates review material for later admission. It does not formally write `wiki/pages/`, `entities/`, `concepts/`, shard indexes, or `wiki/log.md` as accepted wiki knowledge.

## Preconditions

Resolve the active config before candidate discovery:

```bash
openwiki config path --json
openwiki config show --json
```

If the user provides an explicit config path, pass it to all OpenWiki CLI calls:

```bash
openwiki --config /path/to/openwiki.toml config path --json
openwiki --config /path/to/openwiki.toml config show --json
```

If the global `openwiki` command is unavailable or too old, and this is the OpenWiki repository, fall back from the OpenWiki repo root:

```bash
go run ./cmd/openwiki config path --json
go run ./cmd/openwiki config show --json
```

Use the same global CLI or `go run ./cmd/openwiki ...` form consistently for candidate commands. When an explicit config is known, include `--config /path/to/openwiki.toml`.

## CodeAgent Candidate Flow

1. Run the scan command. Include `--config` when a config path is known:

   ```bash
   openwiki candidate codeagent scan --json
   openwiki --config /path/to/openwiki.toml candidate codeagent scan --json
   ```

2. Read the returned `pending` path or identifier. Inspect the pending candidate payload produced by the CLI, not raw codeagent source files.
3. If the scan returns zero records, stop. Report that no candidate review material was created.
4. Read `references/codeagent-extraction-rules.md`.
5. Extract balanced-recall candidates from the pending records:
   - include enough candidates to avoid missing reusable knowledge;
   - avoid low-signal chatter, secrets, and purely local transient details;
   - preserve original external URLs and Feishu URLs exactly.
6. Read `references/review-doc-protocol.md`.
7. Use `lark-doc` v2 to create a Feishu review document following the protocol. The document must contain the protocol header and reviewable candidate cards.
8. Save a snapshot of the created review material into the configured `snapshot_dir` or the snapshot directory returned by the CLI. The snapshot must be enough to reconstruct what was sent to Feishu.
9. Only after both the Feishu review document and local snapshot are successfully created, commit the pending scan:

   ```bash
   openwiki candidate codeagent commit --pending <pending> --review-doc-url <url> --snapshot <snapshot> --json
   openwiki --config /path/to/openwiki.toml candidate codeagent commit --pending <pending> --review-doc-url <url> --snapshot <snapshot> --json
   ```

10. Final report must include:
    - Feishu review document link;
    - number of pending records scanned;
    - number of candidate cards created;
    - pending path or identifier;
    - snapshot path;
    - candidate output path under `<wiki_root>/candidate/` when reported by the CLI;
    - next step: user checks candidates in the Feishu document, then a later ingest/update workflow may admit only checked candidates.

## Guardrails

- Never run `openwiki candidate codeagent commit` before the review doc and snapshot both succeed.
- Never treat unchecked candidates as accepted. Unchecked means explicit user rejection for admission.
- Never write formal wiki pages, entity pages, concept pages, shard indexes, or log entries from this skill.
- Preserve original external URLs and Feishu URLs exactly as provenance.
- Redact secrets before creating the Feishu review document, snapshot, or candidate output.
- Do not manually scan codeagent files; use `openwiki candidate codeagent scan --json` and its pending output.

## References

- Extraction rules: `references/codeagent-extraction-rules.md`
- Review document protocol: `references/review-doc-protocol.md`
