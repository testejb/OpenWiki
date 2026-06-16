package candidate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/openwiki/internal/candidate"
)

func TestCommitRequiresReviewDocAndSnapshot(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath, `{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)
	result, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("ScanCodeAgent returned error: %v", err)
	}
	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")

	if _, err := candidate.CommitCodeAgent(result.PendingPath, "", snapshotPath, fixedCommitTime()); err == nil {
		t.Fatalf("expected empty review doc URL to fail")
	}
	if _, err := candidate.CommitCodeAgent(result.PendingPath, "https://example.com/review", filepath.Join(root, "missing.md"), fixedCommitTime()); err == nil {
		t.Fatalf("expected missing snapshot to fail")
	}
}

func TestCommitAdvancesStateAndMarksPending(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath, `{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)
	result, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("ScanCodeAgent returned error: %v", err)
	}
	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")

	commit, err := candidate.CommitCodeAgent(result.PendingPath, "https://example.com/review", snapshotPath, fixedCommitTime())

	if err != nil {
		t.Fatalf("CommitCodeAgent returned error: %v", err)
	}
	if commit.StatePath != cfg.StatePath {
		t.Errorf("expected state path %s, got %s", cfg.StatePath, commit.StatePath)
	}
	state := loadStateForTest(t, cfg.StatePath)
	if len(state.Files) != 1 {
		t.Fatalf("expected one state file, got %#v", state.Files)
	}
	fileState := state.Files[historyPath]
	if fileState.ProcessedBytes == 0 || fileState.ProcessedLines != 1 {
		t.Errorf("expected committed file progress, got %#v", fileState)
	}
	if state.UpdatedAt == "" {
		t.Errorf("expected state UpdatedAt")
	}

	pending := loadPendingForTest(t, result.PendingPath)
	if pending.Status != "committed" {
		t.Errorf("expected committed pending, got %q", pending.Status)
	}
	if pending.ReviewDocURL != "https://example.com/review" {
		t.Errorf("expected review doc URL saved, got %q", pending.ReviewDocURL)
	}
	if pending.SnapshotPath != snapshotPath {
		t.Errorf("expected snapshot path saved, got %q", pending.SnapshotPath)
	}
	if pending.CommittedAt == "" {
		t.Errorf("expected committed_at")
	}

	runLog, err := os.ReadFile(cfg.RunLogPath)
	if err != nil {
		t.Fatalf("read run log: %v", err)
	}
	if len(runLog) == 0 {
		t.Fatalf("expected non-empty run log")
	}
}
