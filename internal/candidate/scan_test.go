package candidate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestScanDoesNotAdvancePastIncompleteTrailingLine(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	completeLine := `{"session_id":"s1","ts":1781597600,"text":"第一条记录"}` + "\n"
	incompleteLine := `{"session_id":"s2","ts":1781597660,"text":`
	writeCandidateFile(t, historyPath, completeLine+incompleteLine)
	cfg := testCodeAgentConfig(root, historyPath)

	first, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("first ScanCodeAgent returned error: %v", err)
	}
	firstPending := loadPendingForTest(t, first.PendingPath)
	if len(firstPending.Records) != 1 {
		t.Fatalf("expected only the complete line in pending, got %#v", firstPending.Records)
	}
	if firstPending.Records[0].Text != "第一条记录" {
		t.Fatalf("expected first complete record, got %#v", firstPending.Records[0])
	}
	if hasWarningCode(firstPending.Warnings, "JSONL_PARSE_FAILED") {
		t.Fatalf("expected incomplete trailing line not to produce parse warning, got %#v", firstPending.Warnings)
	}
	update := firstPending.StateUpdates[historyPath]
	if update.ProcessedBytes != int64(len(completeLine)) {
		t.Fatalf("expected processed bytes to stop at complete line boundary %d, got %d", len(completeLine), update.ProcessedBytes)
	}
	if info, err := os.Stat(historyPath); err != nil {
		t.Fatalf("stat history: %v", err)
	} else if update.ProcessedBytes == info.Size() {
		t.Fatalf("expected processed bytes not to advance to EOF %d", info.Size())
	}
	if update.ProcessedLines != 1 {
		t.Fatalf("expected processed lines to count only complete newline lines, got %d", update.ProcessedLines)
	}

	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")
	if _, err := candidate.CommitCodeAgent(first.PendingPath, "https://example.com/review-1", snapshotPath, fixedCommitTime()); err != nil {
		t.Fatalf("first CommitCodeAgent returned error: %v", err)
	}

	appendCandidateFile(t, historyPath, `"第二条记录"}`+"\n")
	second, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime().Add(time.Hour), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("second ScanCodeAgent returned error: %v", err)
	}
	secondPending := loadPendingForTest(t, second.PendingPath)
	if len(secondPending.Records) != 1 {
		t.Fatalf("expected completed trailing line to be parsed on next scan, got %#v", secondPending.Records)
	}
	if secondPending.Records[0].Text != "第二条记录" {
		t.Fatalf("expected second record after completion, got %#v", secondPending.Records[0])
	}
	if secondPending.Records[0].LineStart != 2 {
		t.Fatalf("expected completed record to keep line 2, got %d", secondPending.Records[0].LineStart)
	}
}

