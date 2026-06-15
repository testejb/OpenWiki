package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

var allowedLanguages = []string{"zh", "en"}

var requiredLayoutPaths = []string{
	"raw",
	"wiki",
	filepath.Join("wiki", "pages"),
	filepath.Join("wiki", "indexes"),
	filepath.Join("wiki", "index.md"),
	filepath.Join("wiki", "log.md"),
	"entities",
	"concepts",
}

func Validate(cfg *Config) error {
	if cfg.WikiRoot == "" {
		return &ValidationError{
			Code:    "CONFIG_MISSING_FIELD",
			Message: "缺少必填字段 wiki_root",
			Details: map[string]interface{}{"field": "wiki_root"},
		}
	}

	if _, err := os.Stat(cfg.WikiRoot); os.IsNotExist(err) {
		return &ValidationError{
			Code:    "CONFIG_INVALID_PATH",
			Message: fmt.Sprintf("wiki_root 路径不存在: %s", cfg.WikiRoot),
			Details: map[string]interface{}{"field": "wiki_root", "path": cfg.WikiRoot},
		}
	}

	for _, rel := range requiredLayoutPaths {
		if _, err := os.Stat(filepath.Join(cfg.WikiRoot, rel)); err != nil {
			if os.IsNotExist(err) {
				return &ValidationError{
					Code:    "WIKI_LAYOUT_INVALID",
					Message: fmt.Sprintf("wiki_root 缺少必要路径: %s", rel),
					Details: map[string]interface{}{"field": "wiki_root", "path": cfg.WikiRoot, "missing": rel},
				}
			}
			return &ValidationError{
				Code:    "WIKI_LAYOUT_INVALID",
				Message: fmt.Sprintf("wiki_root 无法访问必要路径: %s", rel),
				Details: map[string]interface{}{"field": "wiki_root", "path": cfg.WikiRoot, "missing": rel, "error": err.Error()},
			}
		}
	}

	if !slices.Contains(allowedLanguages, cfg.Wiki.PrimaryLanguage) {
		return &ValidationError{
			Code:    "CONFIG_INVALID_FIELD",
			Message: fmt.Sprintf("primary_language 值无效: '%s'，支持的值: %v", cfg.Wiki.PrimaryLanguage, allowedLanguages),
			Details: map[string]interface{}{
				"field":   "wiki.primary_language",
				"value":   cfg.Wiki.PrimaryLanguage,
				"allowed": allowedLanguages,
			},
		}
	}

	if !slices.Contains(allowedLanguages, cfg.Wiki.SecondaryLanguage) {
		return &ValidationError{
			Code:    "CONFIG_INVALID_FIELD",
			Message: fmt.Sprintf("secondary_language 值无效: '%s'，支持的值: %v", cfg.Wiki.SecondaryLanguage, allowedLanguages),
			Details: map[string]interface{}{
				"field":   "wiki.secondary_language",
				"value":   cfg.Wiki.SecondaryLanguage,
				"allowed": allowedLanguages,
			},
		}
	}

	return nil
}

type ValidationError struct {
	Code    string
	Message string
	Details map[string]interface{}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}
