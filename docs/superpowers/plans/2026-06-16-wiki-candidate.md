# Wiki Candidate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `wiki-candidate` as OpenWiki's candidate-knowledge review workflow, with CLI-backed codeagent incremental scanning and `wiki-ingest` support for checked-only Candidate Review Docs.

**Architecture:** OpenWiki CLI owns deterministic candidate infrastructure: config parsing, codeagent JSONL parsing, scan pending files, state commit, and status. The `wiki-candidate` skill owns semantic extraction and Feishu review-doc creation. `wiki-ingest` gains a protocol guardrail so checked candidates are ingested directly and unchecked candidates are ignored completely.

**Tech Stack:** Go 1.26 standard library, BurntSushi TOML config loading, existing `internal/cli` command pattern, existing `internal/output` JSON envelope, Markdown skill files, Feishu DocxXML via `lark-doc` at runtime.

---

## File Structure

### Create

- `internal/candidate/config.go` — resolve candidate/codeagent config from `config.Config`, apply defaults, resolve paths relative to `wiki_root`.
- `internal/candidate/types.go` — shared record, pending, state, warning, and result structs.
- `internal/candidate/parser.go` — parse `traex-history` and `trae-ide-memory` JSONL records into normalized records.
- `internal/candidate/scan.go` — discover files, apply first-run limits, produce pending scan files without advancing state.
- `internal/candidate/state.go` — load/save state atomically, compute file snapshots, merge committed updates.
- `internal/candidate/log.go` — append human-readable run log lines.
- `internal/candidate/config_test.go` — candidate config default/override tests.
- `internal/candidate/parser_test.go` — parser tests for both source types and malformed lines.
- `internal/candidate/scan_test.go` — scan, first-run limit, append, truncate/rewrite tests.
- `internal/candidate/commit_test.go` — commit validation and atomic state advancement tests.
- `internal/cli/candidate.go` — CLI routing for `openwiki candidate codeagent scan|commit|status`.
- `internal/cli/candidate_test.go` — CLI JSON tests for scan, commit, status, help/error paths.
- `skill/wiki-candidate/SKILL.md` — workflow for generating candidate review docs.
- `skill/wiki-candidate/references/codeagent-extraction-rules.md` — extraction categories, skip rules, redaction rules.
- `skill/wiki-candidate/references/review-doc-protocol.md` — Candidate Review Doc protocol and card structure.
- `skill/wiki-ingest/references/candidate-review-doc.md` — checked-only ingest contract.

### Modify

- `internal/config/config.go` — add `CandidateConfig` fields to `Config` while preserving existing config behavior.
- `internal/config/config_test.go` — verify TOML parses `[candidate]` and relative candidate paths remain raw until candidate resolver.
- `internal/cli/root.go` — route top-level `candidate` command and include it in help.
- `skill/wiki-ingest/SKILL.md` — add Candidate Review Doc guardrail after source acceptance.

---

## Task 1: Candidate Config Model and Defaults

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Create: `internal/candidate/config.go`
- Create: `internal/candidate/types.go`
- Create: `internal/candidate/config_test.go`

- [ ] **Step 1: Write failing config load test**

Append this test to `internal/config/config_test.go`:

```go
func TestLoadCandidateConfig(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "./wiki-root"

[candidate]
state_dir = "candidate-state"
run_log_path = "candidate.log"
snapshot_dir = "candidate-reviews"

[candidate.codeagent]
enabled = true
state_path = "codeagent/state.json"
pending_dir = "codeagent/pending"
run_log_path = "codeagent/run.log"
snapshot_dir = "codeagent/reviews"
initial_days = 7
max_records_per_run = 200

[[candidate.codeagent.agents]]
name = "traex-work"
type = "traex-history"
paths = ["/tmp/history.jsonl"]
enabled = true
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test toml: %v", err)
	}

	cfg, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Candidate.StateDir != "candidate-state" {
		t.Fatalf("expected candidate.state_dir to remain raw, got %q", cfg.Candidate.StateDir)
	}
	if !cfg.Candidate.CodeAgent.Enabled {
		t.Fatal("expected candidate.codeagent.enabled=true")
	}
	if cfg.Candidate.CodeAgent.InitialDays != 7 {
		t.Fatalf("expected initial_days=7, got %d", cfg.Candidate.CodeAgent.InitialDays)
	}
	if cfg.Candidate.CodeAgent.MaxRecordsPerRun != 200 {
		t.Fatalf("expected max_records_per_run=200, got %d", cfg.Candidate.CodeAgent.MaxRecordsPerRun)
	}
	if len(cfg.Candidate.CodeAgent.Agents) != 1 {
		t.Fatalf("expected 1 codeagent agent, got %d", len(cfg.Candidate.CodeAgent.Agents))
	}
	agent := cfg.Candidate.CodeAgent.Agents[0]
	if agent.Name != "traex-work" || agent.Type != "traex-history" || !agent.Enabled {
		t.Fatalf("unexpected agent: %#v", agent)
	}
	if len(agent.Paths) != 1 || agent.Paths[0] != "/tmp/history.jsonl" {
		t.Fatalf("unexpected agent paths: %#v", agent.Paths)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/config -run TestLoadCandidateConfig -v
```

Expected: FAIL with compile errors like `cfg.Candidate undefined`.

- [ ] **Step 3: Add config structs**

Update `internal/config/config.go` so `Config` and the new structs include candidate fields:

```go
type Config struct {
	WikiRoot  string          `toml:"wiki_root"`
	Wiki      WikiConfig      `toml:"wiki"`
	Remote    RemoteConfig    `toml:"remote"`
	Candidate CandidateConfig `toml:"candidate"`
}

type CandidateConfig struct {
	StateDir   string                   `toml:"state_dir"`
	RunLogPath string                   `toml:"run_log_path"`
	SnapshotDir string                  `toml:"snapshot_dir"`
	CodeAgent  CandidateCodeAgentConfig `toml:"codeagent"`
}

type CandidateCodeAgentConfig struct {
	Enabled          bool                   `toml:"enabled"`
	StatePath        string                 `toml:"state_path"`
	PendingDir       string                 `toml:"pending_dir"`
	RunLogPath       string                 `toml:"run_log_path"`
	SnapshotDir      string                 `toml:"snapshot_dir"`
	InitialDays      int                    `toml:"initial_days"`
	MaxRecordsPerRun int                    `toml:"max_records_per_run"`
	Agents           []CandidateAgentConfig `toml:"agents"`
}

type CandidateAgentConfig struct {
	Name    string   `toml:"name"`
	Type    string   `toml:"type"`
	Paths   []string `toml:"paths"`
	Enabled bool     `toml:"enabled"`
}
```

Do not resolve candidate paths in `config.Load`; candidate path resolution belongs in `internal/candidate` so existing config behavior stays stable.

- [ ] **Step 4: Run config tests**

Run:

```bash
go test ./internal/config -run 'TestLoadCandidateConfig|TestLoadValidTOML|TestLoadResolvesRelativeWikiRootFromConfigDir' -v
```

Expected: PASS.

- [ ] **Step 5: Write failing candidate default tests**

Create `internal/candidate/config_test.go`:

