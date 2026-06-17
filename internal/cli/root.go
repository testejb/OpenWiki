package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bytedance/openwiki/internal/config"
)

type GlobalOptions struct {
	ConfigPath string
	JSON       bool
	Quiet      bool
	Verbose    bool
	Force      bool
	NoColor    bool
}

func Run(args []string, version, buildTime string) error {
	return RunWithIO(args, version, buildTime, os.Stdout, os.Stderr)
}

func RunWithIO(args []string, version, buildTime string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printHelp(stdout)
		return nil
	}

	rootFlags := flag.NewFlagSet("openwiki", flag.ContinueOnError)
	rootFlags.SetOutput(stderr)

	var opts GlobalOptions
	rootFlags.StringVar(&opts.ConfigPath, "config", "", "指定配置文件路径")
	rootFlags.StringVar(&opts.ConfigPath, "c", "", "指定配置文件路径 (短选项)")
	rootFlags.BoolVar(&opts.JSON, "json", false, "启用 JSON 输出模式")
	rootFlags.BoolVar(&opts.Quiet, "quiet", false, "抑制非错误输出")
	rootFlags.BoolVar(&opts.Quiet, "q", false, "抑制非错误输出 (短选项)")
	rootFlags.BoolVar(&opts.Verbose, "verbose", false, "输出详细信息")
	rootFlags.BoolVar(&opts.Verbose, "V", false, "输出详细信息 (短选项)")
	rootFlags.BoolVar(&opts.Force, "force", false, "跳过确认提示")
	rootFlags.BoolVar(&opts.Force, "f", false, "跳过确认提示 (短选项)")
	rootFlags.BoolVar(&opts.NoColor, "no-color", false, "禁用颜色输出")

	showVersion := rootFlags.Bool("version", false, "显示版本信息")
	showVersionShort := rootFlags.Bool("v", false, "显示版本信息 (短选项)")
	showHelp := rootFlags.Bool("help", false, "显示帮助信息")
	showHelpShort := rootFlags.Bool("h", false, "显示帮助信息 (短选项)")

	if err := rootFlags.Parse(args); err != nil {
		return err
	}

	if *showVersion || *showVersionShort {
		fmt.Fprintf(stdout, "openwiki 版本 %s (构建时间: %s)\n", version, buildTime)
		return nil
	}

	if *showHelp || *showHelpShort {
		printHelp(stdout)
		return nil
	}

	remaining := rootFlags.Args()
	if len(remaining) == 0 {
		printHelp(stdout)
		return nil
	}

	remaining = scanGlobalFlags(remaining, &opts)

	subcommand := remaining[0]
	subArgs := remaining[1:]

	switch subcommand {
	case "config":
		return runConfig(stdout, stderr, &opts, subArgs)
	case "init":
		return runInit(stdout, stderr, &opts, subArgs)
	case "status":
		return runStatus(stdout, stderr, &opts, subArgs)
	case "page":
		return runPage(stdout, stderr, &opts, subArgs)
	case "index":
		return runIndex(stdout, stderr, &opts, subArgs)
	case "log":
		return runLog(stdout, stderr, &opts, subArgs)
	case "sync":
		return runSync(stdout, stderr, &opts, subArgs)
	case "candidate":
		return runCandidate(stdout, stderr, &opts, subArgs)
	default:
		return fmt.Errorf("未知命令: %s\n使用 'openwiki --help' 查看可用命令", subcommand)
	}
}

func printHelp(w io.Writer) {
	fmt.Fprintf(w, `openwiki - 个人 Wiki 管理工具

用法:
  openwiki [全局选项] <命令> [命令选项]

全局选项:
  --config, -c <path>  指定配置文件路径
  --json              启用 JSON 输出模式
  --quiet, -q         抑制非错误输出
  --verbose, -V       输出详细信息
  --force, -f         跳过确认提示
  --no-color          禁用颜色输出
  --help, -h          显示帮助信息
  --version, -v       显示版本信息

可用命令:
  init     初始化 wiki 实例
  config   管理配置
  status   查看 wiki 状态
  page     管理 wiki 页面
  index    检查和重建分层索引
  log      查看操作日志
  sync     同步到云端
  candidate 管理候选内容

使用 'openwiki <命令> --help' 查看命令详情
`)
}

func discoverConfig(opts *GlobalOptions) (*config.Config, *config.DiscoveryResult, error) {
	discoverer := config.NewDefaultDiscoverer()
	result, err := discoverer.Discover(opts.ConfigPath)
	if err != nil {
		return nil, nil, err
	}

	cfg, err := config.Load(result.Path)
	if err != nil {
		return nil, nil, err
	}

	return cfg, result, nil
}

func scanGlobalFlags(args []string, opts *GlobalOptions) []string {
	var result []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--config", "-c":
			if i+1 < len(args) {
				opts.ConfigPath = args[i+1]
				i++
			} else {
				result = append(result, arg)
			}
		case "--force", "-f":
			opts.Force = true
		case "--quiet", "-q":
			opts.Quiet = true
		case "--verbose", "-V":
			opts.Verbose = true
		case "--no-color":
			opts.NoColor = true
		default:
			switch {
			case strings.HasPrefix(arg, "--config="):
				opts.ConfigPath = strings.TrimPrefix(arg, "--config=")
			case strings.HasPrefix(arg, "-c="):
				opts.ConfigPath = strings.TrimPrefix(arg, "-c=")
			default:
				result = append(result, arg)
			}
		}
	}
	return result
}
