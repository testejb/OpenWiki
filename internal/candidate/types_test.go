package candidate_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/openwiki/internal/candidate"
)

func TestCandidateDomainTypesJSONShape(t *testing.T) {
	now := time.Unix(1781600000, 0).UTC()
	warning := candidate.Warning{Code: "parse_error", Message: "bad line", Path: "history.jsonl", Line: 7}
	record := candidate.Record{
		RecordID:   "rec-1",
		Agent:      "traex",
		Type:       "traex-history",
		SourceFile: "history.jsonl",
		LineStart:  1,
		LineEnd:    2,
		ByteStart:  3,
		ByteEnd:    4,
		Timestamp:  now.Format(time.RFC3339),
		SessionID:  "session-1",
		MessageID:  "message-1",
		Intent:     "fix bug",
		Actions:    []string{"edit"},
		Outcome:    "done",
		Learned:    []string{"lesson"},
		Text:       "important text",
	}
	fileState := candidate.FileState{
		Agent:          "traex",
		Type:           "traex-history",
		FileID:         "file-1",
		Size:           100,
		MTime:          now.Format(time.RFC3339),
		ProcessedLines: 2,
		ProcessedBytes: 4,
		TailHash:       "hash",
		LastScannedAt:  now.Format(time.RFC3339),
	}
	state := candidate.State{
		Version:   1,
		Source:    "codeagent",
		UpdatedAt: now.Format(time.RFC3339),
		Files:     map[string]candidate.FileState{"history.jsonl": fileState},
	}
	pending := candidate.Pending{
		Version:   1,
		Source:    "codeagent",
		CreatedAt: now.Format(time.RFC3339),
		Status:    "pending",
		Config: candidate.PendingConfig{
			ConfigPath:  "openwiki.toml",
			WikiRoot:    "/wiki",
			StatePath:   "/wiki/candidate/codeagent/state.json",
			PendingDir:  "/wiki/candidate/codeagent/pending",
			RunLogPath:  "/wiki/candidate/codeagent/run.log",
			SnapshotDir: "/wiki/candidate/codeagent/reviews",
		},
		Limits:  candidate.PendingLimits{InitialDays: 14, MaxRecordsPerRun: 500},
		Records: []candidate.Record{record},
		StateUpdates: map[string]candidate.FileState{
			"history.jsonl": fileState,
		},
		Warnings:     []candidate.Warning{warning},
		ReviewDocURL: "https://example.com/doc",
		SnapshotPath: "/wiki/candidate/codeagent/reviews/review.json",
		CommittedAt:  now.Format(time.RFC3339),
	}
	scanOptions := candidate.ScanOptions{
		ConfigPath:       "openwiki.toml",
		WikiRoot:         "/wiki",
		Now:              now,
		InitialDays:      14,
		MaxRecordsPerRun: 500,
	}
	scanResult := candidate.ScanResult{
		PendingPath: "/wiki/candidate/codeagent/pending/pending.json",
		StatePath:   "/wiki/candidate/codeagent/state.json",
		RunLogPath:  "/wiki/candidate/codeagent/run.log",
		SnapshotDir: "/wiki/candidate/codeagent/reviews",
		Records:     candidate.ScanRecordSummary{Total: 1, ByAgent: map[string]int{"traex": 1}},
		Limits:      candidate.PendingLimits{InitialDays: scanOptions.InitialDays, MaxRecordsPerRun: scanOptions.MaxRecordsPerRun},
		Warnings:    []candidate.Warning{warning},
	}
	commitResult := candidate.CommitResult{
		PendingPath:  scanResult.PendingPath,
		StatePath:    scanResult.StatePath,
		ReviewDocURL: pending.ReviewDocURL,
		SnapshotPath: pending.SnapshotPath,
		CommittedAt:  pending.CommittedAt,
	}
	statusResult := candidate.StatusResult{
		Enabled:       true,
		StatePath:     scanResult.StatePath,
		PendingDir:    pending.Config.PendingDir,
		RunLogPath:    scanResult.RunLogPath,
		SnapshotDir:   scanResult.SnapshotDir,
		TrackedFiles:  len(state.Files),
		LastUpdatedAt: state.UpdatedAt,
		Agents:        []candidate.AgentConfig{{Name: "traex", Type: "traex-history", Paths: []string{"history.jsonl"}, Enabled: true}},
		Warnings:      []candidate.Warning{warning},
	}

	payload, err := json.Marshal(struct {
		State        candidate.State        `json:"state"`
		Pending      candidate.Pending      `json:"pending"`
		ScanResult   candidate.ScanResult   `json:"scan_result"`
		CommitResult candidate.CommitResult `json:"commit_result"`
		StatusResult candidate.StatusResult `json:"status_result"`
	}{
		State:        state,
		Pending:      pending,
		ScanResult:   scanResult,
		CommitResult: commitResult,
		StatusResult: statusResult,
	})
	if err != nil {
		t.Fatalf("failed to marshal candidate types: %v", err)
	}
	output := string(payload)
	for _, want := range []string{
		`"record_id":"rec-1"`,
		`"file_id":"file-1"`,
		`"processed_lines":2`,
		`"state_updates"`,
		`"max_records_per_run":500`,
		`"pending_path"`,
		`"review_doc_url"`,
		`"last_updated_at"`,
		`"tracked_files":1`,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected marshaled JSON to contain %s, got %s", want, output)
		}
	}
}
