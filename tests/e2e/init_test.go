package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/openwiki/tests/e2e/harness"
)

func TestE2EInit(t *testing.T) {
	h := harness.New(t)

	wikiRoot := h.TempWikiRoot()

	result, err := h.Run("init", wikiRoot, "--non-interactive", "--json")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &data); err != nil {
		t.Fatalf("parse JSON: %v\nstdout: %s", err, result.Stdout)
	}

	if data["success"] != true {
		t.Fatalf("expected success=true, got %v", data)
	}

	paths := []string{
		filepath.Join(wikiRoot, "openwiki.toml"),
		filepath.Join(wikiRoot, "wiki", "index.md"),
		filepath.Join(wikiRoot, "wiki", "log.md"),
		filepath.Join(wikiRoot, "wiki", "pages"),
		filepath.Join(wikiRoot, "raw"),
		filepath.Join(wikiRoot, "concepts"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected path to exist: %s", p)
		}
	}
}

func TestInitCreatesLayeredIndexAndIndexCommandsPass(t *testing.T) {
	h := harness.New(t)

	wikiRoot := h.TempWikiRoot()

	initResult, err := h.Run("init", wikiRoot, "--non-interactive", "--json")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if initResult.ExitCode != 0 {
		t.Fatalf("init failed: stdout=%s stderr=%s", initResult.Stdout, initResult.Stderr)
	}

	var initData map[string]interface{}
	if err := json.Unmarshal([]byte(initResult.Stdout), &initData); err != nil {
		t.Fatalf("parse init JSON: %v\nstdout: %s", err, initResult.Stdout)
	}
	if initData["success"] != true {
		t.Fatalf("init failed: %v", initData)
	}

	harness.AssertLayeredIndexLayout(t, wikiRoot)

	configPath := filepath.Join(wikiRoot, "openwiki.toml")

	checkResult, err := h.Run("--config", configPath, "index", "check", "--json")
	if err != nil {
		t.Fatalf("index check: %v", err)
	}
	if checkResult.ExitCode != 0 {
		t.Fatalf("index check failed: stdout=%s stderr=%s", checkResult.Stdout, checkResult.Stderr)
	}
	var checkData map[string]interface{}
	if err := json.Unmarshal([]byte(checkResult.Stdout), &checkData); err != nil {
		t.Fatalf("parse index check JSON: %v\nstdout: %s", err, checkResult.Stdout)
	}
	if checkData["success"] != true {
		t.Fatalf("expected index check success=true, got %v", checkData)
	}

	rebuildResult, err := h.Run("--config", configPath, "index", "rebuild", "--json")
	if err != nil {
		t.Fatalf("index rebuild: %v", err)
	}
	if rebuildResult.ExitCode != 0 {
		t.Fatalf("index rebuild failed: stdout=%s stderr=%s", rebuildResult.Stdout, rebuildResult.Stderr)
	}
	var rebuildData map[string]interface{}
	if err := json.Unmarshal([]byte(rebuildResult.Stdout), &rebuildData); err != nil {
		t.Fatalf("parse index rebuild JSON: %v\nstdout: %s", err, rebuildResult.Stdout)
	}
	if rebuildData["success"] != true {
		t.Fatalf("expected index rebuild success=true, got %v", rebuildData)
	}
}

