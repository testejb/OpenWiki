package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/openwiki/internal/cli"
	"github.com/bytedance/openwiki/internal/output"
)

func TestCandidateCodeAgentScanJSON(t *testing.T) {
	dir := t.TempDir()
	tomlPath, _ := setupCandidateCLIConfig(t, dir)

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--config", tomlPath, "candidate", "codeagent", "scan", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeCLIResponse(t, stdout.Bytes())
	if !resp.Success {
		t.Fatalf("expected success=true, got error: %#v", resp.Error)
	}
	data := responseDataMap(t, resp)
	if data["pending_path"] == "" {
		t.Fatalf("expected pending_path to be non-empty, got %#v", data["pending_path"])
	}
	if data["state_path"] == "" {
		t.Fatalf("expected state_path to be non-empty, got %#v", data["state_path"])
	}
	records := data["records"].(map[string]interface{})
	if records["total"] != float64(1) {
		t.Fatalf("expected records.total=1, got %#v", records["total"])
	}
}

func TestCandidateCodeAgentScanJSONConfigNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--config", "/nonexistent/openwiki.toml", "candidate", "codeagent", "scan", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected JSON error envelope without Go error, got: %v", err)
	}

	assertCandidateConfigNotFound(t, stdout.Bytes())
}

func TestCandidateCodeAgentCommitJSON(t *testing.T) {
	dir := t.TempDir()
	tomlPath, wikiRoot := setupCandidateCLIConfig(t, dir)

	pendingPath := scanCandidateForPending(t, tomlPath)
	snapshotPath := filepath.Join(wikiRoot, "candidate", "codeagent", "reviews", "snapshot.md")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0755); err != nil {
		t.Fatalf("mkdir snapshot dir failed: %v", err)
	}
	if err := os.WriteFile(snapshotPath, []byte("# review\n"), 0644); err != nil {
		t.Fatalf("write snapshot failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{
		"--config", tomlPath,
		"candidate", "codeagent", "commit",
		"--pending", pendingPath,
		"--review-doc-url", "https://example.com/review",
		"--snapshot", snapshotPath,
		"--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeCLIResponse(t, stdout.Bytes())
	if !resp.Success {
		t.Fatalf("expected success=true, got error: %#v", resp.Error)
	}
	data := responseDataMap(t, resp)
	statePath, ok := data["state_path"].(string)
	if !ok || statePath == "" {
		t.Fatalf("expected state_path string, got %#v", data["state_path"])
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected state file to exist at %s: %v", statePath, err)
	}
}

func TestCandidateCodeAgentStatusJSON(t *testing.T) {
	dir := t.TempDir()
	tomlPath, _ := setupCandidateCLIConfig(t, dir)

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--config", tomlPath, "candidate", "codeagent", "status", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeCLIResponse(t, stdout.Bytes())
	if !resp.Success {
		t.Fatalf("expected success=true, got error: %#v", resp.Error)
	}
	data := responseDataMap(t, resp)
	if _, ok := data["tracked_files"]; !ok {
		t.Fatalf("expected tracked_files in status data, got %#v", data)
	}
}

func TestCandidateCodeAgentStatusJSONConfigNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--config", "/nonexistent/openwiki.toml", "candidate", "codeagent", "status", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected JSON error envelope without Go error, got: %v", err)
	}

	assertCandidateConfigNotFound(t, stdout.Bytes())
}

func TestCandidateRootHelpIncludesCandidate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--help"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "candidate") {
		t.Fatalf("expected root help to contain candidate, got:\n%s", stdout.String())
	}
}

func TestCandidateCodeAgentCommitJSONRequiresPending(t *testing.T) {
	dir := t.TempDir()
	tomlPath, wikiRoot := setupCandidateCLIConfig(t, dir)
	snapshotPath := filepath.Join(wikiRoot, "candidate", "codeagent", "reviews", "snapshot.md")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0755); err != nil {
		t.Fatalf("mkdir snapshot dir failed: %v", err)
	}
	if err := os.WriteFile(snapshotPath, []byte("# review\n"), 0644); err != nil {
		t.Fatalf("write snapshot failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{
		"--config", tomlPath,
		"candidate", "codeagent", "commit",
		"--review-doc-url", "https://example.com/review",
		"--snapshot", snapshotPath,
		"--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected JSON error envelope without Go error, got: %v", err)
	}

	resp := decodeCLIResponse(t, stdout.Bytes())
	if resp.Success {
		t.Fatal("expected success=false for missing pending")
	}
	if resp.Error == nil {
		t.Fatal("expected JSON error")
	}
	if resp.Error.Code != "CANDIDATE_COMMIT_FAILED" {
		t.Fatalf("expected CANDIDATE_COMMIT_FAILED, got %q", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "--pending") {
		t.Fatalf("expected error message to mention --pending, got %q", resp.Error.Message)
	}
}

