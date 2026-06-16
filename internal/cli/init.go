package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/bytedance/openwiki/internal/config"
	"github.com/bytedance/openwiki/internal/output"
	"github.com/bytedance/openwiki/internal/wiki"
)

func runInit(stdout, stderr io.Writer, opts *GlobalOptions, args []string) error {
	initFlags := flag.NewFlagSet("init", flag.ContinueOnError)
	initFlags.SetOutput(stderr)

	nonInteractive := initFlags.Bool("non-interactive", false, "非交互模式")
	primaryLang := initFlags.String("primary-language", "zh", "主语言")
	secondaryLang := initFlags.String("secondary-language", "en", "副语言")

	if err := initFlags.Parse(args); err != nil {
		return err
	}

	remaining := initFlags.Args()
	var wikiRoot string
	if len(remaining) == 0 {
		wikiRoot = "./openwiki/"
	} else {
		wikiRoot = remaining[0]
	}

	cfg := &config.Config{
		WikiRoot: wikiRoot,
		Wiki: config.WikiConfig{
			PrimaryLanguage:   *primaryLang,
			SecondaryLanguage: *secondaryLang,
		},
	}

	fs := wiki.NewOsFS()

	var err error
	if opts.Force {
		err = wiki.InitForce(fs, wikiRoot, cfg)
	} else {
		err = wiki.Init(fs, wikiRoot, cfg)
	}

	if err != nil {
		code := "INTERNAL"
		if os.IsExist(err) || err.Error() == fmt.Sprintf("wiki 实例已存在: %s", wikiRoot) {
			code = "WIKI_ALREADY_EXISTS"
		}

		if opts.JSON {
			return output.JSON(stdout, false, nil, &output.ErrorInfo{
				Code:    code,
				Message: err.Error(),
			})
		}
		return err
	}

	if opts.JSON {
		return output.JSON(stdout, true, map[string]interface{}{
			"wiki_root": wikiRoot,
			"created": []string{
				wikiRoot + "/openwiki.toml",
				wikiRoot + "/wiki/index.md",
				wikiRoot + "/wiki/log.md",
				wikiRoot + "/wiki/pages/",
				wikiRoot + "/wiki/indexes/",
				wikiRoot + "/wiki/indexes/scopes.md",
				wikiRoot + "/wiki/indexes/entities.md",
				wikiRoot + "/wiki/indexes/concepts.md",
				wikiRoot + "/wiki/indexes/tags.md",
				wikiRoot + "/wiki/indexes/recent.md",
				wikiRoot + "/wiki/indexes/hot.md",
				wikiRoot + "/wiki/indexes/query-usage.jsonl",
				wikiRoot + "/raw/",
				wikiRoot + "/concepts/",
				wikiRoot + "/entities/",
			},
		}, nil)
	}

	fmt.Fprintf(stdout, "wiki 实例已创建: %s\n", wikiRoot)
	_ = nonInteractive
	return nil
}