```go
package candidate_test

import (
	"path/filepath"
	"testing"

	"github.com/bytedance/openwiki/internal/candidate"
	"github.com/bytedance/openwiki/internal/config"
)

func TestResolveCodeAgentConfigDefaults(t *testing.T) {
	wikiRoot := t.TempDir()
	cfg := &config.Config{WikiRoot: wikiRoot}

	resolved := candidate.ResolveCodeAgentConfig(cfg)

	if !resolved.Enabled {
		t.Fatal("expected default codeagent candidate source to be enabled")
	}
	if resolved.StatePath != filepath.Join(wikiRoot, "candidate", "codeagent", "state.json") {
		t.Fatalf("unexpected state path: %s", resolved.StatePath)
	}
	if resolved.PendingDir != filepath.Join(wikiRoot, "candidate", "codeagent", "pending") {
		t.Fatalf("unexpected pending dir: %s", resolved.PendingDir)
	}
	if resolved.RunLogPath != filepath.Join(wikiRoot, "candidate", "codeagent", "run.log") {
		t.Fatalf("unexpected run log path: %s", resolved.RunLogPath)
	}
	if resolved.SnapshotDir != filepath.Join(wikiRoot, "candidate", "codeagent", "reviews") {
		t.Fatalf("unexpected snapshot dir: %s", resolved.SnapshotDir)
	}
	if resolved.InitialDays != 14 {
		t.Fatalf("expected initial_days=14, got %d", resolved.InitialDays)
	}
	if resolved.MaxRecordsPerRun != 500 {
		t.Fatalf("expected max_records_per_run=500, got %d", resolved.MaxRecordsPerRun)
	}
	if len(resolved.Agents) != 2 {
		t.Fatalf("expected built-in traex and trae-ide agents, got %d", len(resolved.Agents))
	}
	if resolved.Agents[0].Name != "traex" || resolved.Agents[0].Type != "traex-history" {
		t.Fatalf("unexpected first default agent: %#v", resolved.Agents[0])
	}
	if resolved.Agents[1].Name != "trae-ide" || resolved.Agents[1].Type != "trae-ide-memory" {
		t.Fatalf("unexpected second default agent: %#v", resolved.Agents[1])
	}
}

func TestResolveCodeAgentConfigOverrides(t *testing.T) {
	wikiRoot := t.TempDir()
	cfg := &config.Config{WikiRoot: wikiRoot}
	cfg.Candidate.StateDir = "my-candidates"
	cfg.Candidate.CodeAgent.StatePath = "custom/state.json"
	cfg.Candidate.CodeAgent.PendingDir = "custom/pending"
	cfg.Candidate.CodeAgent.RunLogPath = "custom/run.log"
	cfg.Candidate.CodeAgent.SnapshotDir = "custom/reviews"
	cfg.Candidate.CodeAgent.InitialDays = 3
	cfg.Candidate.CodeAgent.MaxRecordsPerRun = 10
	cfg.Candidate.CodeAgent.Agents = []config.CandidateAgentConfig{{
		Name: "custom", Type: "traex-history", Paths: []string{"/tmp/history.jsonl"}, Enabled: true,
	}}

	resolved := candidate.ResolveCodeAgentConfig(cfg)

	if resolved.StatePath != filepath.Join(wikiRoot, "custom", "state.json") {
		t.Fatalf("unexpected state path: %s", resolved.StatePath)
	}
	if resolved.PendingDir != filepath.Join(wikiRoot, "custom", "pending") {
		t.Fatalf("unexpected pending dir: %s", resolved.PendingDir)
	}
	if resolved.RunLogPath != filepath.Join(wikiRoot, "custom", "run.log") {
		t.Fatalf("unexpected run log path: %s", resolved.RunLogPath)
	}
	if resolved.SnapshotDir != filepath.Join(wikiRoot, "custom", "reviews") {
		t.Fatalf("unexpected snapshot dir: %s", resolved.SnapshotDir)
	}
	if resolved.InitialDays != 3 || resolved.MaxRecordsPerRun != 10 {
		t.Fatalf("unexpected limits: %#v", resolved)
	}
	if len(resolved.Agents) != 1 || resolved.Agents[0].Name != "custom" {
		t.Fatalf("expected configured agents to replace built-ins, got %#v", resolved.Agents)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run:

```bash
go test ./internal/candidate -run TestResolveCodeAgentConfig -v
```

Expected: FAIL because `internal/candidate` does not exist.

- [ ] **Step 7: Create candidate types and resolver**

Create `internal/candidate/types.go`:

```go
package candidate

import "time"

type AgentConfig struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Paths   []string `json:"paths"`
	Enabled bool     `json:"enabled"`
}

