package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bytedance/openwiki/internal/config"
)

func createRequiredLayout(t *testing.T, dir string) {
	t.Helper()
	for _, rel := range []string{
		"raw",
		"wiki/pages",
		"wiki/indexes",
		"entities",
		"concepts",
	} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0755); err != nil {
			t.Fatalf("failed to create %s: %v", rel, err)
		}
	}
	for _, rel := range []string{
		"wiki/index.md",
		"wiki/log.md",
	} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("# test\n"), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", rel, err)
		}
	}
}

func requireValidationCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected validation error code %s, got nil", code)
	}
	validationErr, ok := err.(*config.ValidationError)
	if !ok {
		t.Fatalf("expected *config.ValidationError, got %T: %v", err, err)
	}
	if validationErr.Code != code {
		t.Fatalf("expected error code %s, got %s: %v", code, validationErr.Code, err)
	}
}

func TestValidateValidConfig(t *testing.T) {
	dir := t.TempDir()
	createRequiredLayout(t, dir)
	cfg := &config.Config{
		WikiRoot: dir,
		Wiki: config.WikiConfig{
			PrimaryLanguage:   "zh",
			SecondaryLanguage: "en",
		},
	}

	err := config.Validate(cfg)
	if err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
}

func TestValidateMissingWikiRoot(t *testing.T) {
	cfg := &config.Config{
		WikiRoot: "",
		Wiki: config.WikiConfig{
			PrimaryLanguage:   "zh",
			SecondaryLanguage: "en",
		},
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing wiki_root, got nil")
	}
}

func TestValidateInvalidPrimaryLanguage(t *testing.T) {
	dir := t.TempDir()
	createRequiredLayout(t, dir)
	cfg := &config.Config{
		WikiRoot: dir,
		Wiki: config.WikiConfig{
			PrimaryLanguage:   "fr",
			SecondaryLanguage: "en",
		},
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid primary_language, got nil")
	}
}

func TestValidateInvalidSecondaryLanguage(t *testing.T) {
	dir := t.TempDir()
	createRequiredLayout(t, dir)
	cfg := &config.Config{
		WikiRoot: dir,
		Wiki: config.WikiConfig{
			PrimaryLanguage:   "zh",
			SecondaryLanguage: "de",
		},
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid secondary_language, got nil")
	}
}

func TestValidateMissingIndexesDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{
		"raw",
		"wiki/pages",
		"entities",
		"concepts",
	} {
		if err := os.MkdirAll(filepath.Join(dir, rel), 0755); err != nil {
			t.Fatalf("failed to create %s: %v", rel, err)
		}
	}
	for _, rel := range []string{
		"wiki/index.md",
		"wiki/log.md",
	} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("# test\n"), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", rel, err)
		}
	}

	cfg := &config.Config{
		WikiRoot: dir,
		Wiki: config.WikiConfig{
			PrimaryLanguage:   "zh",
			SecondaryLanguage: "en",
		},
	}

	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected error for missing wiki/indexes directory, got nil")
	}
	if !strings.Contains(err.Error(), "wiki/indexes") {
		t.Fatalf("expected error to contain wiki/indexes, got: %v", err)
	}
}

func TestValidateInvalidLanguageBeforeLayoutCompleteness(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		WikiRoot: dir,
		Wiki: config.WikiConfig{
			PrimaryLanguage:   "fr",
			SecondaryLanguage: "en",
		},
	}

	err := config.Validate(cfg)
	requireValidationCode(t, err, "CONFIG_INVALID_FIELD")
	if strings.Contains(err.Error(), "WIKI_LAYOUT_INVALID") {
		t.Fatalf("expected invalid language to be reported before layout errors, got: %v", err)
	}
}

func TestValidateIndexesPathMustBeDirectory(t *testing.T) {
	dir := t.TempDir()
	createRequiredLayout(t, dir)
	indexesPath := filepath.Join(dir, "wiki", "indexes")
	if err := os.Remove(indexesPath); err != nil {
		t.Fatalf("failed to remove indexes dir: %v", err)
	}
	if err := os.WriteFile(indexesPath, []byte("not a directory\n"), 0644); err != nil {
		t.Fatalf("failed to write indexes file: %v", err)
	}

	cfg := &config.Config{
		WikiRoot: dir,
		Wiki: config.WikiConfig{
			PrimaryLanguage:   "zh",
			SecondaryLanguage: "en",
		},
	}

	err := config.Validate(cfg)
	requireValidationCode(t, err, "WIKI_LAYOUT_INVALID")
	if !strings.Contains(err.Error(), "wiki/indexes") || !strings.Contains(err.Error(), "目录") {
		t.Fatalf("expected clear directory type error for wiki/indexes, got: %v", err)
	}
}

func TestValidateIndexPathMustBeFile(t *testing.T) {
	dir := t.TempDir()
	createRequiredLayout(t, dir)
	indexPath := filepath.Join(dir, "wiki", "index.md")
	if err := os.Remove(indexPath); err != nil {
		t.Fatalf("failed to remove index.md: %v", err)
	}
	if err := os.Mkdir(indexPath, 0755); err != nil {
		t.Fatalf("failed to create index.md directory: %v", err)
	}

	cfg := &config.Config{
		WikiRoot: dir,
		Wiki: config.WikiConfig{
			PrimaryLanguage:   "zh",
			SecondaryLanguage: "en",
		},
	}

	err := config.Validate(cfg)
	requireValidationCode(t, err, "WIKI_LAYOUT_INVALID")
	if !strings.Contains(err.Error(), "wiki/index.md") || !strings.Contains(err.Error(), "文件") {
		t.Fatalf("expected clear file type error for wiki/index.md, got: %v", err)
	}
}
