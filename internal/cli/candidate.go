package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/bytedance/openwiki/internal/candidate"
	"github.com/bytedance/openwiki/internal/output"
)

func runCandidate(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("candidate 需要子命令: codeagent")
	}

	subcommand := args[0]
	subArgs := scanGlobalFlags(args[1:], opts)

	switch subcommand {
	case "codeagent":
		return runCandidateCodeAgent(stdout, stderr, opts, subArgs)
	default:
		return fmt.Errorf("未知 candidate 子命令: %s", subcommand)
	}
}

func runCandidateCodeAgent(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("candidate codeagent 需要子命令: scan, commit, status")
	}

	subcommand := args[0]
	subArgs := scanGlobalFlags(args[1:], opts)

	switch subcommand {
	case "scan":
		return runCandidateCodeAgentScan(stdout, stderr, opts, subArgs)
	case "commit":
		return runCandidateCodeAgentCommit(stdout, stderr, opts, subArgs)
	case "status":
		return runCandidateCodeAgentStatus(stdout, stderr, opts, subArgs)
	default:
		return fmt.Errorf("未知 candidate codeagent 子命令: %s", subcommand)
	}
}

func runCandidateCodeAgentScan(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	flags := flag.NewFlagSet("openwiki candidate codeagent scan", flag.ContinueOnError)
	flags.SetOutput(stderr)

	jsonOutput := flags.Bool("json", false, "启用 JSON 输出模式")
	initialDays := flags.Int("initial-days", 0, "首次扫描最近 N 天记录")
	maxRecords := flags.Int("max-records", 0, "本次最多处理 N 条记录")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *jsonOutput {
		opts.JSON = true
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("candidate codeagent scan 不接受额外参数: %s", strings.Join(flags.Args(), " "))
	}

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

	codeAgentCfg := candidate.ResolveCodeAgentConfig(cfg)
	scanResult, err := candidate.ScanCodeAgent(codeAgentCfg, candidate.ScanOptions{
		ConfigPath:       result.Path,
		WikiRoot:         cfg.WikiRoot,
		InitialDays:      *initialDays,
		MaxRecordsPerRun: *maxRecords,
	})
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{
				Code:    "CANDIDATE_SCAN_FAILED",
				Message: err.Error(),
			})
		}
		return err
	}

	if opts.JSON {
		return output.JSON(stdout, true, scanResult, nil)
	}

	fmt.Fprintf(stdout, "候选扫描完成: %d 条记录\n", scanResult.Records.Total)
	fmt.Fprintf(stdout, "pending: %s\n", scanResult.PendingPath)
	return nil
}

func runCandidateCodeAgentCommit(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	flags := flag.NewFlagSet("openwiki candidate codeagent commit", flag.ContinueOnError)
	flags.SetOutput(stderr)

	jsonOutput := flags.Bool("json", false, "启用 JSON 输出模式")
	pendingPath := flags.String("pending", "", "待提交 pending 文件路径")
	reviewDocURL := flags.String("review-doc-url", "", "评审文档 URL")
	snapshotPath := flags.String("snapshot", "", "评审快照文件路径")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *jsonOutput {
		opts.JSON = true
	}
	if flags.NArg() > 0 {
		return candidateCommitError(stdout, opts, fmt.Errorf("candidate codeagent commit 不接受额外参数: %s", strings.Join(flags.Args(), " ")))
	}
	if strings.TrimSpace(*pendingPath) == "" {
		return candidateCommitError(stdout, opts, fmt.Errorf("缺少 --pending"))
	}
	if strings.TrimSpace(*reviewDocURL) == "" {
		return candidateCommitError(stdout, opts, fmt.Errorf("缺少 --review-doc-url"))
	}
	if strings.TrimSpace(*snapshotPath) == "" {
		return candidateCommitError(stdout, opts, fmt.Errorf("缺少 --snapshot"))
	}

	commitResult, err := candidate.CommitCodeAgent(*pendingPath, *reviewDocURL, *snapshotPath, time.Time{})
	if err != nil {
		return candidateCommitError(stdout, opts, err)
	}

	if opts.JSON {
		return output.JSON(stdout, true, commitResult, nil)
	}

	fmt.Fprintf(stdout, "候选提交完成\n")
	fmt.Fprintf(stdout, "state: %s\n", commitResult.StatePath)
	return nil
}

func runCandidateCodeAgentStatus(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	flags := flag.NewFlagSet("openwiki candidate codeagent status", flag.ContinueOnError)
	flags.SetOutput(stderr)

	jsonOutput := flags.Bool("json", false, "启用 JSON 输出模式")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *jsonOutput {
		opts.JSON = true
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("candidate codeagent status 不接受额外参数: %s", strings.Join(flags.Args(), " "))
	}

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

	statusResult, err := candidate.StatusCodeAgent(candidate.ResolveCodeAgentConfig(cfg))
	if err != nil {
		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{
				Code:    "CANDIDATE_STATUS_FAILED",
				Message: err.Error(),
			})
		}
		return err
	}

	if opts.JSON {
		return output.JSON(stdout, true, statusResult, nil)
	}

	fmt.Fprintf(stdout, "候选状态: tracked_files=%d\n", statusResult.TrackedFiles)
	fmt.Fprintf(stdout, "state: %s\n", statusResult.StatePath)
	return nil
}

func candidateCommitError(stdout io.Writer, opts *GlobalOptions, err error) error {
	if opts.JSON {
		return output.JSON(stdout, false, nil, &output.ErrorInfo{
			Code:    "CANDIDATE_COMMIT_FAILED",
			Message: err.Error(),
		})
	}
	return err
}