func TestCandidateCodeAgentCommitJSONUsesPendingWithoutConfig(t *testing.T) {
	dir := t.TempDir()
	tomlPath, wikiRoot := setupCandidateCLIConfig(t, dir)

	pendingPath := scanCandidateForPending(t, tomlPath)
	snapshotPath := filepath.Join(wikiRoot, "candidate", "codeagent", "reviews", "snapshot.md")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0755); err != nil {
		t.Fatalf("mkdir snapshot dir failed: %v", err)
	}
	if err := os.WriteFile(snapshotPath, []byte("# review\n"), 0644); err != nil {
		t.Fatalf("write snapshot failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{
		"--config", filepath.Join(dir, "missing-openwiki.toml"),
		"candidate", "codeagent", "commit",
		"--pending", pendingPath,
		"--review-doc-url", "https://example.com/review",
		"--snapshot", snapshotPath,
		"--json",
	}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeCLIResponse(t, stdout.Bytes())
	if !resp.Success {
		t.Fatalf("expected commit to use pending without config discovery, got error: %#v", resp.Error)
	}
}

func setupCandidateCLIConfig(t *testing.T, dir string) (tomlPath, wikiRoot string) {
	t.Helper()

	wikiRoot = filepath.Join(dir, "wiki")
	historyPath := filepath.Join(dir, "history.jsonl")
	if err := os.MkdirAll(wikiRoot, 0755); err != nil {
		t.Fatalf("mkdir wiki root failed: %v", err)
	}
	ts := time.Now().Add(-time.Hour).Unix()
	if err := os.WriteFile(historyPath, []byte(`{"session_id":"s1","ts":`+fmt.Sprint(ts)+`,"text":"候选记录"}`+"\n"), 0644); err != nil {
		t.Fatalf("write history failed: %v", err)
	}

	tomlPath = filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "` + filepath.ToSlash(wikiRoot) + `"

[wiki]
primary_language = "zh"
secondary_language = "en"

[candidate]
state_dir = "candidate"

[candidate.codeagent]
initial_days = 30
max_records_per_run = 10

[[candidate.codeagent.agents]]
name = "traex"
type = "traex-history"
paths = ["` + filepath.ToSlash(historyPath) + `"]
enabled = true
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	return tomlPath, wikiRoot
}

func scanCandidateForPending(t *testing.T, tomlPath string) string {
	t.Helper()

	var stdout, stderr bytes.Buffer
	err := cli.RunWithIO([]string{"--config", tomlPath, "candidate", "codeagent", "scan", "--json"}, "1.0.0", "2026-06-01T00:00:00Z", &stdout, &stderr)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	resp := decodeCLIResponse(t, stdout.Bytes())
	if !resp.Success {
		t.Fatalf("scan success=false: %#v", resp.Error)
	}
	data := responseDataMap(t, resp)
	pendingPath, ok := data["pending_path"].(string)
	if !ok || pendingPath == "" {
		t.Fatalf("expected pending_path string, got %#v", data["pending_path"])
	}
	return pendingPath
}

func decodeCLIResponse(t *testing.T, data []byte) output.Response {
	t.Helper()

	var resp output.Response
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal response %q: %v", string(data), err)
	}
	return resp
}

func responseDataMap(t *testing.T, resp output.Response) map[string]interface{} {
	t.Helper()

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data to be map, got %#v", resp.Data)
	}
	return data
}

func assertCandidateConfigNotFound(t *testing.T, data []byte) {
	t.Helper()

	resp := decodeCLIResponse(t, data)
	if resp.Success {
		t.Fatal("expected success=false for missing config")
	}
	if resp.Error == nil {
		t.Fatal("expected JSON error")
	}
	if resp.Error.Code != "CONFIG_NOT_FOUND" {
		t.Fatalf("expected CONFIG_NOT_FOUND, got %q", resp.Error.Code)
	}
}
