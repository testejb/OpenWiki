package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	WikiRoot string       `toml:"wiki_root"`
	Wiki     WikiConfig   `toml:"wiki"`
	Remote   RemoteConfig `toml:"remote"`
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

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析 TOML 配置失败: %w", err)
	}
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
	if err := toml.Unmarshal(data, &fileCfg); err != nil {
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
	if err := encoder.Encode(cfg); err != nil {
		return "", "", fmt.Errorf("编码 TOML 失败: %w", err)
	}

	return oldVal, value, nil
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
