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
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath, `{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)

	result, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})

	if err != nil {
		t.Fatalf("ScanCodeAgent returned error: %v", err)
	}
	if result.Records.Total != 1 {
		t.Fatalf("expected 1 scanned record, got %d", result.Records.Total)
	}
	if result.Records.ByAgent["traex"] != 1 {
		t.Errorf("expected traex record count 1, got %#v", result.Records.ByAgent)
	}
	if result.PendingPath == "" {
		t.Fatalf("expected pending path")
	}
	if _, err := os.Stat(result.PendingPath); err != nil {
		t.Fatalf("expected pending file to exist: %v", err)
	}
	if _, err := os.Stat(cfg.StatePath); !os.IsNotExist(err) {
		t.Fatalf("expected scan not to create or advance state, stat err=%v", err)
	}

	pending := loadPendingForTest(t, result.PendingPath)
	if pending.Status != "pending" {
		t.Errorf("expected pending status, got %q", pending.Status)
	}
	if len(pending.Records) != 1 {
		t.Fatalf("expected pending records=1, got %d", len(pending.Records))
	}
	if len(pending.StateUpdates) != 1 {
		t.Fatalf("expected pending state updates, got %#v", pending.StateUpdates)
	}
	update := pending.StateUpdates[historyPath]
	info, err := os.Stat(historyPath)
	if err != nil {
		t.Fatalf("stat history: %v", err)
	}
	if update.ProcessedBytes != info.Size() {
		t.Errorf("expected processed bytes %d, got %d", info.Size(), update.ProcessedBytes)
	}
	if update.ProcessedLines != 1 {
		t.Errorf("expected processed lines 1, got %d", update.ProcessedLines)
	}
}

func TestScanCodeAgentDisabledCreatesEmptyPendingWithoutReadingAgents(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath, `{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)
	cfg.Enabled = false
	cfg.Agents = append(cfg.Agents, candidate.AgentConfig{
		Name:    "missing",
		Type:    "traex-history",
		Paths:   []string{filepath.Join(root, "missing.jsonl")},
		Enabled: true,
	})

	result, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})

	if err != nil {
		t.Fatalf("ScanCodeAgent returned error: %v", err)
	}
	if result.Records.Total != 0 {
		t.Fatalf("expected disabled scan to return 0 records, got %d", result.Records.Total)
	}
	if len(result.Records.ByAgent) != 0 {
		t.Fatalf("expected disabled scan to have empty agent summary, got %#v", result.Records.ByAgent)
	}
	if result.PendingPath == "" {
		t.Fatalf("expected disabled scan to still create pending path")
	}
	pending := loadPendingForTest(t, result.PendingPath)
	if len(pending.Records) != 0 {
		t.Fatalf("expected disabled pending records to be empty, got %#v", pending.Records)
	}
	if len(pending.StateUpdates) != 0 {
		t.Fatalf("expected disabled pending state updates to be empty, got %#v", pending.StateUpdates)
	}
	if len(pending.Warnings) != 0 {
		t.Fatalf("expected disabled scan warnings to be empty, got %#v", pending.Warnings)
	}
}