type CodeAgentConfig struct {
	Enabled          bool          `json:"enabled"`
	StatePath        string        `json:"state_path"`
	PendingDir       string        `json:"pending_dir"`
	RunLogPath       string        `json:"run_log_path"`
	SnapshotDir      string        `json:"snapshot_dir"`
	InitialDays      int           `json:"initial_days"`
	MaxRecordsPerRun int           `json:"max_records_per_run"`
	Agents           []AgentConfig `json:"agents"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Line    int    `json:"line,omitempty"`
}

type Record struct {
	RecordID   string   `json:"record_id"`
	Agent      string   `json:"agent"`
	Type       string   `json:"type"`
	SourceFile string   `json:"source_file"`
	LineStart  int      `json:"line_start"`
	LineEnd    int      `json:"line_end"`
	ByteStart  int64    `json:"byte_start"`
	ByteEnd    int64    `json:"byte_end"`
	Timestamp  string   `json:"timestamp,omitempty"`
	SessionID  string   `json:"session_id,omitempty"`
	MessageID  string   `json:"message_id,omitempty"`
	Intent     string   `json:"intent,omitempty"`
	Actions    []string `json:"actions,omitempty"`
	Outcome    string   `json:"outcome,omitempty"`
	Learned    []string `json:"learned,omitempty"`
	Text       string   `json:"text"`
}

type FileState struct {
	Agent         string `json:"agent"`
	Type          string `json:"type"`
	FileID        string `json:"file_id"`
	Size          int64  `json:"size"`
	MTime         string `json:"mtime"`
	ProcessedLines int   `json:"processed_lines"`
	ProcessedBytes int64 `json:"processed_bytes"`
	TailHash      string `json:"tail_hash"`
	LastScannedAt string `json:"last_scanned_at"`
}

type State struct {
	Version   int                  `json:"version"`
	Source    string               `json:"source"`
	UpdatedAt string               `json:"updated_at"`
	Files     map[string]FileState `json:"files"`
}

type Pending struct {
	Version      int                  `json:"version"`
	Source       string               `json:"source"`
	CreatedAt    string               `json:"created_at"`
	Status       string               `json:"status"`
	Config       PendingConfig        `json:"config"`
	Limits       PendingLimits        `json:"limits"`
	Records      []Record             `json:"records"`
	StateUpdates map[string]FileState `json:"state_updates"`
	Warnings     []Warning            `json:"warnings,omitempty"`
	ReviewDocURL string               `json:"review_doc_url,omitempty"`
	SnapshotPath string               `json:"snapshot_path,omitempty"`
	CommittedAt  string               `json:"committed_at,omitempty"`
}

type PendingConfig struct {
	ConfigPath  string `json:"config_path"`
	WikiRoot    string `json:"wiki_root"`
	StatePath   string `json:"state_path"`
	PendingDir  string `json:"pending_dir"`
	RunLogPath  string `json:"run_log_path"`
	SnapshotDir string `json:"snapshot_dir"`
}

type PendingLimits struct {
	InitialDays      int `json:"initial_days"`
	MaxRecordsPerRun int `json:"max_records_per_run"`
}

type ScanOptions struct {
	ConfigPath       string
	WikiRoot         string
	Now              time.Time
	InitialDays      int
	MaxRecordsPerRun int
}

type ScanResult struct {
	PendingPath string            `json:"pending_path"`
	StatePath   string            `json:"state_path"`
	RunLogPath  string            `json:"run_log_path"`
	SnapshotDir string            `json:"snapshot_dir"`
	Records     ScanRecordSummary `json:"records"`
	Limits      PendingLimits     `json:"limits"`
	Warnings    []Warning         `json:"warnings,omitempty"`
}

type ScanRecordSummary struct {
	Total   int            `json:"total"`
	ByAgent map[string]int `json:"by_agent"`
}

type CommitResult struct {
	PendingPath  string `json:"pending_path"`
	StatePath    string `json:"state_path"`
	ReviewDocURL string `json:"review_doc_url"`
	SnapshotPath string `json:"snapshot_path"`
	CommittedAt  string `json:"committed_at"`
}

type StatusResult struct {
	Enabled       bool      `json:"enabled"`
	StatePath     string    `json:"state_path"`
	PendingDir    string    `json:"pending_dir"`
	RunLogPath    string    `json:"run_log_path"`
	SnapshotDir   string    `json:"snapshot_dir"`
	TrackedFiles  int       `json:"tracked_files"`
	LastUpdatedAt string    `json:"last_updated_at,omitempty"`
	Agents        []AgentConfig `json:"agents"`
	Warnings      []Warning `json:"warnings,omitempty"`
}
```

Create `internal/candidate/config.go`:

```go
package candidate

import (
	"path/filepath"

	"github.com/bytedance/openwiki/internal/config"
)

const (
	DefaultInitialDays      = 14
	DefaultMaxRecordsPerRun = 500
)

func ResolveCodeAgentConfig(cfg *config.Config) CodeAgentConfig {
	candidateRoot := resolvePath(cfg.WikiRoot, cfg.Candidate.StateDir, filepath.Join(cfg.WikiRoot, "candidate"))
	code := cfg.Candidate.CodeAgent
	resolved := CodeAgentConfig{
		Enabled:          true,
		StatePath:        resolvePath(cfg.WikiRoot, code.StatePath, filepath.Join(candidateRoot, "codeagent", "state.json")),
		PendingDir:       resolvePath(cfg.WikiRoot, code.PendingDir, filepath.Join(candidateRoot, "codeagent", "pending")),
		RunLogPath:       resolvePath(cfg.WikiRoot, code.RunLogPath, filepath.Join(candidateRoot, "codeagent", "run.log")),
		SnapshotDir:      resolvePath(cfg.WikiRoot, code.SnapshotDir, filepath.Join(candidateRoot, "codeagent", "reviews")),
		InitialDays:      code.InitialDays,
		MaxRecordsPerRun: code.MaxRecordsPerRun,
	}
	if !code.Enabled && hasCodeAgentConfig(code) {
		resolved.Enabled = false
	}
	if resolved.InitialDays <= 0 {
		resolved.InitialDays = DefaultInitialDays
	}
	if resolved.MaxRecordsPerRun <= 0 {
		resolved.MaxRecordsPerRun = DefaultMaxRecordsPerRun
	}
	if len(code.Agents) == 0 {
		resolved.Agents = defaultCodeAgentAgents()
		return resolved
	}
	for _, agent := range code.Agents {
		if !agent.Enabled {
			continue
		}
		resolved.Agents = append(resolved.Agents, AgentConfig{
			Name: agent.Name, Type: agent.Type, Paths: agent.Paths, Enabled: agent.Enabled,
		})
	}
	return resolved
}

func hasCodeAgentConfig(code config.CandidateCodeAgentConfig) bool {
	return code.StatePath != "" || code.PendingDir != "" || code.RunLogPath != "" || code.SnapshotDir != "" || code.InitialDays != 0 || code.MaxRecordsPerRun != 0 || len(code.Agents) > 0
}

func resolvePath(wikiRoot, configured, fallback string) string {
	if configured == "" {
		return filepath.Clean(fallback)
	}
	if filepath.IsAbs(configured) {
		return filepath.Clean(configured)
	}
	return filepath.Clean(filepath.Join(wikiRoot, configured))
}

func defaultCodeAgentAgents() []AgentConfig {
	return []AgentConfig{
		{Name: "traex", Type: "traex-history", Paths: []string{"/Users/bytedance/.trae/cli/history.jsonl"}, Enabled: true},
		{Name: "trae-ide", Type: "trae-ide-memory", Paths: []string{"/Users/bytedance/.trae-cn/memory/projects/**/session_memory_*.jsonl"}, Enabled: true},
	}
}
```

- [ ] **Step 8: Run tests and commit**

Run:

```bash
go test ./internal/config ./internal/candidate -run 'TestLoadCandidateConfig|TestResolveCodeAgentConfig' -v
```

Expected: PASS.

Commit:

```bash
git add internal/config/config.go internal/config/config_test.go internal/candidate/types.go internal/candidate/config.go internal/candidate/config_test.go
git commit -m "feat: 添加候选配置默认值"
```

---

## Task 2: CodeAgent JSONL Parsers

**Files:**
- Create: `internal/candidate/parser.go`
- Create: `internal/candidate/parser_test.go`

- [ ] **Step 1: Write failing parser tests**

Create `internal/candidate/parser_test.go`:

```go
package candidate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/openwiki/internal/candidate"
)

func TestParseTraexHistoryJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	content := strings.Join([]string{
		`{"session_id":"s1","ts":1781597600,"text":"如何使用 openwiki config path --json"}`,
		`{"session_id":"s1","ts":1781597700,"text":"https://example.com/docs 这个文档有用"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	records, warnings, err := candidate.ParseJSONLFile(candidate.AgentConfig{Name: "traex", Type: "traex-history"}, path, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Agent != "traex" || records[0].Type != "traex-history" {
		t.Fatalf("unexpected record metadata: %#v", records[0])
	}
	if records[0].SessionID != "s1" || records[0].Timestamp == "" {
		t.Fatalf("expected session and timestamp, got %#v", records[0])
	}
	if !strings.Contains(records[0].RecordID, "history.jsonl:line:1") {
		t.Fatalf("unexpected record id: %s", records[0].RecordID)
	}
	if records[1].LineStart != 2 || records[1].ByteStart <= records[0].ByteEnd {
		t.Fatalf("expected line and byte offsets to advance: %#v %#v", records[0], records[1])
	}
}

func TestParseTraeIDEMemoryJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session_memory_x.jsonl")
	content := `{"intent":"修改 openwiki init 默认路径","actions":["分析 internal/cli/init.go","更新测试"],"outcome":"测试通过","learned":["init 默认路径是 ./openwiki/"],"message_summary_time":"2026-06-04 14:29:35","message_id":"m1"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	records, warnings, err := candidate.ParseJSONLFile(candidate.AgentConfig{Name: "trae-ide", Type: "trae-ide-memory"}, path, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 || len(records) != 1 {
		t.Fatalf("records=%d warnings=%#v", len(records), warnings)
	}
	r := records[0]
	if r.MessageID != "m1" || r.Intent == "" || len(r.Actions) != 2 || len(r.Learned) != 1 {
		t.Fatalf("unexpected parsed record: %#v", r)
	}
	if !strings.Contains(r.Text, "修改 openwiki init 默认路径") || !strings.Contains(r.Text, "测试通过") {
		t.Fatalf("expected readable text summary, got %q", r.Text)
	}
}

func TestParseJSONLFileSkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	content := "{bad json}\n" + `{"session_id":"s2","ts":1781597600,"text":"有效记录"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	records, warnings, err := candidate.ParseJSONLFile(candidate.AgentConfig{Name: "traex", Type: "traex-history"}, path, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 valid record, got %d", len(records))
	}
	if len(warnings) != 1 || warnings[0].Code != "JSONL_PARSE_FAILED" || warnings[0].Line != 1 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/candidate -run 'TestParse' -v
```

Expected: FAIL because `ParseJSONLFile` is undefined.

- [ ] **Step 3: Implement parser**

Create `internal/candidate/parser.go`:

```go
package candidate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type traexHistoryLine struct {
	SessionID string `json:"session_id"`
	TS        int64  `json:"ts"`
	Text      string `json:"text"`
}

type traeIDEMemoryLine struct {
	Intent             string   `json:"intent"`
	Actions            []string `json:"actions"`
	Outcome            string   `json:"outcome"`
	Learned            []string `json:"learned"`
	MessageSummaryTime string   `json:"message_summary_time"`
	MessageID          string   `json:"message_id"`
}

func ParseJSONLFile(agent AgentConfig, path string, startByte int64, startLine int) ([]Record, []Warning, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	if startByte > 0 {
		if _, err := f.Seek(startByte, 0); err != nil {
			return nil, nil, err
		}
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := startLine
	byteStart := startByte
	var records []Record
	var warnings []Warning

	for scanner.Scan() {
		lineNo++
		lineBytes := append([]byte(nil), scanner.Bytes()...)
		lineText := string(lineBytes)
		byteEnd := byteStart + int64(len(lineBytes)) + 1
		record, ok, warn := parseLine(agent, path, lineNo, byteStart, byteEnd, lineText)
		if warn != nil {
			warnings = append(warnings, *warn)
		}
		if ok {
			records = append(records, record)
		}
		byteStart = byteEnd
	}
	if err := scanner.Err(); err != nil {
		return records, warnings, err
	}
	return records, warnings, nil
}

func parseLine(agent AgentConfig, path string, lineNo int, byteStart, byteEnd int64, line string) (Record, bool, *Warning) {
	switch agent.Type {
	case "traex-history":
		var raw traexHistoryLine
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return Record{}, false, parseWarning(path, lineNo, err)
		}
		if strings.TrimSpace(raw.Text) == "" {
			return Record{}, false, nil
		}
		return Record{
			RecordID: fmt.Sprintf("%s:%s:line:%d", agent.Name, path, lineNo),
			Agent: agent.Name, Type: agent.Type, SourceFile: path,
			LineStart: lineNo, LineEnd: lineNo, ByteStart: byteStart, ByteEnd: byteEnd,
			Timestamp: unixTimestamp(raw.TS), SessionID: raw.SessionID, Text: raw.Text,
		}, true, nil
	case "trae-ide-memory":
		var raw traeIDEMemoryLine
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return Record{}, false, parseWarning(path, lineNo, err)
		}
		text := buildTraeIDEText(raw)
		if strings.TrimSpace(text) == "" {
			return Record{}, false, nil
		}
		return Record{
			RecordID: fmt.Sprintf("%s:%s:line:%d", agent.Name, path, lineNo),
			Agent: agent.Name, Type: agent.Type, SourceFile: path,
			LineStart: lineNo, LineEnd: lineNo, ByteStart: byteStart, ByteEnd: byteEnd,
			Timestamp: parseTraeIDETime(raw.MessageSummaryTime), MessageID: raw.MessageID,
			Intent: raw.Intent, Actions: raw.Actions, Outcome: raw.Outcome, Learned: raw.Learned, Text: text,
		}, true, nil
	default:
		return Record{}, false, &Warning{Code: "UNSUPPORTED_AGENT_TYPE", Message: fmt.Sprintf("unsupported codeagent type: %s", agent.Type), Path: path, Line: lineNo}
	}
}

func parseWarning(path string, line int, err error) *Warning {
	return &Warning{Code: "JSONL_PARSE_FAILED", Message: err.Error(), Path: path, Line: line}
}

func unixTimestamp(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format(time.RFC3339)
}

func parseTraeIDETime(value string) string {
	if value == "" {
		return ""
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return t.Format(time.RFC3339)
	}
	return value
}

func buildTraeIDEText(raw traeIDEMemoryLine) string {
	var b strings.Builder
	if raw.Intent != "" {
		b.WriteString("意图：")
		b.WriteString(raw.Intent)
	}
	if len(raw.Actions) > 0 {
		appendSection(&b, "\n行动：", raw.Actions)
	}
	if raw.Outcome != "" {
		if b.Len() > 0 { b.WriteString("\n") }
		b.WriteString("结果：")
		b.WriteString(raw.Outcome)
	}
	if len(raw.Learned) > 0 {
		appendSection(&b, "\n学到：", raw.Learned)
	}
	return strings.TrimSpace(b.String())
}

func appendSection(b *strings.Builder, title string, items []string) {
	b.WriteString(title)
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		b.WriteString("\n- ")
		b.WriteString(item)
	}
}
```

- [ ] **Step 4: Run parser tests**

Run:

```bash
go test ./internal/candidate -run 'TestParse' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/candidate/parser.go internal/candidate/parser_test.go
git commit -m "feat: 添加会话记录解析"
```

---

## Task 3: Scan, Pending, and State Management

**Files:**
- Create: `internal/candidate/state.go`
- Create: `internal/candidate/scan.go`
- Create: `internal/candidate/log.go`
- Create: `internal/candidate/scan_test.go`
- Create: `internal/candidate/commit_test.go`

- [ ] **Step 1: Write failing scan and commit tests**

Create `internal/candidate/scan_test.go`:

```go
package candidate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bytedance/openwiki/internal/candidate"
)

