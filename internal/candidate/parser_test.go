package candidate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/openwiki/internal/candidate"
)

func TestParseTraexHistoryJSONL(t *testing.T) {
	path := writeJSONL(t, "history.jsonl",
		`{"session_id":"s1","ts":1781597600,"text":"第一条记录"}`,
		`{"session_id":"s2","ts":1781597660,"text":"第二条记录"}`,
	)
	agent := candidate.AgentConfig{Name: "traex", Type: "traex-history"}

	records, warnings, err := candidate.ParseJSONLFile(agent, path, 0, 0)

	if err != nil {
		t.Fatalf("ParseJSONLFile returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d: %#v", len(records), records)
	}

	first := records[0]
	if first.Agent != "traex" || first.Type != "traex-history" || first.SourceFile != path {
		t.Errorf("unexpected first record metadata: %#v", first)
	}
	if first.LineStart != 1 || first.LineEnd != 1 {
		t.Errorf("expected first record line 1, got %d-%d", first.LineStart, first.LineEnd)
	}
	if !strings.Contains(first.RecordID, "traex:"+path+":line:1") {
		t.Errorf("expected first record id to include agent/path/line, got %q", first.RecordID)
	}
	if first.SessionID != "s1" {
		t.Errorf("expected session_id s1, got %q", first.SessionID)
	}
	wantTimestamp := time.Unix(1781597600, 0).UTC().Format(time.RFC3339)
	if first.Timestamp != wantTimestamp {
		t.Errorf("expected timestamp %s, got %s", wantTimestamp, first.Timestamp)
	}
	if first.Text != "第一条记录" {
		t.Errorf("expected first text, got %q", first.Text)
	}

	second := records[1]
	if second.LineStart != 2 || second.LineEnd != 2 {
		t.Errorf("expected second record line 2, got %d-%d", second.LineStart, second.LineEnd)
	}
	if second.ByteStart <= first.ByteStart {
		t.Errorf("expected second byte start to advance from first, got first=%d second=%d", first.ByteStart, second.ByteStart)
	}
	if second.ByteEnd <= first.ByteEnd {
		t.Errorf("expected second byte end to advance from first, got first=%d second=%d", first.ByteEnd, second.ByteEnd)
	}
}

func TestParseTraeIDEMemoryJSONL(t *testing.T) {
	path := writeJSONL(t, "session_memory_1.jsonl",
		`{"intent":"修复解析问题","actions":["添加测试","实现解析器"],"outcome":"测试通过","learned":["JSONL 按行解析"],"message_summary_time":"2026-06-04 14:29:35","message_id":"m1"}`,
	)
	agent := candidate.AgentConfig{Name: "trae-ide", Type: "trae-ide-memory"}

	records, warnings, err := candidate.ParseJSONLFile(agent, path, 0, 0)

	if err != nil {
		t.Fatalf("ParseJSONLFile returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d: %#v", len(records), records)
	}

	record := records[0]
	if record.MessageID != "m1" {
		t.Errorf("expected message_id m1, got %q", record.MessageID)
	}
	if record.Intent != "修复解析问题" {
		t.Errorf("expected intent, got %q", record.Intent)
	}
	if len(record.Actions) != 2 || record.Actions[0] != "添加测试" || record.Actions[1] != "实现解析器" {
		t.Errorf("unexpected actions: %#v", record.Actions)
	}
	if record.Outcome != "测试通过" {
		t.Errorf("expected outcome, got %q", record.Outcome)
	}
	if len(record.Learned) != 1 || record.Learned[0] != "JSONL 按行解析" {
		t.Errorf("unexpected learned: %#v", record.Learned)
	}
	if !strings.Contains(record.Text, "修复解析问题") || !strings.Contains(record.Text, "测试通过") {
		t.Errorf("expected text to include intent and outcome, got %q", record.Text)
	}
}

func TestParseJSONLFileSkipsMalformedLines(t *testing.T) {
	path := writeJSONL(t, "history.jsonl",
		`{"session_id":"s1","ts":1781597600,"text":`,
		`{"session_id":"s2","ts":1781597660,"text":"有效记录"}`,
	)
	agent := candidate.AgentConfig{Name: "traex", Type: "traex-history"}

	records, warnings, err := candidate.ParseJSONLFile(agent, path, 0, 0)

	if err != nil {
		t.Fatalf("ParseJSONLFile returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d: %#v", len(records), records)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %#v", len(warnings), warnings)
	}
	if warnings[0].Code != "JSONL_PARSE_FAILED" {
		t.Errorf("expected JSONL_PARSE_FAILED warning, got %q", warnings[0].Code)
	}
	if warnings[0].Line != 1 {
		t.Errorf("expected warning line 1, got %d", warnings[0].Line)
	}
}

func writeJSONL(t *testing.T, name string, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test JSONL: %v", err)
	}
	return path
}
