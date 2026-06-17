package candidate_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestCommitRejectsStaleBacklogPending(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath,
		`{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n"+
			`{"session_id":"s2","ts":1781597660,"text":"第二条记录"}`+"\n"+
			`{"session_id":"s3","ts":1781597720,"text":"第三条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)
	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")

	initial, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 2})
	if err != nil {
		t.Fatalf("initial ScanCodeAgent returned error: %v", err)
	}
	if _, err := candidate.CommitCodeAgent(initial.PendingPath, "https://example.com/review-initial", snapshotPath, fixedCommitTime()); err != nil {
		t.Fatalf("initial CommitCodeAgent returned error: %v", err)
	}

	pending1, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime().Add(time.Minute), InitialDays: 30, MaxRecordsPerRun: 2})
	if err != nil {
		t.Fatalf("first backlog ScanCodeAgent returned error: %v", err)
	}
	pending2, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime().Add(time.Minute + time.Second), InitialDays: 30, MaxRecordsPerRun: 2})
	if err != nil {
		t.Fatalf("second backlog ScanCodeAgent returned error: %v", err)
	}

	if _, err := candidate.CommitCodeAgent(pending1.PendingPath, "https://example.com/review-1", snapshotPath, fixedCommitTime().Add(time.Minute)); err != nil {
		t.Fatalf("commit first backlog pending returned error: %v", err)
	}
	if _, err := candidate.CommitCodeAgent(pending2.PendingPath, "https://example.com/review-2", snapshotPath, fixedCommitTime().Add(time.Minute+time.Second)); err == nil {
		t.Fatalf("expected stale backlog pending commit to fail")
	}
}

func TestCommitRejectsStalePending(t *testing.T) {
	root := t.TempDir()
	historyPath := filepath.Join(root, "history.jsonl")
	writeCandidateFile(t, historyPath, `{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`+"\n")
	cfg := testCodeAgentConfig(root, historyPath)
	pending1, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime(), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("first ScanCodeAgent returned error: %v", err)
	}

	appendCandidateFile(t, historyPath, `{"session_id":"s2","ts":1781597660,"text":"第二条记录"}`+"\n")
	pending2, err := candidate.ScanCodeAgent(cfg, candidate.ScanOptions{Now: fixedScanTime().Add(time.Second), InitialDays: 30, MaxRecordsPerRun: 10})
	if err != nil {
		t.Fatalf("second ScanCodeAgent returned error: %v", err)
	}
	snapshotPath := filepath.Join(root, "snapshot.md")
	writeCandidateFile(t, snapshotPath, "# snapshot\n")
	if _, err := candidate.CommitCodeAgent(pending2.PendingPath, "https://example.com/review-new", snapshotPath, fixedCommitTime()); err != nil {
		t.Fatalf("commit newer pending returned error: %v", err)
	}
	if _, err := candidate.CommitCodeAgent(pending1.PendingPath, "https://example.com/review-old", snapshotPath, fixedCommitTime().Add(time.Second)); err == nil {
		t.Fatalf("expected stale pending commit to fail")
	}
	state := loadStateForTest(t, cfg.StatePath)
	if state.Files[historyPath].ProcessedLines != 2 {
		t.Fatalf("expected state to remain at newer progress, got %#v", state.Files[historyPath])
	}
}