func TestScanAppendUsesProcessedBytes(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath, `{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)

	first, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("first ScanCodeAgent returned error: %v", err)
	}
	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")
	if _, err := candidate.CommitCodeAgent(first.PendingPath, "https://example.com/review", snapshotPath, fixedCommitTime()); err != nil {
		t.Fatalf("CommitCodeAgent returned error: %v", err)
	}

	appendCandidateFile(t, historyPath, `{"session_id":"s2","ts":1781597660,"text":"第二条记录"}`+"\n")
	second, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime().Add(time.Hour), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("second ScanCodeAgent returned error: %v", err)
	}

	pending := loadPendingForTest(t, second.PendingPath)
	if second.Records.Total != 1 || len(pending.Records) != 1 {
		t.Fatalf("expected only appended record, result=%#v pending records=%#v", second.Records, pending.Records)
	}
	if pending.Records[0].Text != "第二条记录" {
		t.Errorf("expected appended second record, got %q", pending.Records[0].Text)
	}
	if pending.Records[0].LineStart != 2 {
		t.Errorf("expected resumed line 2, got %d", pending.Records[0].LineStart)
	}
	state := loadStateForTest(t, cfg.StatePath)
	oldBytes := state.Files[historyPath].ProcessedBytes
	newBytes := pending.StateUpdates[historyPath].ProcessedBytes
	if newBytes <= oldBytes {
		t.Errorf("expected pending state update to advance bytes beyond %d, got %d", oldBytes, newBytes)
	}
}

func testCodeAgentConfig(root, historyPath string) candidate.CodeAgentConfig {
	return candidate.CodeAgentConfig{
		Enabled:          true,
		StatePath:        filepath.Join(root, "state", "state.json"),
		PendingDir:       filepath.Join(root, "pending"),
		RunLogPath:       filepath.Join(root, "run.log"),
		SnapshotDir:      filepath.Join(root, "reviews"),
		InitialDays:      14,
		MaxRecordsPerRun: 500,
		Agents: []candidate.AgentConfig{{
			Name:    "traex",
			Type:    "traex-history",
			Paths:   []string{historyPath},
			Enabled: true,
		}},
	}
}

func fixedScanTime() time.Time {
	return time.Date(2026, 6, 16, 10, 11, 12, 0, time.UTC)
}

func fixedCommitTime() time.Time {
	return time.Date(2026, 6, 16, 11, 12, 13, 0, time.UTC)
}

func loadPendingForTest(t *testing.T, path string) candidate.Pending {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pending: %v", err)
	}
	var pending candidate.Pending
	if err := json.Unmarshal(data, &pending); err != nil {
		t.Fatalf("unmarshal pending: %v", err)
	}
	return pending
}

func loadStateForTest(t *testing.T, path string) candidate.State {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var state candidate.State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	return state
}

func writeCandidateFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func appendCandidateFile(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open append %s: %v", path, err)
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("append %s: %v", path, err)
	}
}

func TestScanMaxRecordsKeepsLatestRecordsAndAdvancesToLatestPendingBoundary(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath,
		`{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n"+
			`{"session_id":"s2","ts":1781597660,"text":"第二条记录"}`+"\n"+
			`{"session_id":"s3","ts":1781597720,"text":"第三条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)

	result, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 2})
	if err != nil {
		t.Fatalf("ScanCodeAgent returned error: %v", err)
	}
	pending := loadPendingForTest(t, result.PendingPath)
	if len(pending.Records) != 2 {
		t.Fatalf("expected maxRecords to keep 2 records, got %d", len(pending.Records))
	}
	if pending.Records[0].Text != "第二条记录" || pending.Records[1].Text != "第三条记录" {
		t.Fatalf("expected latest two unprocessed records, got %#v", pending.Records)
	}
	update := pending.StateUpdates[historyPath]
	if update.ProcessedBytes != pending.Records[1].ByteEnd {
		t.Fatalf("expected state update bytes to stop at latest pending record %d, got %d", pending.Records[1].ByteEnd, update.ProcessedBytes)
	}
	if update.ProcessedLines != pending.Records[1].LineEnd {
		t.Fatalf("expected state update lines to stop at latest pending record %d, got %d", pending.Records[1].LineEnd, update.ProcessedLines)
	}

	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")
	if _, err := candidate.CommitCodeAgent(result.PendingPath, "https://example.com/review", snapshotPath, fixedCommitTime()); err != nil {
		t.Fatalf("CommitCodeAgent returned error: %v", err)
	}
	state := loadStateForTest(t, cfg.StatePath)
	if state.Files[historyPath].ProcessedBytes != pending.Records[1].ByteEnd {
		t.Fatalf("expected committed state to advance to latest pending record, got %#v", state.Files[historyPath])
	}

	next, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime().Add(time.Hour), InitialDays: 30, MaxRecordsPerRun: 2})
	if err != nil {
		t.Fatalf("next ScanCodeAgent returned error: %v", err)
	}
	nextPending := loadPendingForTest(t, next.PendingPath)
	if len(nextPending.Records) != 0 {
		t.Fatalf("expected next scan not to repeat records already included in pending, got %#v", nextPending.Records)
	}
}

func TestScanDetectsRewriteBeforeAppend(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath, `{"session_id":"old","ts":1781597600,"text":"旧记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)
	first, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("first ScanCodeAgent returned error: %v", err)
	}
	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")
	if _, err := candidate.CommitCodeAgent(first.PendingPath, "https://example.com/review", snapshotPath, fixedCommitTime()); err != nil {
		t.Fatalf("CommitCodeAgent returned error: %v", err)
	}

	writeCandidateFile(t, historyPath,
		`{"session_id":"new1","ts":1781597660,"text":"新记录一"}`+"\n"+
			`{"session_id":"new2","ts":1781597720,"text":"新记录二"}`+"\n")
	second, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime().Add(time.Hour), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("second ScanCodeAgent returned error: %v", err)
	}
	pending := loadPendingForTest(t, second.PendingPath)
	if len(pending.Records) != 2 || pending.Records[0].Text != "新记录一" || pending.Records[0].LineStart != 1 {
		t.Fatalf("expected rewritten file to scan from start, got %#v", pending.Records)
	}
	if !hasWarningCode(pending.Warnings, "SOURCE_FILE_RESET") && !hasWarningCode(pending.Warnings, "SOURCE_FILE_REWRITTEN") {
		t.Fatalf("expected rewrite/reset warning, got %#v", pending.Warnings)
	}
}

func TestScanCreatesUniquePendingPathsWithinSameSecond(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath, `{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)

	first, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("first ScanCodeAgent returned error: %v", err)
	}
	second, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("second ScanCodeAgent returned error: %v", err)
	}
	if first.PendingPath == second.PendingPath {
		t.Fatalf("expected unique pending paths within same second, got %s", first.PendingPath)
	}
	if _, err := os.Stat(first.PendingPath); err != nil {
		t.Fatalf("expected first pending file to exist: %v", err)
	}
	if _, err := os.Stat(second.PendingPath); err != nil {
		t.Fatalf("expected second pending file to exist: %v", err)
	}
}

func hasWarningCode(warnings []candidate.Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