func TestScanCreatesPendingWithoutAdvancingState(t *testing.T) {
	dir := t.TempDir()
	history := filepath.Join(dir, "history.jsonl")
	content := `{"session_id":"s1","ts":1781597600,"text":"openwiki config show --json 的使用"}` + "\n"
	if err := os.WriteFile(history, []byte(content), 0644); err != nil { t.Fatal(err) }

	cfg := candidate.CodeAgentConfig{
		Enabled: true,
		StatePath: filepath.Join(dir, "candidate", "codeagent", "state.json"),
		PendingDir: filepath.Join(dir, "candidate", "codeagent", "pending"),
		RunLogPath: filepath.Join(dir, "candidate", "codeagent", "run.log"),
		SnapshotDir: filepath.Join(dir, "candidate", "codeagent", "reviews"),
		InitialDays: 14,
		MaxRecordsPerRun: 500,
		Agents: []candidate.AgentConfig{{Name:"traex", Type:"traex-history", Paths:[]string{history}, Enabled:true}},
	}

	result, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{ConfigPath: filepath.Join(dir, "openwiki.toml"), WikiRoot: dir, Now: time.Unix(1781600000, 0)})
	if err != nil { t.Fatalf("scan failed: %v", err) }
	if result.Records.Total != 1 { t.Fatalf("expected 1 record, got %#v", result.Records) }
	if _, err := os.Stat(result.PendingPath); err != nil { t.Fatalf("pending missing: %v", err) }
	if _, err := os.Stat(cfg.StatePath); !os.IsNotExist(err) { t.Fatalf("state should not exist before commit, err=%v", err) }

	var pending candidate.Pending
	data, err := os.ReadFile(result.PendingPath)
	if err != nil { t.Fatal(err) }
	if err := json.Unmarshal(data, &pending); err != nil { t.Fatal(err) }
	if pending.Status != "pending" || len(pending.Records) != 1 || len(pending.StateUpdates) != 1 {
		t.Fatalf("unexpected pending: %#v", pending)
	}
}

func TestScanAppendUsesProcessedBytes(t *testing.T) {
	dir := t.TempDir()
	history := filepath.Join(dir, "history.jsonl")
	first := `{"session_id":"s1","ts":1781597600,"text":"第一条"}` + "\n"
	second := `{"session_id":"s1","ts":1781597700,"text":"第二条"}` + "\n"
	if err := os.WriteFile(history, []byte(first), 0644); err != nil { t.Fatal(err) }

	cfg := testCodeAgentConfig(dir, history)
	firstResult, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{ConfigPath: "config", WikiRoot: dir, Now: time.Unix(1781600000,0)})
	if err != nil { t.Fatal(err) }
	snapshot := filepath.Join(cfg.SnapshotDir, "first.json")
	if err := os.MkdirAll(cfg.SnapshotDir, 0755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(snapshot, []byte(`{"review_doc_url":"https://example.com/doc"}`), 0644); err != nil { t.Fatal(err) }
	if _, err := candidate.CommitCodeAgent(firstResult.PendingPath, "https://example.com/doc", snapshot, time.Unix(1781600100,0)); err != nil { t.Fatal(err) }

	f, err := os.OpenFile(history, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil { t.Fatal(err) }
	if _, err := f.WriteString(second); err != nil { t.Fatal(err) }
	if err := f.Close(); err != nil { t.Fatal(err) }

	secondResult, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{ConfigPath: "config", WikiRoot: dir, Now: time.Unix(1781600200,0)})
	if err != nil { t.Fatal(err) }
	if secondResult.Records.Total != 1 { t.Fatalf("expected only appended record, got %#v", secondResult.Records) }
	data, _ := os.ReadFile(secondResult.PendingPath)
	var pending candidate.Pending
	if err := json.Unmarshal(data, &pending); err != nil { t.Fatal(err) }
	if pending.Records[0].Text != "第二条" { t.Fatalf("expected appended text, got %#v", pending.Records[0]) }
}

func testCodeAgentConfig(dir, history string) candidate.CodeAgentConfig {
	return candidate.CodeAgentConfig{
		Enabled: true,
		StatePath: filepath.Join(dir, "candidate", "codeagent", "state.json"),
		PendingDir: filepath.Join(dir, "candidate", "codeagent", "pending"),
		RunLogPath: filepath.Join(dir, "candidate", "codeagent", "run.log"),
		SnapshotDir: filepath.Join(dir, "candidate", "codeagent", "reviews"),
		InitialDays: 14,
		MaxRecordsPerRun: 500,
		Agents: []candidate.AgentConfig{{Name:"traex", Type:"traex-history", Paths:[]string{history}, Enabled:true}},
	}
}
```

Create `internal/candidate/commit_test.go`:

```go
package candidate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bytedance/openwiki/internal/candidate"
)

func TestCommitRequiresReviewDocAndSnapshot(t *testing.T) {
	dir := t.TempDir()
	pendingPath := filepath.Join(dir, "pending.json")
	pending := candidate.Pending{Version:1, Source:"codeagent", Status:"pending", Config:candidate.PendingConfig{StatePath:filepath.Join(dir,"state.json"), RunLogPath:filepath.Join(dir,"run.log")}, StateUpdates: map[string]candidate.FileState{}}
	data, _ := json.Marshal(pending)
	if err := os.WriteFile(pendingPath, data, 0644); err != nil { t.Fatal(err) }

	if _, err := candidate.CommitCodeAgent(pendingPath, "", filepath.Join(dir,"missing.json"), time.Now()); err == nil {
		t.Fatal("expected error for empty review doc url")
	}
	if _, err := candidate.CommitCodeAgent(pendingPath, "https://example.com/doc", filepath.Join(dir,"missing.json"), time.Now()); err == nil {
		t.Fatal("expected error for missing snapshot")
	}
}

func TestCommitAdvancesStateAndMarksPending(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	runLog := filepath.Join(dir, "run.log")
	pendingPath := filepath.Join(dir, "pending.json")
	snapshot := filepath.Join(dir, "review.json")
	if err := os.WriteFile(snapshot, []byte(`{"candidates":[]}`), 0644); err != nil { t.Fatal(err) }

	pending := candidate.Pending{
		Version:1, Source:"codeagent", Status:"pending",
		Config:candidate.PendingConfig{StatePath:statePath, RunLogPath:runLog},
		StateUpdates: map[string]candidate.FileState{"/tmp/history.jsonl":{Agent:"traex", Type:"traex-history", ProcessedLines:2, ProcessedBytes:100}},
	}
	data, _ := json.MarshalIndent(pending, "", "  ")
	if err := os.WriteFile(pendingPath, data, 0644); err != nil { t.Fatal(err) }

	result, err := candidate.CommitCodeAgent(pendingPath, "https://example.com/doc", snapshot, time.Unix(1781600000,0))
	if err != nil { t.Fatalf("commit failed: %v", err) }
	if result.ReviewDocURL != "https://example.com/doc" || result.SnapshotPath != snapshot { t.Fatalf("unexpected result: %#v", result) }

	stateData, err := os.ReadFile(statePath)
	if err != nil { t.Fatalf("state missing: %v", err) }
	var state candidate.State
	if err := json.Unmarshal(stateData, &state); err != nil { t.Fatal(err) }
	if state.Files["/tmp/history.jsonl"].ProcessedLines != 2 { t.Fatalf("state not advanced: %#v", state) }

	pendingData, err := os.ReadFile(pendingPath)
	if err != nil { t.Fatal(err) }
	var committed candidate.Pending
	if err := json.Unmarshal(pendingData, &committed); err != nil { t.Fatal(err) }
	if committed.Status != "committed" || committed.ReviewDocURL == "" || committed.SnapshotPath == "" {
		t.Fatalf("pending not marked committed: %#v", committed)
	}
	logData, err := os.ReadFile(runLog)
	if err != nil { t.Fatal(err) }
	if string(logData) == "" { t.Fatal("expected run log entry") }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/candidate -run 'TestScan|TestCommit' -v
```

Expected: FAIL because `ScanCodeAgent` and `CommitCodeAgent` are undefined.

- [ ] **Step 3: Implement state helpers**

Create `internal/candidate/state.go` with these functions:

```go
package candidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

func LoadState(path string) (State, error) {
	state := State{Version: 1, Source: "codeagent", Files: map[string]FileState{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil { return state, err }
	if err := json.Unmarshal(data, &state); err != nil { return state, err }
	if state.Files == nil { state.Files = map[string]FileState{} }
	return state, nil
}

func SaveStateAtomic(path string, state State) error {
	state.Version = 1
	state.Source = "codeagent"
	if state.Files == nil { state.Files = map[string]FileState{} }
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { return err }
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil { return err }
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil { return err }
	return os.Rename(tmp, path)
}

func LoadPending(path string) (Pending, error) {
	var pending Pending
	data, err := os.ReadFile(path)
	if err != nil { return pending, err }
	if err := json.Unmarshal(data, &pending); err != nil { return pending, err }
	return pending, nil
}

func SavePendingAtomic(path string, pending Pending) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { return err }
	data, err := json.MarshalIndent(pending, "", "  ")
	if err != nil { return err }
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil { return err }
	return os.Rename(tmp, path)
}

func fileID(info os.FileInfo) string {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino)
	}
	return fmt.Sprintf("%s:%d:%d", runtime.GOOS, info.Size(), info.ModTime().UnixNano())
}