func TestE2EPageLifecycle(t *testing.T) {
	h := harness.New(t)

	wikiRoot := h.TempWikiRoot()

	initResult, err := h.Run("init", wikiRoot, "--non-interactive", "--json")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	var initData map[string]interface{}
	if err := json.Unmarshal([]byte(initResult.Stdout), &initData); err != nil {
		t.Fatalf("parse init JSON: %v", initResult.Stdout)
	}
	if initData["success"] != true {
		t.Fatalf("init failed: %v", initData)
	}

	configPath := filepath.Join(wikiRoot, "openwiki.toml")

	pageContent := `---
title: 测试页面
tags: [test, e2e]
scope_level: repo
scope_code: test-repo
updated: 2026-06-02
---

这是测试页面的内容

## 参考

- [[other-page]]
`
	contentFile := filepath.Join(h.TempWikiRoot(), "test-content.md")
	if err := os.WriteFile(contentFile, []byte(pageContent), 0644); err != nil {
		t.Fatalf("write content file: %v", err)
	}

	createResult, err := h.Run("--config", configPath, "page", "create", "--file", contentFile, "test-page", "--json")
	if err != nil {
		t.Fatalf("page create: %v", err)
	}

	var createData map[string]interface{}
	if err := json.Unmarshal([]byte(createResult.Stdout), &createData); err != nil {
		t.Fatalf("parse create JSON: %v\nstdout: %s", err, createResult.Stdout)
	}
	if createData["success"] != true {
		t.Fatalf("page create failed: %v", createData)
	}

	getResult, err := h.Run("--config", configPath, "page", "get", "test-page", "--json")
	if err != nil {
		t.Fatalf("page get: %v", err)
	}

	var getData map[string]interface{}
	if err := json.Unmarshal([]byte(getResult.Stdout), &getData); err != nil {
		t.Fatalf("parse get JSON: %v\nstdout: %s", err, getResult.Stdout)
	}
	if getData["success"] != true {
		t.Fatalf("page get failed: %v", getData)
	}

	updatedContent := `---
title: 更新后的测试页面
tags: [test, e2e, updated]
scope_level: repo
scope_code: test-repo
updated: 2026-06-03
---

这是更新后的内容
`
	updateFile := filepath.Join(h.TempWikiRoot(), "update-content.md")
	if err := os.WriteFile(updateFile, []byte(updatedContent), 0644); err != nil {
		t.Fatalf("write update file: %v", err)
	}

	updateResult, err := h.Run("--config", configPath, "page", "update", "--file", updateFile, "test-page", "--json")
	if err != nil {
		t.Fatalf("page update: %v", err)
	}

	var updateData map[string]interface{}
	if err := json.Unmarshal([]byte(updateResult.Stdout), &updateData); err != nil {
		t.Fatalf("parse update JSON: %v\nstdout: %s", err, updateResult.Stdout)
	}
	if updateData["success"] != true {
		t.Fatalf("page update failed: %v", updateData)
	}

	deleteResult, err := h.Run("--config", configPath, "page", "delete", "test-page", "--json")
	if err != nil {
		t.Fatalf("page delete: %v", err)
	}

	var deleteData map[string]interface{}
	if err := json.Unmarshal([]byte(deleteResult.Stdout), &deleteData); err != nil {
		t.Fatalf("parse delete JSON: %v\nstdout: %s", err, deleteResult.Stdout)
	}
	if deleteData["success"] != true {
		t.Fatalf("page delete failed: %v", deleteData)
	}

	pagePath := filepath.Join(wikiRoot, "wiki", "pages", "test-page.md")
	if _, err := os.Stat(pagePath); !os.IsNotExist(err) {
		t.Error("expected page file to be deleted")
	}
}

func TestE2EStatus(t *testing.T) {
	h := harness.New(t)

	wikiRoot := h.TempWikiRoot()

	initResult, err := h.Run("init", wikiRoot, "--non-interactive", "--json")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	var initData map[string]interface{}
	if err := json.Unmarshal([]byte(initResult.Stdout), &initData); err != nil {
		t.Fatalf("parse init JSON: %v", initResult.Stdout)
	}
	if initData["success"] != true {
		t.Fatalf("init failed: %v", initData)
	}

	configPath := filepath.Join(wikiRoot, "openwiki.toml")

	statusResult, err := h.Run("--config", configPath, "status", "--json")
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	var statusData map[string]interface{}
	if err := json.Unmarshal([]byte(statusResult.Stdout), &statusData); err != nil {
		t.Fatalf("parse status JSON: %v\nstdout: %s", err, statusResult.Stdout)
	}
	if statusData["success"] != true {
		t.Fatalf("status failed: %v", statusData)
	}

	data, ok := statusData["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}

	pages, ok := data["pages"].(map[string]interface{})
	if !ok {
		t.Fatal("expected pages to be a map")
	}

	total, ok := pages["total"].(float64)
	if !ok {
		t.Fatal("expected total to be a number")
	}
	if total != 0 {
		t.Errorf("expected total=0, got %v", total)
	}
}
