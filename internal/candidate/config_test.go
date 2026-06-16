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
		t.Errorf("expected enabled=true")
	}
	if resolved.StatePath != filepath.Join(wikiRoot, "candidate", "codeagent", "state.json") {
		t.Errorf("expected default state path under wiki root, got %s", resolved.StatePath)
	}
	if resolved.PendingDir != filepath.Join(wikiRoot, "candidate", "codeagent", "pending") {
		t.Errorf("expected default pending dir under wiki root, got %s", resolved.PendingDir)
	}
	if resolved.RunLogPath != filepath.Join(wikiRoot, "candidate", "codeagent", "run.log") {
		t.Errorf("expected default run log path under wiki root, got %s", resolved.RunLogPath)
	}
	if resolved.SnapshotDir != filepath.Join(wikiRoot, "candidate", "codeagent", "reviews") {
		t.Errorf("expected default snapshot dir under wiki root, got %s", resolved.SnapshotDir)
	}
	if resolved.InitialDays != 14 {
		t.Errorf("expected initial days=14, got %d", resolved.InitialDays)
	}
	if resolved.MaxRecordsPerRun != 500 {
		t.Errorf("expected max records per run=500, got %d", resolved.MaxRecordsPerRun)
	}
	if len(resolved.Agents) != 2 {
		t.Fatalf("expected 2 default agents, got %d", len(resolved.Agents))
	}
	assertAgent(t, resolved.Agents[0], "traex", "traex-history", []string{"/Users/bytedance/.trae/cli/history.jsonl"}, true)
	assertAgent(t, resolved.Agents[1], "trae-ide", "trae-ide-memory", []string{"/Users/bytedance/.trae-cn/memory/projects/**/session_memory_*.jsonl"}, true)
}

func TestResolveCodeAgentConfigOverrides(t *testing.T) {
	wikiRoot := t.TempDir()
	cfg := &config.Config{
		WikiRoot: wikiRoot,
		Candidate: config.CandidateConfig{
			StateDir: "candidate-state",
			CodeAgent: config.CandidateCodeAgentConfig{
				Enabled:          true,
				StatePath:        "custom/state.json",
				PendingDir:       "custom/pending",
				RunLogPath:       "custom/run.log",
				SnapshotDir:      "custom/reviews",
				InitialDays:      3,
				MaxRecordsPerRun: 10,
				Agents: []config.CandidateAgentConfig{
					{
						Name:    "custom",
						Type:    "custom-history",
						Paths:   []string{"relative/history.jsonl", "/absolute/history.jsonl"},
						Enabled: true,
					},
					{
						Name:    "disabled",
						Type:    "disabled-history",
						Paths:   []string{"disabled.jsonl"},
						Enabled: false,
					},
				},
			},
		},
	}

	resolved := candidate.ResolveCodeAgentConfig(cfg)

	if !resolved.Enabled {
		t.Errorf("expected enabled=true")
	}
	if resolved.StatePath != filepath.Join(wikiRoot, "custom", "state.json") {
		t.Errorf("expected relative state path resolved against wiki root, got %s", resolved.StatePath)
	}
	if resolved.PendingDir != filepath.Join(wikiRoot, "custom", "pending") {
		t.Errorf("expected relative pending dir resolved against wiki root, got %s", resolved.PendingDir)
	}
	if resolved.RunLogPath != filepath.Join(wikiRoot, "custom", "run.log") {
		t.Errorf("expected relative run log path resolved against wiki root, got %s", resolved.RunLogPath)
	}
	if resolved.SnapshotDir != filepath.Join(wikiRoot, "custom", "reviews") {
		t.Errorf("expected relative snapshot dir resolved against wiki root, got %s", resolved.SnapshotDir)
	}
	if resolved.InitialDays != 3 {
		t.Errorf("expected initial days=3, got %d", resolved.InitialDays)
	}
	if resolved.MaxRecordsPerRun != 10 {
		t.Errorf("expected max records per run=10, got %d", resolved.MaxRecordsPerRun)
	}
	if len(resolved.Agents) != 1 {
		t.Fatalf("expected only enabled custom agent, got %d", len(resolved.Agents))
	}
	assertAgent(t, resolved.Agents[0], "custom", "custom-history", []string{filepath.Join(wikiRoot, "relative", "history.jsonl"), "/absolute/history.jsonl"}, true)
}

func assertAgent(t *testing.T, agent candidate.AgentConfig, name, typ string, paths []string, enabled bool) {
	t.Helper()
	if agent.Name != name {
		t.Errorf("expected agent name=%s, got %s", name, agent.Name)
	}
	if agent.Type != typ {
		t.Errorf("expected agent type=%s, got %s", typ, agent.Type)
	}
	if agent.Enabled != enabled {
		t.Errorf("expected agent enabled=%v, got %v", enabled, agent.Enabled)
	}
	if len(agent.Paths) != len(paths) {
		t.Fatalf("expected %d paths, got %d: %#v", len(paths), len(agent.Paths), agent.Paths)
	}
	for i := range paths {
		if agent.Paths[i] != paths[i] {
			t.Errorf("expected path[%d]=%s, got %s", i, paths[i], agent.Paths[i])
		}
	}
}