func tailHash(path string) string {
	f, err := os.Open(path)
	if err != nil { return "" }
	defer f.Close()
	info, err := f.Stat()
	if err != nil { return "" }
	start := info.Size() - 4096
	if start < 0 { start = 0 }
	if _, err := f.Seek(start, 0); err != nil { return "" }
	h := sha256.New()
	_, _ = io.Copy(h, f)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func newFileState(agent AgentConfig, path string, info os.FileInfo, processedLines int, processedBytes int64, scannedAt time.Time) FileState {
	return FileState{
		Agent: agent.Name, Type: agent.Type, FileID: fileID(info), Size: info.Size(), MTime: info.ModTime().Format(time.RFC3339),
		ProcessedLines: processedLines, ProcessedBytes: processedBytes, TailHash: tailHash(path), LastScannedAt: scannedAt.Format(time.RFC3339),
	}
}
```

- [ ] **Step 4: Implement scan and commit**

Create `internal/candidate/scan.go`:

```go
package candidate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func ScanCodeAgent(cfg CodeAgentConfig, opts ScanOptions) (ScanResult, error) {
	now := opts.Now
	if now.IsZero() { now = time.Now() }
	initialDays := opts.InitialDays
	if initialDays <= 0 { initialDays = cfg.InitialDays }
	if initialDays <= 0 { initialDays = DefaultInitialDays }
	maxRecords := opts.MaxRecordsPerRun
	if maxRecords <= 0 { maxRecords = cfg.MaxRecordsPerRun }
	if maxRecords <= 0 { maxRecords = DefaultMaxRecordsPerRun }

	state, err := LoadState(cfg.StatePath)
	if err != nil { return ScanResult{}, err }

	var all []Record
	var warnings []Warning
	updates := map[string]FileState{}
	byAgent := map[string]int{}
	for _, agent := range cfg.Agents {
		if !agent.Enabled { continue }
		paths := expandAgentPaths(agent.Paths)
		if len(paths) == 0 {
			warnings = append(warnings, Warning{Code:"NO_MATCHING_FILES", Message:"no files matched codeagent paths", Path: fmt.Sprintf("%v", agent.Paths)})
			continue
		}
		for _, path := range paths {
			info, err := os.Stat(path)
			if err != nil {
				warnings = append(warnings, Warning{Code:"SOURCE_FILE_NOT_FOUND", Message:err.Error(), Path:path})
				continue
			}
			previous, known := state.Files[path]
			startByte, startLine := int64(0), 0
			firstRun := !known
			if known && previous.FileID == fileID(info) && info.Size() >= previous.ProcessedBytes && previous.TailHash == tailHash(path) {
				if info.Size() == previous.ProcessedBytes { continue }
				startByte = previous.ProcessedBytes
				startLine = previous.ProcessedLines
			} else if known && info.Size() < previous.ProcessedBytes {
				warnings = append(warnings, Warning{Code:"SOURCE_FILE_TRUNCATED", Message:"source file became smaller; scanning from beginning", Path:path})
			} else if known && previous.FileID != fileID(info) {
				warnings = append(warnings, Warning{Code:"SOURCE_FILE_REPLACED", Message:"source file id changed; scanning from beginning", Path:path})
			} else if known {
				warnings = append(warnings, Warning{Code:"SOURCE_FILE_REWRITTEN", Message:"source file changed unexpectedly; scanning from beginning", Path:path})
			}

			records, parseWarnings, err := ParseJSONLFile(agent, path, startByte, startLine)
			if err != nil { warnings = append(warnings, Warning{Code:"SOURCE_FILE_READ_FAILED", Message:err.Error(), Path:path}); continue }
			warnings = append(warnings, parseWarnings...)
			if firstRun { records = filterFirstRun(records, now, initialDays) }
			all = append(all, records...)
			byAgent[agent.Name] += len(records)
			updates[path] = newFileState(agent, path, info, countLines(path), info.Size(), now)
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Timestamp < all[j].Timestamp })
	if len(all) > maxRecords { all = all[len(all)-maxRecords:] }

	pendingPath := filepath.Join(cfg.PendingDir, now.Format("2006-01-02-150405")+"-scan.json")
	pending := Pending{Version:1, Source:"codeagent", CreatedAt:now.Format(time.RFC3339), Status:"pending", Config:PendingConfig{ConfigPath:opts.ConfigPath, WikiRoot:opts.WikiRoot, StatePath:cfg.StatePath, PendingDir:cfg.PendingDir, RunLogPath:cfg.RunLogPath, SnapshotDir:cfg.SnapshotDir}, Limits:PendingLimits{InitialDays:initialDays, MaxRecordsPerRun:maxRecords}, Records:all, StateUpdates:updates, Warnings:warnings}
	if err := SavePendingAtomic(pendingPath, pending); err != nil { return ScanResult{}, err }
	_ = appendRunLog(cfg.RunLogPath, now, "pending_created", fmt.Sprintf("path=%s records=%d", pendingPath, len(all)))
	return ScanResult{PendingPath:pendingPath, StatePath:cfg.StatePath, RunLogPath:cfg.RunLogPath, SnapshotDir:cfg.SnapshotDir, Records:ScanRecordSummary{Total:len(all), ByAgent:byAgent}, Limits:PendingLimits{InitialDays:initialDays, MaxRecordsPerRun:maxRecords}, Warnings:warnings}, nil
}

func CommitCodeAgent(pendingPath, reviewDocURL, snapshotPath string, now time.Time) (CommitResult, error) {
	if reviewDocURL == "" { return CommitResult{}, fmt.Errorf("review doc url is required") }
	if _, err := os.Stat(snapshotPath); err != nil { return CommitResult{}, fmt.Errorf("snapshot is required: %w", err) }
	if now.IsZero() { now = time.Now() }
	pending, err := LoadPending(pendingPath)
	if err != nil { return CommitResult{}, err }
	if pending.Status != "pending" { return CommitResult{}, fmt.Errorf("pending status must be pending, got %s", pending.Status) }
	state, err := LoadState(pending.Config.StatePath)
	if err != nil { return CommitResult{}, err }
	if state.Files == nil { state.Files = map[string]FileState{} }
	for path, update := range pending.StateUpdates { state.Files[path] = update }
	state.UpdatedAt = now.Format(time.RFC3339)
	if err := SaveStateAtomic(pending.Config.StatePath, state); err != nil { return CommitResult{}, err }
	pending.Status = "committed"
	pending.ReviewDocURL = reviewDocURL
	pending.SnapshotPath = snapshotPath
	pending.CommittedAt = now.Format(time.RFC3339)
	if err := SavePendingAtomic(pendingPath, pending); err != nil { return CommitResult{}, err }
	_ = appendRunLog(pending.Config.RunLogPath, now, "committed", fmt.Sprintf("pending=%s review_doc=%s snapshot=%s", pendingPath, reviewDocURL, snapshotPath))
	return CommitResult{PendingPath:pendingPath, StatePath:pending.Config.StatePath, ReviewDocURL:reviewDocURL, SnapshotPath:snapshotPath, CommittedAt:pending.CommittedAt}, nil
}

func StatusCodeAgent(cfg CodeAgentConfig) (StatusResult, error) {
	state, err := LoadState(cfg.StatePath)
	if err != nil { return StatusResult{}, err }
	return StatusResult{Enabled:cfg.Enabled, StatePath:cfg.StatePath, PendingDir:cfg.PendingDir, RunLogPath:cfg.RunLogPath, SnapshotDir:cfg.SnapshotDir, TrackedFiles:len(state.Files), LastUpdatedAt:state.UpdatedAt, Agents:cfg.Agents}, nil
}

func expandAgentPaths(patterns []string) []string {
	seen := map[string]bool{}
	var paths []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 { matches = []string{pattern} }
		for _, match := range matches { if !seen[match] { seen[match]=true; paths=append(paths, match) } }
	}
	sort.Strings(paths)
	return paths
}

func filterFirstRun(records []Record, now time.Time, days int) []Record {
	cutoff := now.Add(-time.Duration(days)*24*time.Hour)
	var filtered []Record
	for _, record := range records {
		if record.Timestamp == "" { filtered = append(filtered, record); continue }
		if ts, err := time.Parse(time.RFC3339, record.Timestamp); err == nil && !ts.Before(cutoff) { filtered = append(filtered, record) }
	}
	return filtered
}

func countLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil { return 0 }
	count := 0
	for _, b := range data { if b == '\n' { count++ } }
	if len(data) > 0 && data[len(data)-1] != '\n' { count++ }
	return count
}

