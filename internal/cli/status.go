package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/bytedance/openwiki/internal/output"
	"github.com/bytedance/openwiki/internal/wiki"
)

type StatusResult struct {
	Pages   PageStats    `json:"pages"`
	Config  ConfigInfo   `json:"config"`
	Index   IndexStatus  `json:"index"`
	Details []PageDetail `json:"details,omitempty"`
}

type PageStats struct {
	Total    int            `json:"total"`
	ByScope  map[string]int `json:"by_scope"`
	Orphaned []string       `json:"orphaned"`
	Stale    []string       `json:"stale"`
}

type ConfigInfo struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}

type IndexStatus struct {
	Health         string   `json:"health"`
	MissingFiles   []string `json:"missing_files,omitempty"`
	UnindexedPages []string `json:"unindexed_pages,omitempty"`
}

type PageDetail struct {
	Slug       string   `json:"slug"`
	Title      string   `json:"title"`
	Tags       []string `json:"tags"`
	ScopeLevel string   `json:"scope_level"`
	ScopeCode  string   `json:"scope_code"`
	Updated    string   `json:"updated"`
}

func runStatus(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	cfg, result, err := discoverConfig(opts)
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{
				Code:    "CONFIG_NOT_FOUND",
				Message: err.Error(),
			})
		}
		return err
	}

	fs := wiki.NewOsFS()

	pages, err := wiki.ListPages(fs, cfg.WikiRoot)
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{
				Code:    "IO_ERROR",
				Message: err.Error(),
			})
		}
		return err
	}

	stats := PageStats{
		Total:   len(pages),
		ByScope: make(map[string]int),
	}

	allSlugs := make(map[string]bool)
	for _, p := range pages {
		allSlugs[p.Slug] = true
		scopeKey := p.ScopeLevel
		if p.ScopeCode != "" {
			scopeKey = p.ScopeLevel + "/" + p.ScopeCode
		}
		stats.ByScope[scopeKey]++
	}

	var details []PageDetail
	if opts.Verbose {
		for _, p := range pages {
			details = append(details, PageDetail{
				Slug:       p.Slug,
				Title:      p.Title,
				Tags:       p.Tags,
				ScopeLevel: p.ScopeLevel,
				ScopeCode:  p.ScopeCode,
				Updated:    p.Updated,
			})
		}
	}

	now := time.Now()
	for _, p := range pages {
		if p.Updated != "" {
			updatedTime, parseErr := time.Parse("2006-01-02", p.Updated)
			if parseErr == nil && now.Sub(updatedTime) > 90*24*time.Hour {
				stats.Stale = append(stats.Stale, p.Slug)
			}
		}
	}

	indexStatus := IndexStatus{Health: "unknown"}
	if check, err := wiki.CheckIndex(fs, cfg.WikiRoot); err == nil {
		indexStatus = IndexStatus{
			Health:         check.Health,
			MissingFiles:   check.MissingFiles,
			UnindexedPages: check.UnindexedPages,
		}
	}

	statusResult := StatusResult{
		Pages: stats,
		Config: ConfigInfo{
			Source: result.Source,
			Path:   result.Path,
		},
		Index:   indexStatus,
		Details: details,
	}

	if opts.JSON {
		return output.JSON(stdout, true, statusResult, nil)
	}

	fmt.Fprintf(stdout, "配置来源: %s (%s)\n", result.Source, result.Path)
	fmt.Fprintf(stdout, "页面总数: %d\n", stats.Total)
	fmt.Fprintf(stdout, "索引健康状态: %s\n", indexStatus.Health)
	if len(indexStatus.MissingFiles) > 0 {
		fmt.Fprintf(stdout, "缺失索引文件: %v\n", indexStatus.MissingFiles)
	}
	if len(indexStatus.UnindexedPages) > 0 {
		fmt.Fprintf(stdout, "未索引页面: %v\n", indexStatus.UnindexedPages)
	}
	fmt.Fprintf(stdout, "按范围统计:\n")
	for scope, count := range stats.ByScope {
		fmt.Fprintf(stdout, "  %s: %d\n", scope, count)
	}
	if len(stats.Orphaned) > 0 {
		fmt.Fprintf(stdout, "孤立页面: %v\n", stats.Orphaned)
	}
	if len(stats.Stale) > 0 {
		fmt.Fprintf(stdout, "过期页面: %v\n", stats.Stale)
	}
	if opts.Verbose {
		fmt.Fprintf(stdout, "\n页面详情:\n")
		for _, d := range details {
			fmt.Fprintf(stdout, "  %s: %s [%s] %s\n", d.Slug, d.Title, d.ScopeLevel+"/"+d.ScopeCode, d.Updated)
		}
	}

	_ = cfg
	return nil
}
