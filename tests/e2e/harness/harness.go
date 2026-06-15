package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type Harness struct {
	BinPath   string
	WikiRoot  string
	ConfigDir string
	workDir   string
}

func New(t *testing.T) *Harness {
	t.Helper()

	workDir := t.TempDir()
	binPath := filepath.Join(workDir, "openwiki")

	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repo root: %v", err)
	}

	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/openwiki/")
	buildCmd.Dir = repoRoot
	buildCmd.Env = append(os.Environ(), "GOFLAGS=")
	out, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build binary: %v\n%s", err, string(out))
	}

	wikiRoot := filepath.Join(workDir, "wiki-data")
	configDir := filepath.Join(workDir, "config")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}

	return &Harness{
		BinPath:   binPath,
		WikiRoot:  wikiRoot,
		ConfigDir: configDir,
		workDir:   workDir,
	}
}

func findRepoRoot() (string, error) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func (h *Harness) Run(args ...string) (*RunResult, error) {
	cmd := exec.Command(h.BinPath, args...)
	cmd.Dir = h.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &RunResult{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		return result, err
	}

	return result, nil
}

func (h *Harness) RunJSON(args ...string) (map[string]interface{}, error) {
	result, err := h.Run(args...)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &data); err != nil {
		return nil, fmt.Errorf("parse JSON: %w\nstdout: %s", err, result.Stdout)
	}

	return data, nil
}

func (h *Harness) ConfigPath() string {
	return filepath.Join(h.ConfigDir, "openwiki.toml")
}

func (h *Harness) WriteConfig(content string) error {
	return os.WriteFile(h.ConfigPath(), []byte(content), 0644)
}

func (h *Harness) TempWikiRoot() string {
	return h.WikiRoot
}

func AssertLayeredIndexLayout(t testing.TB, wikiRoot string) {
	t.Helper()

	expected := []string{
		filepath.Join(wikiRoot, "wiki", "index.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "scopes.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "entities.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "concepts.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "tags.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "recent.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "hot.md"),
		filepath.Join(wikiRoot, "wiki", "indexes", "query-usage.jsonl"),
	}
	for _, path := range expected {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected layered index path %s: %v", path, err)
		}
	}
}

func (h *Harness) Cleanup() {
	os.RemoveAll(h.workDir)
}

func (h *Harness) WaitForFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for file: %s", path)
}
