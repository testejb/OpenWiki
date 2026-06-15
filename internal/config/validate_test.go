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