func WriteCandidateSnapshot(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { return err }
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil { return err }
	return os.WriteFile(path, data, 0644)
}
```

Create `internal/candidate/log.go`:

```go
package candidate

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func appendRunLog(path string, at time.Time, event, details string) error {
	if path == "" { return nil }
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil { return err }
	line := fmt.Sprintf("%s | %s | %s\n", at.Format(time.RFC3339), event, details)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil { return err }
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}
```

- [ ] **Step 4: Run scan/commit tests and fix compile details**

Run:

```bash
go test ./internal/candidate -run 'TestScan|TestCommit' -v
```

Expected: PASS after resolving any import formatting or struct field alignment issues.

- [ ] **Step 5: Run all candidate tests**

Run:

```bash
go test ./internal/candidate -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/candidate/state.go internal/candidate/scan.go internal/candidate/log.go internal/candidate/scan_test.go internal/candidate/commit_test.go
git commit -m "feat: 添加候选扫描状态"
```

---

## Task 4: Candidate CLI Commands

**Files:**
- Modify: `internal/cli/root.go`
- Create: `internal/cli/candidate.go`
- Create: `internal/cli/candidate_test.go`

- [ ] **Step 1: Write failing CLI tests**

Create `internal/cli/candidate_test.go`:

```go
package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/openwiki/internal/cli"
	"github.com/bytedance/openwiki/internal/output"
)

func TestCandidateCodeAgentScanJSON(t *testing.T) {
	dir := t.TempDir()
	history := filepath.Join(dir, "history.jsonl")
	if err := os.WriteFile(history, []byte(`{"session_id":"s1","ts":1781597600,"text":"openwiki candidate codeagent scan"}`+"\n"), 0644); err != nil { t.Fatal(err) }
	configPath := writeCandidateCLIConfig(t, dir, history)

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--config", configPath, "candidate", "codeagent", "scan", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil { t.Fatalf("scan failed: %v stderr=%s", err, stderr.String()) }
	var resp output.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil { t.Fatalf("unmarshal: %v\n%s", err, stdout.String()) }
	if !resp.Success { t.Fatalf("expected success: %#v", resp.Error) }
	data := resp.Data.(map[string]interface{})
	if data["pending_path"] == "" || data["state_path"] == "" { t.Fatalf("missing paths: %#v", data) }
	records := data["records"].(map[string]interface{})
	if records["total"].(float64) != 1 { t.Fatalf("expected 1 record, got %#v", records) }
}

