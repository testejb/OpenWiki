package candidate

import (
	"path/filepath"

	"github.com/bytedance/openwiki/internal/config"
)

const (
	defaultInitialDays      = 14
	defaultMaxRecordsPerRun = 500
)

func ResolveCodeAgentConfig(cfg *config.Config) CodeAgentConfig {
	wikiRoot := cfg.WikiRoot
	candidateRoot := resolvePath(wikiRoot, cfg.Candidate.StateDir)
	if candidateRoot == "" {
		candidateRoot = filepath.Join(wikiRoot, "candidate")
	}
	codeAgentRoot := filepath.Join(candidateRoot, "codeagent")
	codeAgentCfg := cfg.Candidate.CodeAgent

	resolved := CodeAgentConfig{
		Enabled:          true,
		StatePath:        resolvePath(wikiRoot, codeAgentCfg.StatePath),
		PendingDir:       resolvePath(wikiRoot, codeAgentCfg.PendingDir),
		RunLogPath:       resolvePath(wikiRoot, codeAgentCfg.RunLogPath),
		SnapshotDir:      resolvePath(wikiRoot, codeAgentCfg.SnapshotDir),
		InitialDays:      codeAgentCfg.InitialDays,
		MaxRecordsPerRun: codeAgentCfg.MaxRecordsPerRun,
	}

	if resolved.StatePath == "" {
		resolved.StatePath = filepath.Join(codeAgentRoot, "state.json")
	}
	if resolved.PendingDir == "" {
		resolved.PendingDir = filepath.Join(codeAgentRoot, "pending")
	}
	if resolved.RunLogPath == "" {
		resolved.RunLogPath = filepath.Join(codeAgentRoot, "run.log")
	}
	if resolved.SnapshotDir == "" {
		resolved.SnapshotDir = filepath.Join(codeAgentRoot, "reviews")
	}
	if resolved.InitialDays <= 0 {
		resolved.InitialDays = defaultInitialDays
	}
	if resolved.MaxRecordsPerRun <= 0 {
		resolved.MaxRecordsPerRun = defaultMaxRecordsPerRun
	}
	if codeAgentCfg.Configured && codeAgentCfg.EnabledSet && !codeAgentCfg.Enabled {
		resolved.Enabled = false
	}

	if len(codeAgentCfg.Agents) == 0 {
		resolved.Agents = defaultAgents()
		return resolved
	}

	for _, agent := range codeAgentCfg.Agents {
		if !agent.Enabled {
			continue
		}
		resolved.Agents = append(resolved.Agents, AgentConfig{
			Name:    agent.Name,
			Type:    agent.Type,
			Paths:   resolvePaths(wikiRoot, agent.Paths),
			Enabled: agent.Enabled,
		})
	}

	return resolved
}

func resolvePath(wikiRoot, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(wikiRoot, path))
}

func resolvePaths(wikiRoot string, paths []string) []string {
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		resolved = append(resolved, resolvePath(wikiRoot, path))
	}
	return resolved
}

func defaultAgents() []AgentConfig {
	return []AgentConfig{
		{
			Name:    "traex",
			Type:    "traex-history",
			Paths:   []string{"/Users/bytedance/.trae/cli/history.jsonl"},
			Enabled: true,
		},
		{
			Name:    "trae-ide",
			Type:    "trae-ide-memory",
			Paths:   []string{"/Users/bytedance/.trae-cn/memory/projects/**/session_memory_*.jsonl"},
			Enabled: true,
		},
	}
}
