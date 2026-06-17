package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	WikiRoot  string          `toml:"wiki_root"`
	Wiki      WikiConfig      `toml:"wiki"`
	Remote    RemoteConfig    `toml:"remote"`
	Candidate CandidateConfig `toml:"candidate"`
}

type WikiConfig struct {
	PrimaryLanguage   string            `toml:"primary_language"`
	SecondaryLanguage string            `toml:"secondary_language"`
	SourceTypes       SourceTypesConfig `toml:"source_types"`
	Index             IndexConfig       `toml:"index"`
}

type SourceTypesConfig struct {
	Types []string `toml:"types"`
}

type IndexConfig struct {
	Categories []string `toml:"categories"`
}

type RemoteConfig struct {
	SyncPath string `toml:"sync_path"`
	AutoSync bool   `toml:"auto_sync"`
}

type CandidateConfig struct {
	StateDir    string                   `toml:"state_dir"`
	RunLogPath  string                   `toml:"run_log_path"`
	SnapshotDir string                   `toml:"snapshot_dir"`
	CodeAgent   CandidateCodeAgentConfig `toml:"codeagent"`
}

type CandidateCodeAgentConfig struct {
	Enabled          bool                   `toml:"enabled"`
	StatePath        string                 `toml:"state_path"`
	PendingDir       string                 `toml:"pending_dir"`
	RunLogPath       string                 `toml:"run_log_path"`
	SnapshotDir      string                 `toml:"snapshot_dir"`
	InitialDays      int                    `toml:"initial_days"`
	MaxRecordsPerRun int                    `toml:"max_records_per_run"`
	Agents           []CandidateAgentConfig `toml:"agents"`
	Configured       bool                   `toml:"-"`
	EnabledSet       bool                   `toml:"-"`
}

