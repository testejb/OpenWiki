package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/openwiki/internal/config"
)

func TestLoadValidTOML(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "/Users/me/wiki"

[wiki]
primary_language = "zh"
secondary_language = "en"

[wiki.source_types]
types = ["papers", "urls", "code", "docs", "transcripts"]

[wiki.index]
categories = ["资料页", "概念页", "适用范围", "快速导航"]

[remote]
sync_path = "wiki"
auto_sync = false
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test toml: %v", err)
	}

	cfg, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.WikiRoot != "/Users/me/wiki" {
		t.Errorf("expected wiki_root=/Users/me/wiki, got %s", cfg.WikiRoot)
	}
	if cfg.Wiki.PrimaryLanguage != "zh" {
		t.Errorf("expected primary_language=zh, got %s", cfg.Wiki.PrimaryLanguage)
	}
	if cfg.Wiki.SecondaryLanguage != "en" {
		t.Errorf("expected secondary_language=en, got %s", cfg.Wiki.SecondaryLanguage)
	}
	if len(cfg.Wiki.SourceTypes.Types) != 5 {
		t.Errorf("expected 5 source types, got %d", len(cfg.Wiki.SourceTypes.Types))
	}
	if cfg.Remote.SyncPath != "wiki" {
		t.Errorf("expected sync_path=wiki, got %s", cfg.Remote.SyncPath)
	}
	if cfg.Remote.AutoSync != false {
		t.Errorf("expected auto_sync=false, got %v", cfg.Remote.AutoSync)
	}
}

func TestLoadResolvesRelativeWikiRootFromConfigDir(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "./openwiki"`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test toml: %v", err)
	}

	cfg, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(dir, "openwiki")
	if cfg.WikiRoot != expected {
		t.Errorf("expected wiki_root=%s, got %s", expected, cfg.WikiRoot)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path/openwiki.toml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = ["this", "should", "be", "a", "string"]`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test toml: %v", err)
	}

	_, err := config.Load(tomlPath)
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}

func TestSetNestedField(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "/Users/me/wiki"

[wiki]
primary_language = "zh"
secondary_language = "en"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test toml: %v", err)
	}

	oldVal, newVal, err := config.Set(tomlPath, "wiki.primary_language", "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if oldVal != "zh" {
		t.Errorf("expected old value=zh, got %s", oldVal)
	}
	if newVal != "en" {
		t.Errorf("expected new value=en, got %s", newVal)
	}

	cfg, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if cfg.Wiki.PrimaryLanguage != "en" {
		t.Errorf("expected primary_language=en after set, got %s", cfg.Wiki.PrimaryLanguage)
	}
}

func TestSetNestedFieldPreservesRelativeWikiRoot(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "./openwiki"

[wiki]
primary_language = "zh"
secondary_language = "en"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test toml: %v", err)
	}

	_, _, err := config.Set(tomlPath, "wiki.primary_language", "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatalf("failed to read test toml: %v", err)
	}
	output := string(data)
	if !strings.Contains(output, `wiki_root = "./openwiki"`) {
		t.Errorf("expected file to preserve relative wiki_root, got:\n%s", output)
	}
	if !strings.Contains(output, `primary_language = "en"`) {
		t.Errorf("expected file to update primary_language, got:\n%s", output)
	}
}

func TestSetTopLevelField(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "/Users/me/wiki"

[wiki]
primary_language = "zh"
secondary_language = "en"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test toml: %v", err)
	}

	oldVal, newVal, err := config.Set(tomlPath, "wiki_root", "/new/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if oldVal != "/Users/me/wiki" {
		t.Errorf("expected old value=/Users/me/wiki, got %s", oldVal)
	}
	if newVal != "/new/path" {
		t.Errorf("expected new value=/new/path, got %s", newVal)
	}

	cfg, err := config.Load(tomlPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if cfg.WikiRoot != "/new/path" {
		t.Errorf("expected wiki_root=/new/path after set, got %s", cfg.WikiRoot)
	}
}

func TestSetUnknownField(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "/Users/me/wiki"

[wiki]
primary_language = "zh"
secondary_language = "en"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test toml: %v", err)
	}

	_, _, err := config.Set(tomlPath, "nonexistent.field", "value")
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestLoadCandidateConfig(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "openwiki.toml")
	content := `wiki_root = "/Users/me/wiki"

[candidate]
state_dir = "candidate-state"

[candidate.codeagent]
enabled = true
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
		t.Errorf("expected candidate state_dir=candidate-state, got %s", cfg.Candidate.StateDir)
	}
	if !cfg.Candidate.CodeAgent.Enabled {
		t.Errorf("expected candidate codeagent enabled=true")
	}
	if cfg.Candidate.CodeAgent.InitialDays != 7 {
		t.Errorf("expected initial_days=7, got %d", cfg.Candidate.CodeAgent.InitialDays)
	}
	if cfg.Candidate.CodeAgent.MaxRecordsPerRun != 200 {
		t.Errorf("expected max_records_per_run=200, got %d", cfg.Candidate.CodeAgent.MaxRecordsPerRun)
	}
	if len(cfg.Candidate.CodeAgent.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(cfg.Candidate.CodeAgent.Agents))
	}
	agent := cfg.Candidate.CodeAgent.Agents[0]
	if agent.Name != "traex-work" {
		t.Errorf("expected agent name=traex-work, got %s", agent.Name)
	}
	if agent.Type != "traex-history" {
		t.Errorf("expected agent type=traex-history, got %s", agent.Type)
	}
	if len(agent.Paths) != 1 || agent.Paths[0] != "/tmp/history.jsonl" {
		t.Errorf("expected agent paths=[/tmp/history.jsonl], got %#v", agent.Paths)
	}
	if !agent.Enabled {
		t.Errorf("expected agent enabled=true")
	}
}