func TestScanResetAppliesInitialDaysFilter(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath, `{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)
	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")

	first, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("first ScanCodeAgent returned error: %v", err)
	}
	if _, err := candidate.CommitCodeAgent(first.PendingPath, "https://example.com/review-1", snapshotPath, fixedCommitTime()); err != nil {
		t.Fatalf("first CommitCodeAgent returned error: %v", err)
	}

	oldRecordTime := fixedScanTime().AddDate(0, 0, -31)
	writeCandidateFile(t, historyPath, `{"session_id":"old","ts":`+strconv.FormatInt(oldRecordTime.Unix(), 10)+`,"text":"旧时间记录"}`+"\n")
	if err := os.Chtimes(historyPath, fixedScanTime(), fixedScanTime()); err != nil {
		t.Fatalf("chtimes history: %v", err)
	}

	second, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("second ScanCodeAgent returned error: %v", err)
	}
	if second.Records.Total != 0 {
		t.Fatalf("expected reset scan to filter old records, got %d", second.Records.Total)
	}
	pending := loadPendingForTest(t, second.PendingPath)
	if len(pending.Records) != 0 {
		t.Fatalf("expected reset pending records to be empty, got %#v", pending.Records)
	}
	if len(pending.BacklogUpdate) != 0 {
		t.Fatalf("expected reset old records not to enter backlog, got %#v", pending.BacklogUpdate)
	}
	if !hasWarningCode(pending.Warnings, "SOURCE_FILE_RESET") {
		t.Fatalf("expected SOURCE_FILE_RESET warning, got %#v", pending.Warnings)
	}
}

func TestScanDisabledPendingDoesNotSerializeBacklog(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath,
		`{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n"+
			`{"session_id":"s2","ts":1781597660,"text":"第二条记录"}`+"\n"+
			`{"session_id":"s3","ts":1781597720,"text":"第三条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)
	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")

	first, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 2})
	if err != nil {
		t.Fatalf("first ScanCodeAgent returned error: %v", err)
	}
	if _, err := candidate.CommitCodeAgent(first.PendingPath, "https://example.com/review-1", snapshotPath, fixedCommitTime()); err != nil {
		t.Fatalf("first CommitCodeAgent returned error: %v", err)
	}
	stateBefore := loadStateForTest(t, cfg.StatePath)
	if len(stateBefore.Backlog) == 0 {
		t.Fatalf("test setup expected backlog, got %#v", stateBefore.Backlog)
	}

	cfg.Enabled = false
	disabled, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime().Add(time.Minute), InitialDays: 30, MaxRecordsPerRun: 2})
	if err != nil {
		t.Fatalf("disabled ScanCodeAgent returned error: %v", err)
	}
	if disabled.Records.Total != 0 {
		t.Fatalf("expected disabled scan to expose 0 records, got %d", disabled.Records.Total)
	}
	pending := loadPendingForTest(t, disabled.PendingPath)
	if len(pending.Records) != 0 {
		t.Fatalf("expected disabled pending records to be empty, got %#v", pending.Records)
	}
	if len(pending.BacklogUpdate) != 0 {
		t.Fatalf("expected disabled pending not to carry backlog update, got %#v", pending.BacklogUpdate)
	}
	rawPending, err := os.ReadFile(disabled.PendingPath)
	if err != nil {
		t.Fatalf("read disabled pending: %v", err)
	}
	if strings.Contains(string(rawPending), "backlog_update") {
		t.Fatalf("expected disabled pending JSON not to serialize backlog_update, got %s", rawPending)
	}
	for _, record := range stateBefore.Backlog {
		if strings.Contains(string(rawPending), record.Text) {
			t.Fatalf("expected disabled pending JSON not to expose backlog record %q, got %s", record.Text, rawPending)
		}
	}

	if _, err := candidate.CommitCodeAgent(disabled.PendingPath, "https://example.com/review-disabled", snapshotPath, fixedCommitTime().Add(time.Minute)); err != nil {
		t.Fatalf("disabled CommitCodeAgent returned error: %v", err)
	}
	stateAfter := loadStateForTest(t, cfg.StatePath)
	if len(stateAfter.Backlog) != len(stateBefore.Backlog) {
		t.Fatalf("expected disabled commit not to clear backlog, got %#v want %#v", stateAfter.Backlog, stateBefore.Backlog)
	}
	for i := range stateBefore.Backlog {
		if stateAfter.Backlog[i].Text != stateBefore.Backlog[i].Text {
			t.Fatalf("expected disabled commit to preserve backlog, got %#v want %#v", stateAfter.Backlog, stateBefore.Backlog)
		}
	}
}

func TestExpandAgentPathsSupportsDoubleStarRecursive(t *testing.T) {
	root := t.TempDir()
	memoryPath := filepath.Join(root, "projects", "proj", "20260616", "session_memory_x.jsonl")
	writeCandidateFile(t, memoryPath, `{"intent":"修复递归扫描","actions":["添加测试"],"outcome":"找到深层文件","learned":["doublestar"]}`+"\n")

	cfg := testCodeAgentConfig(root, filepath.Join(root, "unused-history.jsonl"))
	cfg.Agents = []candidate.AgentConfig{{
		Name:    "trae-ide",
		Type:    "trae-ide-memory",
		Paths:   []string{filepath.Join(root, "projects", "**", "session_memory_*.jsonl")},
		Enabled: true,
	}}

	result, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})

	if err != nil {
		t.Fatalf("ScanCodeAgent returned error: %v", err)
	}
	if result.Records.Total != 1 {
		t.Fatalf("expected doublestar scan to find 1 record, got %d with warnings %#v", result.Records.Total, result.Warnings)
	}
	pending := loadPendingForTest(t, result.PendingPath)
	if len(pending.Records) != 1 {
		t.Fatalf("expected one pending record, got %#v", pending.Records)
	}
	if pending.Records[0].SourceFile != memoryPath {
		t.Fatalf("expected source file %s, got %s", memoryPath, pending.Records[0].SourceFile)
	}
	if !strings.Contains(pending.Records[0].Text, "修复递归扫描") {
		t.Fatalf("expected parsed memory text, got %#v", pending.Records[0])
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

func assertPendingTexts(t *testing.T, pending candidate.Pending, want []string) {
	t.Helper()
	if len(pending.Records) != len(want) {
		t.Fatalf("expected %d records, got %#v", len(want), pending.Records)
	}
	for i, text := range want {
		if pending.Records[i].Text != text {
			t.Fatalf("expected texts %#v, got %#v", want, pending.Records)
		}
	}
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

func TestScanMaxRecordsKeepsLatestAndStoresSkippedEarlierRecordsInBacklog(t *testing.T) {
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
	if _, ok := pending.StateUpdates[historyPath]; !ok {
		t.Fatalf("expected state update to advance scanned file while skipped records are preserved in backlog")
	}
	if len(pending.BacklogUpdate) != 1 || pending.BacklogUpdate[0].Text != "第一条记录" {
		t.Fatalf("expected skipped earlier record in backlog update, got %#v", pending.BacklogUpdate)
	}

	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")
	if _, err := candidate.CommitCodeAgent(result.PendingPath, "https://example.com/review", snapshotPath, fixedCommitTime()); err != nil {
		t.Fatalf("CommitCodeAgent returned error: %v", err)
	}
	state := loadStateForTest(t, cfg.StatePath)
	info, err := os.Stat(historyPath)
	if err != nil {
		t.Fatalf("stat history: %v", err)
	}
	if state.Files[historyPath].ProcessedBytes != info.Size() || state.Files[historyPath].ProcessedLines != 3 {
		t.Fatalf("expected committed state to advance file to EOF, got %#v", state.Files[historyPath])
	}
	if len(state.Backlog) != 1 || state.Backlog[0].Text != "第一条记录" {
		t.Fatalf("expected committed state backlog to retain skipped record, got %#v", state.Backlog)
	}
}

func TestScanMaxRecordsBacklogPreventsRepeatingLatestRecords(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath,
		`{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n"+
			`{"session_id":"s2","ts":1781597660,"text":"第二条记录"}`+"\n"+
			`{"session_id":"s3","ts":1781597720,"text":"第三条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)
	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")

	first, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 2})
	if err != nil {
		t.Fatalf("first ScanCodeAgent returned error: %v", err)
	}
	firstPending := loadPendingForTest(t, first.PendingPath)
	assertPendingTexts(t, firstPending, []string{"第二条记录", "第三条记录"})
	if _, err := candidate.CommitCodeAgent(first.PendingPath, "https://example.com/review-1", snapshotPath, fixedCommitTime()); err != nil {
		t.Fatalf("first CommitCodeAgent returned error: %v", err)
	}

	second, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime().Add(time.Minute), InitialDays: 30, MaxRecordsPerRun: 2})
	if err != nil {
		t.Fatalf("second ScanCodeAgent returned error: %v", err)
	}
	secondPending := loadPendingForTest(t, second.PendingPath)
	assertPendingTexts(t, secondPending, []string{"第一条记录"})
	if _, err := candidate.CommitCodeAgent(second.PendingPath, "https://example.com/review-2", snapshotPath, fixedCommitTime().Add(time.Minute)); err != nil {
		t.Fatalf("second CommitCodeAgent returned error: %v", err)
	}

	third, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime().Add(2 * time.Minute), InitialDays: 30, MaxRecordsPerRun: 2})
	if err != nil {
		t.Fatalf("third ScanCodeAgent returned error: %v", err)
	}
	thirdPending := loadPendingForTest(t, third.PendingPath)
	if third.Records.Total != 0 || len(thirdPending.Records) != 0 {
		t.Fatalf("expected third scan to have no records, result=%#v pending=%#v", third.Records, thirdPending.Records)
	}
	state := loadStateForTest(t, cfg.StatePath)
	info, err := os.Stat(historyPath)
	if err != nil {
		t.Fatalf("stat history: %v", err)
	}
	fileState := state.Files[historyPath]
	if fileState.ProcessedBytes != info.Size() || fileState.ProcessedLines != 3 {
		t.Fatalf("expected state tracked file at EOF, got %#v size=%d", fileState, info.Size())
	}
}

func TestScanFirstRunSkipsTimestamplessOldFile(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath, `{"session_id":"s1","ts":0,"text":"无时间记录"}`+"\n")
	oldTime := fixedScanTime().AddDate(0, 0, -31)
	if err := os.Chtimes(historyPath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes history: %v", err)
	}
	cfg := testCodeAgentConfig(root, historyPath)

	result, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("ScanCodeAgent returned error: %v", err)
	}
	if result.Records.Total != 0 {
		t.Fatalf("expected old timestampless file to produce 0 records, got %d", result.Records.Total)
	}
	pending := loadPendingForTest(t, result.PendingPath)
	if len(pending.Records) != 0 {
		t.Fatalf("expected old timestampless pending records to be empty, got %#v", pending.Records)
	}
	if !hasWarningCode(pending.Warnings, "TIMESTAMPLESS_OLD_FILE_SKIPPED") {
		t.Fatalf("expected TIMESTAMPLESS_OLD_FILE_SKIPPED warning, got %#v", pending.Warnings)
	}
}

func TestScanFirstRunIncludesTimestamplessRecentFile(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath, `{"session_id":"s1","ts":0,"text":"无时间记录"}`+"\n")
	recentTime := fixedScanTime().AddDate(0, 0, -1)
	if err := os.Chtimes(historyPath, recentTime, recentTime); err != nil {
		t.Fatalf("chtimes history: %v", err)
	}
	cfg := testCodeAgentConfig(root, historyPath)

	result, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("ScanCodeAgent returned error: %v", err)
	}
	if result.Records.Total != 1 {
		t.Fatalf("expected recent timestampless file to produce 1 record, got %d", result.Records.Total)
	}
	pending := loadPendingForTest(t, result.PendingPath)
	if len(pending.Records) != 1 {
		t.Fatalf("expected one pending record, got %#v", pending.Records)
	}
	if pending.Records[0].Timestamp != "" || pending.Records[0].Text != "无时间记录" {
		t.Fatalf("expected timestampless record to be included, got %#v", pending.Records[0])
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