type CandidateAgentConfig struct {
	Name    string   `toml:"name"`
	Type    string   `toml:"type"`
	Paths   []string `toml:"paths"`
	Enabled bool     `toml:"enabled"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	metadata, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return nil, fmt.Errorf("解析 TOML 配置失败: %w", err)
	}
	cfg.Candidate.CodeAgent.Configured = metadata.IsDefined("candidate", "codeagent")
	cfg.Candidate.CodeAgent.EnabledSet = metadata.IsDefined("candidate", "codeagent", "enabled")
	if cfg.WikiRoot != "" && !filepath.IsAbs(cfg.WikiRoot) {
		cfg.WikiRoot = filepath.Clean(filepath.Join(filepath.Dir(path), cfg.WikiRoot))
	}

	return &cfg, nil
}

func Set(path, key, value string) (oldVal, newVal string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("读取配置文件失败: %w", err)
	}

	var fileCfg Config
	metadata, err := toml.Decode(string(data), &fileCfg)
	if err != nil {
		return "", "", fmt.Errorf("解析 TOML 配置失败: %w", err)
	}

	cfg, err := Load(path)
	if err != nil {
		return "", "", err
	}

	oldVal, err = getFieldValue(cfg, key)
	if err != nil {
		return "", "", err
	}

	if err := setFieldValue(cfg, key, value); err != nil {
		return "", "", err
	}
	if key != "wiki_root" {
		cfg.WikiRoot = fileCfg.WikiRoot
	}

	f, err := os.Create(path)
	if err != nil {
		return "", "", fmt.Errorf("写入配置文件失败: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(newConfigForWrite(cfg, metadata)); err != nil {
		return "", "", fmt.Errorf("编码 TOML 失败: %w", err)
	}

	return oldVal, value, nil
}

type configForWrite struct {
	WikiRoot  string                   `toml:"wiki_root"`
	Wiki      WikiConfig               `toml:"wiki"`
	Remote    RemoteConfig             `toml:"remote"`
	Candidate *candidateConfigForWrite `toml:"candidate,omitempty"`
}

type candidateConfigForWrite struct {
	StateDir    *string                  `toml:"state_dir,omitempty"`
	RunLogPath  *string                  `toml:"run_log_path,omitempty"`
	SnapshotDir *string                  `toml:"snapshot_dir,omitempty"`
	CodeAgent   *codeAgentConfigForWrite `toml:"codeagent,omitempty"`
}

type codeAgentConfigForWrite struct {
	Enabled          *bool                  `toml:"enabled,omitempty"`
	StatePath        *string                `toml:"state_path,omitempty"`
	PendingDir       *string                `toml:"pending_dir,omitempty"`
	RunLogPath       *string                `toml:"run_log_path,omitempty"`
	SnapshotDir      *string                `toml:"snapshot_dir,omitempty"`
	InitialDays      *int                   `toml:"initial_days,omitempty"`
	MaxRecordsPerRun *int                   `toml:"max_records_per_run,omitempty"`
	Agents           []CandidateAgentConfig `toml:"agents,omitempty"`
}

func newConfigForWrite(cfg *Config, metadata toml.MetaData) configForWrite {
	out := configForWrite{
		WikiRoot: cfg.WikiRoot,
		Wiki:     cfg.Wiki,
		Remote:   cfg.Remote,
	}
	if !metadata.IsDefined("candidate") {
		return out
	}

	candidate := &candidateConfigForWrite{}
	if metadata.IsDefined("candidate", "state_dir") {
		candidate.StateDir = &cfg.Candidate.StateDir
	}
	if metadata.IsDefined("candidate", "run_log_path") {
		candidate.RunLogPath = &cfg.Candidate.RunLogPath
	}
	if metadata.IsDefined("candidate", "snapshot_dir") {
		candidate.SnapshotDir = &cfg.Candidate.SnapshotDir
	}
	if metadata.IsDefined("candidate", "codeagent") {
		candidate.CodeAgent = newCodeAgentConfigForWrite(cfg.Candidate.CodeAgent, metadata)
	}
	out.Candidate = candidate

	return out
}

func newCodeAgentConfigForWrite(cfg CandidateCodeAgentConfig, metadata toml.MetaData) *codeAgentConfigForWrite {
	out := &codeAgentConfigForWrite{}
	if metadata.IsDefined("candidate", "codeagent", "enabled") {
		out.Enabled = &cfg.Enabled
	}
	if metadata.IsDefined("candidate", "codeagent", "state_path") {
		out.StatePath = &cfg.StatePath
	}
	if metadata.IsDefined("candidate", "codeagent", "pending_dir") {
		out.PendingDir = &cfg.PendingDir
	}
	if metadata.IsDefined("candidate", "codeagent", "run_log_path") {
		out.RunLogPath = &cfg.RunLogPath
	}
	if metadata.IsDefined("candidate", "codeagent", "snapshot_dir") {
		out.SnapshotDir = &cfg.SnapshotDir
	}
	if metadata.IsDefined("candidate", "codeagent", "initial_days") {
		out.InitialDays = &cfg.InitialDays
	}
	if metadata.IsDefined("candidate", "codeagent", "max_records_per_run") {
		out.MaxRecordsPerRun = &cfg.MaxRecordsPerRun
	}
	if metadata.IsDefined("candidate", "codeagent", "agents") {
		out.Agents = cfg.Agents
	}
	return out
}

func getFieldValue(cfg *Config, key string) (string, error) {
	switch key {
	case "wiki_root":
		return cfg.WikiRoot, nil
	case "wiki.primary_language":
		return cfg.Wiki.PrimaryLanguage, nil
	case "wiki.secondary_language":
		return cfg.Wiki.SecondaryLanguage, nil
	case "remote.sync_path":
		return cfg.Remote.SyncPath, nil
	case "remote.auto_sync":
		if cfg.Remote.AutoSync {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("未知配置项: %s", key)
	}
}

func setFieldValue(cfg *Config, key, value string) error {
	switch key {
	case "wiki_root":
		cfg.WikiRoot = value
	case "wiki.primary_language":
		cfg.Wiki.PrimaryLanguage = value
	case "wiki.secondary_language":
		cfg.Wiki.SecondaryLanguage = value
	case "remote.sync_path":
		cfg.Remote.SyncPath = value
	case "remote.auto_sync":
		cfg.Remote.AutoSync = value == "true"
	default:
		return fmt.Errorf("未知配置项: %s", key)
	}
	return nil
}
