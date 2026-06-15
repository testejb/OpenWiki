package cli

import (
	"fmt"
	"io"

	"github.com/bytedance/openwiki/internal/output"
	"github.com/bytedance/openwiki/internal/wiki"
)

func runIndex(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("index 需要子命令: check, rebuild")
	}

	subcommand := args[0]

	switch subcommand {
	case "check":
		return runIndexCheck(stdout, stderr, opts)
	case "rebuild":
		return runIndexRebuild(stdout, stderr, opts)
	default:
		return fmt.Errorf("未知 index 子命令: %s", subcommand)
	}
}

func runIndexCheck(stdout, stderr io.Writer, opts *GlobalOptions) error {
	cfg, _, err := discoverConfig(opts)
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{
				Code:    "CONFIG_NOT_FOUND",
				Message: err.Error(),
			})
		}
		return err
	}

	result, err := wiki.CheckIndex(wiki.NewOsFS(), cfg.WikiRoot)
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{
				Code:    "INDEX_CHECK_FAILED",
				Message: err.Error(),
			})
		}
		return err
	}

	if opts.JSON {
		return output.JSON(stdout, true, result, nil)
	}

	fmt.Fprintf(stdout, "索引健康状态: %s\n", result.Health)
	fmt.Fprintf(stdout, "页面总数: %d\n", result.PageCount)
	if len(result.MissingFiles) > 0 {
		fmt.Fprintf(stdout, "缺失索引文件: %v\n", result.MissingFiles)
	}
	if len(result.UnindexedPages) > 0 {
		fmt.Fprintf(stdout, "未索引页面: %v\n", result.UnindexedPages)
	}
	return nil
}

func runIndexRebuild(stdout, stderr io.Writer, opts *GlobalOptions) error {
	cfg, _, err := discoverConfig(opts)
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{
				Code:    "CONFIG_NOT_FOUND",
				Message: err.Error(),
			})
		}
		return err
	}

	result, err := wiki.RebuildIndex(wiki.NewOsFS(), cfg.WikiRoot)
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{
				Code:    "INDEX_REBUILD_FAILED",
				Message: err.Error(),
			})
		}
		return err
	}

	if opts.JSON {
		return output.JSON(stdout, true, result, nil)
	}

	fmt.Fprintf(stdout, "索引已重建，页面总数: %d\n", result.PageCount)
	for _, file := range result.RebuiltFiles {
		fmt.Fprintf(stdout, "  %s\n", file)
	}
	if result.BackupPath != "" {
		fmt.Fprintf(stdout, "旧 index 备份: %s\n", result.BackupPath)
	}
	return nil
}