func TestCandidateCodeAgentCommitJSON(t *testing.T) {
	dir := t.TempDir()
	history := filepath.Join(dir, "history.jsonl")
	if err := os.WriteFile(history, []byte(`{"session_id":"s1","ts":1781597600,"text":"待审核知识"}`+"\n"), 0644); err != nil { t.Fatal(err) }
	configPath := writeCandidateCLIConfig(t, dir, history)

	var scanOut, scanErr bytes.Buffer
	if err := cli.RunWithIO([]string{"--config", configPath, "candidate", "codeagent", "scan", "--json"}, "1.0.0", "2026", &scanOut, &scanErr); err != nil { t.Fatal(err) }
	var scanResp output.Response
	if err := json.Unmarshal(scanOut.Bytes(), &scanResp); err != nil { t.Fatal(err) }
	pendingPath := scanResp.Data.(map[string]interface{})["pending_path"].(string)
	snapshot := filepath.Join(dir, "wiki-root", "candidate", "codeagent", "reviews", "snapshot.json")
	if err := os.MkdirAll(filepath.Dir(snapshot), 0755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(snapshot, []byte(`{"candidates":[]}`), 0644); err != nil { t.Fatal(err) }

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--config", configPath, "candidate", "codeagent", "commit", "--pending", pendingPath, "--review-doc-url", "https://example.com/doc", "--snapshot", snapshot, "--json"}, "1.0.0", "2026", &stdout, &stderr)
	if err != nil { t.Fatalf("commit failed: %v stderr=%s", err, stderr.String()) }
	if !strings.Contains(stdout.String(), `"success": true`) { t.Fatalf("expected success JSON, got %s", stdout.String()) }
	if _, err := os.Stat(filepath.Join(dir, "wiki-root", "candidate", "codeagent", "state.json")); err != nil { t.Fatalf("state missing: %v", err) }
}

func TestCandidateCodeAgentStatusJSON(t *testing.T) {
	dir := t.TempDir()
	history := filepath.Join(dir, "history.jsonl")
	if err := os.WriteFile(history, []byte(""), 0644); err != nil { t.Fatal(err) }
	configPath := writeCandidateCLIConfig(t, dir, history)

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--config", configPath, "candidate", "codeagent", "status", "--json"}, "1.0.0", "2026", &stdout, &stderr)
	if err != nil { t.Fatalf("status failed: %v stderr=%s", err, stderr.String()) }
	if !strings.Contains(stdout.String(), `"tracked_files"`) { t.Fatalf("expected status fields, got %s", stdout.String()) }
}

func writeCandidateCLIConfig(t *testing.T, dir, history string) string {
	t.Helper()
	wikiRoot := filepath.Join(dir, "wiki-root")
	if err := os.MkdirAll(wikiRoot, 0755); err != nil { t.Fatal(err) }
	configPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "` + wikiRoot + `"

[wiki]
primary_language = "zh"
secondary_language = "en"

[candidate.codeagent]
initial_days = 14
max_records_per_run = 500

[[candidate.codeagent.agents]]
name = "traex-test"
type = "traex-history"
paths = ["` + history + `"]
enabled = true
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil { t.Fatal(err) }
	return configPath
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cli -run TestCandidateCodeAgent -v
```

Expected: FAIL with unknown command `candidate`.

- [ ] **Step 3: Wire root command**

Modify `internal/cli/root.go`:

```go
	switch subcommand {
	case "config":
		return runConfig(stdout, stderr, &opts, subArgs)
	case "init":
		return runInit(stdout, stderr, &opts, subArgs)
	case "status":
		return runStatus(stdout, stderr, &opts, subArgs)
	case "page":
		return runPage(stdout, stderr, &opts, subArgs)
	case "index":
		return runIndex(stdout, stderr, &opts, subArgs)
	case "log":
		return runLog(stdout, stderr, &opts, subArgs)
	case "sync":
		return runSync(stdout, stderr, &opts, subArgs)
	case "candidate":
		return runCandidate(stdout, stderr, &opts, subArgs)
	default:
		return fmt.Errorf("未知命令: %s\n使用 'openwiki --help' 查看可用命令", subcommand)
	}
```

Add to help command list:

```text
  candidate 管理候选知识扫描与审核状态
```

- [ ] **Step 4: Implement CLI command file**

Create `internal/cli/candidate.go`:

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/bytedance/openwiki/internal/candidate"
	"github.com/bytedance/openwiki/internal/output"
)

func runCandidate(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	if len(args) == 0 { return fmt.Errorf("candidate 需要子命令: codeagent") }
	switch args[0] {
	case "codeagent":
		return runCandidateCodeAgent(stdout, stderr, opts, args[1:])
	default:
		return fmt.Errorf("未知 candidate 子命令: %s", args[0])
	}
}

func runCandidateCodeAgent(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	if len(args) == 0 { return fmt.Errorf("candidate codeagent 需要子命令: scan, commit, status") }
	switch args[0] {
	case "scan":
		return runCandidateCodeAgentScan(stdout, stderr, opts, args[1:])
	case "commit":
		return runCandidateCodeAgentCommit(stdout, stderr, opts, args[1:])
	case "status":
		return runCandidateCodeAgentStatus(stdout, stderr, opts, args[1:])
	default:
		return fmt.Errorf("未知 candidate codeagent 子命令: %s", args[0])
	}
}

func runCandidateCodeAgentScan(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	flags := flag.NewFlagSet("candidate codeagent scan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	initialDays := flags.Int("initial-days", 0, "首次扫描最近天数")
	maxRecords := flags.Int("max-records", 0, "每次最多 records 数")
	jsonFlag := flags.Bool("json", false, "JSON 输出")
	if err := flags.Parse(args); err != nil { return err }
	if *jsonFlag { opts.JSON = true }
	cfg, result, err := discoverConfig(opts)
	if err != nil { return candidateJSONError(stdout, opts, "CONFIG_NOT_FOUND", err) }
	codeCfg := candidate.ResolveCodeAgentConfig(cfg)
	res, err := candidate.ScanCodeAgent(codeCfg, candidate.ScanOptions{ConfigPath: result.Path, WikiRoot: cfg.WikiRoot, Now: time.Now(), InitialDays: *initialDays, MaxRecordsPerRun: *maxRecords})
	if err != nil { return candidateJSONError(stdout, opts, "CANDIDATE_SCAN_FAILED", err) }
	if opts.JSON { return output.JSON(stdout, true, res, nil) }
	fmt.Fprintf(stdout, "候选扫描完成，records: %d\nPending: %s\n", res.Records.Total, res.PendingPath)
	return nil
}

func runCandidateCodeAgentCommit(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	flags := flag.NewFlagSet("candidate codeagent commit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	pending := flags.String("pending", "", "pending scan 文件")
	reviewDocURL := flags.String("review-doc-url", "", "飞书审核文档 URL")
	snapshot := flags.String("snapshot", "", "候选快照 JSON")
	jsonFlag := flags.Bool("json", false, "JSON 输出")
	if err := flags.Parse(args); err != nil { return err }
	if *jsonFlag { opts.JSON = true }
	if *pending == "" { return fmt.Errorf("candidate codeagent commit 需要 --pending") }
	res, err := candidate.CommitCodeAgent(*pending, *reviewDocURL, *snapshot, time.Now())
	if err != nil { return candidateJSONError(stdout, opts, "CANDIDATE_COMMIT_FAILED", err) }
	if opts.JSON { return output.JSON(stdout, true, res, nil) }
	fmt.Fprintf(stdout, "候选扫描状态已提交: %s\n", res.StatePath)
	return nil
}

func runCandidateCodeAgentStatus(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	flags := flag.NewFlagSet("candidate codeagent status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonFlag := flags.Bool("json", false, "JSON 输出")
	if err := flags.Parse(args); err != nil { return err }
	if *jsonFlag { opts.JSON = true }
	cfg, _, err := discoverConfig(opts)
	if err != nil { return candidateJSONError(stdout, opts, "CONFIG_NOT_FOUND", err) }
	res, err := candidate.StatusCodeAgent(candidate.ResolveCodeAgentConfig(cfg))
	if err != nil { return candidateJSONError(stdout, opts, "CANDIDATE_STATUS_FAILED", err) }
	if opts.JSON { return output.JSON(stdout, true, res, nil) }
	fmt.Fprintf(stdout, "候选状态: tracked_files=%d state=%s\n", res.TrackedFiles, res.StatePath)
	return nil
}

func candidateJSONError(stdout io.Writer, opts *GlobalOptions, code string, err error) error {
	if opts.JSON {
		return output.JSON(stdout, false, nil, &output.ErrorInfo{Code: code, Message: err.Error()})
	}
	return err
}
```

- [ ] **Step 5: Run CLI tests**

Run:

```bash
go test ./internal/cli -run TestCandidateCodeAgent -v
```

Expected: PASS.

- [ ] **Step 6: Run root help test**

Run:

```bash
go test ./internal/cli -run 'TestRootHelp|TestRootUnknownCommand' -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/root.go internal/cli/candidate.go internal/cli/candidate_test.go
git commit -m "feat: 添加候选命令入口"
```

---

## Task 5: wiki-candidate Skill Documentation

**Files:**
- Create: `skill/wiki-candidate/SKILL.md`
- Create: `skill/wiki-candidate/references/codeagent-extraction-rules.md`
- Create: `skill/wiki-candidate/references/review-doc-protocol.md`

- [ ] **Step 1: Create skill directory**

Run:

```bash
mkdir -p skill/wiki-candidate/references
```

Expected: directory exists.

- [ ] **Step 2: Write `SKILL.md`**

Create `skill/wiki-candidate/SKILL.md`:

```markdown
---
name: wiki-candidate
description: Use when discovering candidate knowledge for OpenWiki review before ingest, especially from code agent sessions, conversation history, or agent memory files.
---
# Wiki Candidate

Discover candidate knowledge, create a Feishu review document, and let the user choose what may enter OpenWiki.

## Runtime Contract

- Use `openwiki.toml` as the runtime contract.
- Do not scan codeagent files manually.
- Use OpenWiki CLI candidate commands as the source of truth for config discovery, incremental scan state, pending files, and commit.
- Candidate files live under `<wiki_root>/candidate/` by default.
- This skill creates review material only; formal wiki writes are performed later by `wiki-ingest`.

## Pre-condition

Resolve the active config through OpenWiki CLI:

```bash
openwiki config path --json
openwiki config show --json
```

If the global CLI is unavailable or too old and this is the OpenWiki repository, use:

```bash
go run ./cmd/openwiki config path --json
go run ./cmd/openwiki config show --json
```

If neither works, ask the user to install/update OpenWiki CLI or provide an explicit `openwiki.toml` path.

## CodeAgent Candidate Flow

### 1. Scan through CLI

Run:

```bash
openwiki candidate codeagent scan --json
```

If the user provides a config path, pass it with `--config`:

```bash
openwiki --config /path/to/openwiki.toml candidate codeagent scan --json
```

Do not advance state yourself. The scan command creates a pending file and returns the pending path.

### 2. Read pending records

Read the returned pending JSON. If it contains zero records, report that no new codeagent records were found and stop without creating a Feishu document.

### 3. Load extraction rules

Read `references/codeagent-extraction-rules.md` before selecting candidates.

### 4. Extract candidates

Use balanced recall. Produce candidates only from the pending records. Do not perform candidate-level deduplication in this version.

Each candidate must include:

- `candidate_id` such as `CAND-001`
- `slug`
- `title`
- `category`
- `target_wiki_area`
- `reason`
- `proposed_content`
- `evidence`
- `risk_and_redaction`
- `original_links`

External URLs and Feishu document URLs must preserve their original links.

### 5. Load review document protocol

Read `references/review-doc-protocol.md` before creating the Feishu document.

### 6. Create Feishu review document

Use `lark-doc` v2. Before creation, follow `lark-doc` requirements for DocxXML creation.

The document must contain:

```text
OPENWIKI_CANDIDATE_REVIEW_DOC v1
source: codeagent
admission: ONLY_CHECKED_CANDIDATES
```

The document must use grouped candidate cards. The candidate card title checkbox is the only admission signal.

### 7. Save local candidate snapshot

Save a JSON snapshot under the scan result `snapshot_dir`, using a timestamped filename such as:

```text
<snapshot_dir>/2026-06-16-173000-candidates.json
```

The snapshot must include the review doc URL, pending path, protocol version, created time, and all candidates. It is for audit only and is not an admission source.

### 8. Commit scan state only after success

Only after both the Feishu review document and local snapshot are created successfully, run:

```bash
openwiki candidate codeagent commit --pending <pending.json> --review-doc-url <url> --snapshot <snapshot.json> --json
```

If the commit fails, report that the review document exists but scan state was not advanced.

### 9. Report

Tell the user:

- Feishu review document URL.
- candidate count.
- pending path.
- snapshot path.
- state path.
- next step: check desired candidates in the Feishu document, then give the link to `wiki-ingest`.

## Guardrails

- Never commit scan state before the review document and snapshot are created.
- Never treat unchecked candidates as accepted knowledge.
- Never write formal wiki pages from this skill.
- Preserve original external URLs and Feishu URLs.
- Redact secrets and personal information according to `codeagent-extraction-rules.md`.
```

- [ ] **Step 3: Write extraction rules**

Create `skill/wiki-candidate/references/codeagent-extraction-rules.md`:

```markdown
# CodeAgent Extraction Rules

Use balanced recall: include information with clear reuse value or strong evidence that the user may want to preserve, but explain the reason and risk for each candidate.

## Candidate Categories

1. 工具/产品使用说明 — product, CLI, plugin, skill, or platform usage.
2. 工作流/流程规范 — repeatable procedures and collaboration protocols.
3. 项目规则/团队约定 — naming, directory, config, commit, and review conventions.
4. 可复用问题排查经验 — diagnostics, log paths, verification commands, common failure causes.
5. 外部资料索引 — URLs, Feishu docs, wikis, PRDs, articles, API docs, and when to consult them.
6. 设计决策/架构知识 — design tradeoffs, module boundaries, and reasons for a structure.
7. 命令与配置片段 — reusable commands, config fields, environment variables, and script invocations.
8. 用户明确给出的知识材料 — user-provided rules, facts, background, standards, or decisions.

## Do Not Extract

- One-off task progress.
- Temporary plans or temporary TODOs.
- Unverified guesses.
- Pure operation logs.
- Chit-chat.
- Large code diffs.
- Details with no reuse value outside the current session.
- Sensitive content that cannot be safely redacted.
- Agent self-evaluation.
- Session summaries that only say what was done and contain no reusable knowledge.

## Redaction

Use medium redaction.

Must redact:

- token, password, private key, secret, cookie, authorization header.
- phone numbers, emails, personal accounts.
- local usernames in absolute paths, such as `/Users/<user>/...`.
- obvious internal credentials, long random access IDs, and temporary authorization links.
- URL query parameters named token, key, signature, auth, password, secret, cookie, or session.

May preserve:

- original external URLs.
- original Feishu document URLs.
- public API docs.
- command names, parameter names, and config fields.
- project-relative paths.
- non-sensitive errors and log path patterns.

Internal URLs may keep enough context to be useful, but redact sensitive query parameters and overlong identifiers. If the URL itself is a credential, replace it with `<redacted-url>`.

## Candidate Field Requirements

Every candidate must include:

```yaml
candidate_id: CAND-001
slug: lowercase-hyphen-slug
title: readable title
category: one of the eight categories
target_wiki_area: wiki/pages | concepts | entities
reason: why it is worth preserving
proposed_content: concise proposed wiki content
evidence:
  - agent: source agent name
    source_file: redacted path
    line_start: line number
    session_id: optional session id
    message_id: optional message id
    timestamp: optional timestamp
    quote: sanitized source quote
risk_and_redaction:
  - redaction or risk note
original_links:
  - original external or Feishu URL
```
```

- [ ] **Step 4: Write review doc protocol**

Create `skill/wiki-candidate/references/review-doc-protocol.md`:

```markdown
# Candidate Review Doc Protocol

A Candidate Review Doc is a Feishu document that lets the user explicitly choose which candidate knowledge may enter OpenWiki.

## Required Header

The document must contain this protocol block near the top:

```text
OPENWIKI_CANDIDATE_REVIEW_DOC v1
source: codeagent
admission: ONLY_CHECKED_CANDIDATES
```

Also include this warning in a prominent callout:

> Only checked candidate cards may be ingested by `wiki-ingest`. Unchecked cards are explicit user rejection and must not be summarized, merged, cited, or used as background material.

## Layout

Use grouped cards:

1. Title: `OpenWiki 候选知识审核 - CodeAgent`
2. Protocol block.
3. Scan summary: scan time, pending path, candidate count, source agents, limits.
4. How to review: check desired candidates, leave rejected candidates unchecked, give this link to `wiki-ingest`.
5. Candidate overview.
6. Category sections.
7. Candidate cards.
8. Next step.

## Candidate Card

The title line checkbox is the only admission signal:

```text
☐ CAND-001｜openwiki-runtime-discovery｜OpenWiki 配置发现规则
```

Each card must include:

- 建议分类
- 建议入库位置
- 拟 slug
- 为什么值得沉淀
- 拟入库内容
- 证据
- 原始链接
- 风险与脱敏

## Parse Contract

- `CAND-xxx` must be unique.
- The candidate block starts at a candidate title checkbox and ends before the next candidate title or category heading.
- Only the title checkbox is admission.
- Checkboxes inside the card body are ignored.
- Unchecked candidates are forbidden source material.
- If checkbox state cannot be reliably parsed, `wiki-ingest` must stop.

## Fallback

Prefer real Feishu checkbox blocks. If DocxXML checkbox support is unreliable, use textual markers in title lines:

```text
[ ] CAND-001｜slug｜title
[x] CAND-002｜slug｜title
```

When using textual markers, document that `[x]` is the only accepted state.
```

- [ ] **Step 5: Static verification**

Run:

```bash
grep -R "OPENWIKI_CANDIDATE_REVIEW_DOC v1" -n skill/wiki-candidate
grep -R "ONLY_CHECKED_CANDIDATES" -n skill/wiki-candidate
grep -R "openwiki candidate codeagent scan" -n skill/wiki-candidate
```

Expected: each grep prints at least one matching line.

- [ ] **Step 6: Commit**

```bash
git add skill/wiki-candidate/SKILL.md skill/wiki-candidate/references/codeagent-extraction-rules.md skill/wiki-candidate/references/review-doc-protocol.md
git commit -m "feat: 添加候选审核技能"
```

---

## Task 6: wiki-ingest Candidate Review Doc Contract

**Files:**
- Modify: `skill/wiki-ingest/SKILL.md`
- Create: `skill/wiki-ingest/references/candidate-review-doc.md`

- [ ] **Step 1: Add reference document**

Create `skill/wiki-ingest/references/candidate-review-doc.md`:

```markdown
# Candidate Review Doc Ingest Contract

Use this when a Feishu document contains:

```text
OPENWIKI_CANDIDATE_REVIEW_DOC v1
admission: ONLY_CHECKED_CANDIDATES
```

## Mandatory Behavior

- Treat the document as a candidate review document, not as an ordinary source document.
- Use `lark-doc` to read DocxXML so checkbox state can be inspected.
- Only candidate cards whose title checkbox is checked may be source material.
- Unchecked candidate cards are explicit user rejection.
- Do not summarize, merge, cite, quote, or use unchecked cards as background context.
- If no candidate cards are checked, stop and write nothing.
- Checked candidate cards represent prior user approval. Do not ask for a second confirmation before ingesting them.

## Candidate Boundary

A candidate starts at a title line or checkbox block matching:

```text
CAND-001｜slug｜title
```

It ends before the next candidate title or the next category heading.

Only the title checkbox controls admission. Ignore checkboxes inside the card body.

## Fallback Text Markers

If the document uses text markers, only `[x]` title lines are accepted. `[ ]` title lines are rejected.

## Failure Cases

- Protocol marker exists but `admission: ONLY_CHECKED_CANDIDATES` is missing: stop; do not ingest as a normal document.
- Checkbox state cannot be determined: stop; ask the user to regenerate the review doc or use `[x]` markers.
- Checked candidate lacks proposed content: skip that candidate and report it.
- Checked candidate lacks slug: generate a slug using `references/slug-rules.md` and report it.
- Checked candidate lacks evidence: ingest may continue, but mark the source as evidence-limited in the final report.
```

- [ ] **Step 2: Add guardrail to `wiki-ingest/SKILL.md`**

Insert this section immediately after the `### 1. Accept the source` source list in `skill/wiki-ingest/SKILL.md`:

```markdown
#### Candidate Review Doc Guardrail

If the source is a Feishu/Lark document, read enough of it with `lark-doc` to detect whether it contains:

- `OPENWIKI_CANDIDATE_REVIEW_DOC v1`
- `admission: ONLY_CHECKED_CANDIDATES`

If both markers are present, read `references/candidate-review-doc.md` and follow it strictly. Only checked candidate cards may be treated as source material. Unchecked candidates are explicit user rejection and must not be summarized, merged, cited, quoted, or used as background context.

For checked candidate cards, the user's checkbox is the confirmation to ingest. Do not ask for the normal second confirmation before writing wiki pages for those checked candidates. Continue with the existing page-writing, shard-index update, logging, verification, and reporting flow.

If `OPENWIKI_CANDIDATE_REVIEW_DOC v1` is present but `admission: ONLY_CHECKED_CANDIDATES` is missing, stop and ask the user to provide a valid Candidate Review Doc. Do not ingest it as a normal document.
```

- [ ] **Step 3: Static verification**

Run:

```bash
grep -n "Candidate Review Doc Guardrail" skill/wiki-ingest/SKILL.md
grep -n "Do not ask for the normal second confirmation" skill/wiki-ingest/SKILL.md
grep -n "Unchecked candidate cards are explicit user rejection" skill/wiki-ingest/references/candidate-review-doc.md
```

Expected: all greps print matching lines.

- [ ] **Step 4: Commit**

```bash
git add skill/wiki-ingest/SKILL.md skill/wiki-ingest/references/candidate-review-doc.md
git commit -m "feat: 支持候选文档入库"
```

---

## Task 7: End-to-End Verification

**Files:**
- No new production files required.
- May create temporary files under `$(mktemp -d)` only.

- [ ] **Step 1: Run unit tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Build CLI**

Run:

```bash
go build ./cmd/openwiki
```

Expected: PASS and local `openwiki` binary produced in repository root or build cache, depending on Go behavior.

- [ ] **Step 3: Manual CLI smoke test**

Run:

```bash
tmp="$(mktemp -d)"
mkdir -p "$tmp/wiki-root" "$tmp/sessions"
cat > "$tmp/sessions/history.jsonl" <<'JSONL'
{"session_id":"s1","ts":1781597600,"text":"openwiki config path --json 是配置发现源头，参考 https://example.com/openwiki-doc"}
{"session_id":"s1","ts":1781597700,"text":"这是一次性任务进展，不应该成为候选"}
JSONL
cat > "$tmp/openwiki.toml" <<EOF2
wiki_root = "$tmp/wiki-root"

[wiki]
primary_language = "zh"
secondary_language = "en"

[candidate.codeagent]
initial_days = 14
max_records_per_run = 500

[[candidate.codeagent.agents]]
name = "traex-test"
type = "traex-history"
paths = ["$tmp/sessions/history.jsonl"]
enabled = true
EOF2
go run ./cmd/openwiki --config "$tmp/openwiki.toml" candidate codeagent scan --json
```

Expected: JSON response with `"success": true`, `records.total` equal to `2`, and a `pending_path` under `$tmp/wiki-root/candidate/codeagent/pending`.

- [ ] **Step 4: Manual commit smoke test**

Use the `pending_path` from Step 3:

```bash
snapshot="$tmp/wiki-root/candidate/codeagent/reviews/snapshot.json"
mkdir -p "$(dirname "$snapshot")"
printf '{"candidates":[]}' > "$snapshot"
go run ./cmd/openwiki --config "$tmp/openwiki.toml" candidate codeagent commit --pending "<pending_path_from_step_3>" --review-doc-url "https://example.com/review-doc" --snapshot "$snapshot" --json
```

Expected: JSON response with `"success": true`; state file exists at `$tmp/wiki-root/candidate/codeagent/state.json`.

- [ ] **Step 5: Verify state status**

Run:

```bash
go run ./cmd/openwiki --config "$tmp/openwiki.toml" candidate codeagent status --json
```

Expected: JSON response with `"tracked_files": 1`.

- [ ] **Step 6: Verify skill docs contain protocol**

Run:

```bash
grep -R "OPENWIKI_CANDIDATE_REVIEW_DOC v1" -n skill/wiki-candidate skill/wiki-ingest
grep -R "ONLY_CHECKED_CANDIDATES" -n skill/wiki-candidate skill/wiki-ingest
grep -R "Do not ask for the normal second confirmation" -n skill/wiki-ingest
```

Expected: all grep commands print matching lines.

- [ ] **Step 7: Final status check**

Run:

```bash
git status --short
```

Expected: only intentional changes remain. Note that this repository may already have an unrelated modified `bin/openwiki`; do not include unrelated changes in commits.

- [ ] **Step 8: Final commit if verification changed tracked docs**

If Task 7 required any small fixes, commit them:

```bash
git add <fixed-files>
git commit -m "feat: 完成候选审核验证"
```

If no files changed, do not create an empty commit.

---

## Self-Review Checklist

- Spec coverage: configuration, CLI scan/commit/status, pending state, no candidate dedup, default candidate paths, codeagent parsers, Feishu review doc protocol, `wiki-ingest` checked-only behavior, and no second confirmation are all covered by tasks.
- Placeholder scan: plan contains no implementation placeholders. Temporary strings such as `<pending_path_from_step_3>` are explicit manual smoke-test substitutions.
- Type consistency: config structs, candidate structs, function names, command names, and JSON field names are consistent across tasks.
