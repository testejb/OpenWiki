# Skill CLI Runtime Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every wiki skill delegate runtime discovery to the OpenWiki CLI so global fallback (`~/.openwiki/openwiki.toml`) works consistently.

**Architecture:** Replace hand-maintained discovery instructions in skill docs with a shared CLI-first discovery protocol: `openwiki config path --json` then `openwiki config show --json`. The CLI remains the only source of truth for discovery order, and skills warn users when the active config source is `global`.

**Tech Stack:** Markdown skill docs, shell static checks, Go CLI behavior verification.

---

## File Structure

### Existing files to modify

- `skill/wiki-init/SKILL.md`
  - Use CLI discovery when reusing or checking existing runtime.
- `skill/wiki-ingest/SKILL.md`
  - Replace manual discovery chain with CLI-first Pre-condition.
- `skill/wiki-query/SKILL.md`
  - Replace manual discovery chain with CLI-first Pre-condition.
- `skill/wiki-update/SKILL.md`
  - Replace manual discovery chain with CLI-first Pre-condition.
- `skill/wiki-lint/SKILL.md`
  - Replace manual discovery chain with CLI-first Pre-condition.

### Optional files to modify only if static checks need a home

- None. Use shell grep checks for this small doc-only change.

---

### Task 1: Update all wiki skills to use CLI runtime discovery

**Files:**
- Modify: `skill/wiki-init/SKILL.md`
- Modify: `skill/wiki-ingest/SKILL.md`
- Modify: `skill/wiki-query/SKILL.md`
- Modify: `skill/wiki-update/SKILL.md`
- Modify: `skill/wiki-lint/SKILL.md`

- [ ] **Step 1: Add a shared CLI discovery block to each skill**

In each file, ensure the Pre-condition or Runtime Contract section contains this protocol, adapted to surrounding wording:

```markdown
## Runtime Discovery

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
```

- [ ] **Step 2: Add explicit config handling to each skill**

In each skill, add this instruction near Runtime Discovery:

```markdown
If the user provides an explicit config path, pass it to the CLI:

```bash
openwiki --config /path/to/openwiki.toml config path --json
openwiki --config /path/to/openwiki.toml config show --json
```

If the user provides a project directory and `<project>/openwiki.toml` exists, pass that file with `--config`. If it does not exist, run CLI discovery from that directory or ask the user for the exact config path.
```

- [ ] **Step 3: Add CLI fallback handling to each skill**

Each skill already has an index-command guardrail. Ensure runtime discovery also says:

```markdown
If the global `openwiki` command is unavailable or too old, and this is the OpenWiki repository, use the repository-built CLI from the repository root:

```bash
go run ./cmd/openwiki config path --json
go run ./cmd/openwiki config show --json
```

If neither CLI path works, ask the user to install/update OpenWiki CLI or provide an explicit `openwiki.toml` path.
```

- [ ] **Step 4: Remove partial hand-maintained discovery chains**

In these files, remove or replace old wording like:

```text
1. If the user explicitly provides a config path or project directory, use that openwiki.toml.
2. Otherwise, search upward from the current working directory for openwiki.toml.
3. If not found, ask the user for the project path or tell them to run wiki-init first.
```

Do not leave a partial discovery chain that omits `OPENWIKI_CONFIG` or `~/.openwiki/openwiki.toml`.

- [ ] **Step 5: Verify static doc checks**

Run:

```bash
grep -R "openwiki config path --json" skill/wiki-init/SKILL.md skill/wiki-ingest/SKILL.md skill/wiki-query/SKILL.md skill/wiki-update/SKILL.md skill/wiki-lint/SKILL.md
grep -R "openwiki config show --json" skill/wiki-init/SKILL.md skill/wiki-ingest/SKILL.md skill/wiki-query/SKILL.md skill/wiki-update/SKILL.md skill/wiki-lint/SKILL.md
grep -R "Do not reimplement" skill/wiki-init/SKILL.md skill/wiki-ingest/SKILL.md skill/wiki-query/SKILL.md skill/wiki-update/SKILL.md skill/wiki-lint/SKILL.md
grep -R "~/.openwiki/openwiki.toml" skill/wiki-init/SKILL.md skill/wiki-ingest/SKILL.md skill/wiki-query/SKILL.md skill/wiki-update/SKILL.md skill/wiki-lint/SKILL.md
grep -R "source = \"global\"" skill/wiki-init/SKILL.md skill/wiki-ingest/SKILL.md skill/wiki-query/SKILL.md skill/wiki-update/SKILL.md skill/wiki-lint/SKILL.md
! grep -R "Otherwise, search upward from the current working directory" skill/wiki-init/SKILL.md skill/wiki-ingest/SKILL.md skill/wiki-query/SKILL.md skill/wiki-update/SKILL.md skill/wiki-lint/SKILL.md
```

Expected: first five commands print matches in all five files; final command exits 0.

- [ ] **Step 6: Verify CLI behavior from a no-local directory**

Build and run the current CLI from a temporary directory that has no local `openwiki.toml`:

```bash
tmpbin=$(mktemp /tmp/openwiki-discovery.XXXXXX)
go build -o "$tmpbin" ./cmd/openwiki
tmpdir=$(mktemp -d)
(
  cd "$tmpdir"
  "$tmpbin" config path --json
)
rm -rf "$tmpdir" "$tmpbin"
```

Expected when global config exists:

```json
"source": "global"
```

If the local machine has no global config, expected result is a clear `CONFIG_NOT_FOUND` response. Do not create user global config as part of this task.

- [ ] **Step 7: Run full Go tests**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add skill/wiki-init/SKILL.md skill/wiki-ingest/SKILL.md skill/wiki-query/SKILL.md skill/wiki-update/SKILL.md skill/wiki-lint/SKILL.md
git commit -m "feat: 统一技能运行时发现为 CLI 委托"
```

---

## Self-Review Checklist

After implementing, verify:

- Each of the five skill docs contains `openwiki config path --json`.
- Each contains `openwiki config show --json`.
- Each says not to reimplement discovery inside the skill.
- Each mentions CLI discovery includes `OPENWIKI_CONFIG` and `~/.openwiki/openwiki.toml`.
- Each instructs the agent to tell the user when `source = "global"`.
- No skill contains the old partial discovery sequence that searches upward and then immediately asks the user.
- `go test ./... -count=1` passes.
