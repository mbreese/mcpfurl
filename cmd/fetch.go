package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/mbreese/mcpfurl/fetchurl"
	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch <url> [selector]",
	Short: "Fetch a webpage and (optionally) convert it to Markdown",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		convertToMarkdown = convertToMarkdown || convertToMarkdown2
		applyMCPConfig(cmd)
		applyMCPHTTPConfig(cmd)
		applyGoogleCustomConfig(cmd)
		applyCacheConfig(cmd)

		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "You must specify a URL")
			os.Exit(1)
		}

		url := args[0]
		selector := ""
		if len(args) > 1 {
			selector = args[1]
		}

		// fmt.Fprintf(os.Stderr, "Fetching URL: %s\n", args[0])
		var logger *slog.Logger
		if verbose {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		}
		var fetcher *fetchurl.WebFetcher
		var err error
		fetcher, err = fetchurl.NewWebFetcher(fetchurl.WebFetcherOptions{
			ConvertAbsoluteHref: useAbsHref,
			// WebDriverPort:       webDriverPort,
			// ChromeDriverPath:    webDriverPath,
			// WebDriverLogging:    webDriverLog,
			Logger:          logger,
			UsePandoc:       usePandoc,
			AllowedURLGlobs: httpAllowGlobs,
			DenyURLGlobs:    httpDenyGlobs,
			UrlSelectors:    selectors,
		})
		if err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}
		if err := fetcher.Start(); err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}

		defer fetcher.Stop()

		// Example using a go routine / channel to do this async
		//
		// resCh := make(chan (*webfetcher.FetchedWebPageResult))
		// go func() {
		// 	resCh <- fetcher.FetchURLError(args[0])
		// }()
		// res := <-resCh
		// if res.Err != nil {
		// 	log.Fatalf("ERROR: %v\n", res.Err)
		// }
		// webpage := res.Page

		ctx := context.Background()
		if outputPNG == "" {
			webpage, err := fetcher.FetchURL(ctx, url, selector)
			if err != nil {
				log.Fatalf("ERROR: %v\n", err)
			}

			if webpage != nil {
				if convertToMarkdown {
					if md, err := fetcher.WebpageToMarkdownYaml(webpage); err == nil {
						fmt.Println(md)
					} else {
						fmt.Println(err)
					}
				} else {
					fmt.Println(webpage.Src)
				}
			} else {
				fmt.Fprintf(os.Stderr, "Unable to fetch web page: %s\n", url)
			}
		} else {
			data, err := fetcher.FetchURLPNG(ctx, url, selector)
			if err != nil {
				log.Fatalf("ERROR: %v\n", err)
			}

			if err := os.WriteFile(outputPNG, data, 0644); err != nil {
				log.Fatalf("ERROR: %v\n", err)
			}

		}
	},
}

var usePandoc bool
var convertToMarkdown bool
var convertToMarkdown2 bool
var useAbsHref bool
var verbose bool
var outputPNG string

// var webDriverPort int
// var webDriverPath string

// var webDriverLog string

func init() {
	// fetchCmd.Flags().IntVar(&webDriverPort, "wd-port", 9515, "Use this port to communicate with chromedriver")
	// fetchCmd.Flags().StringVar(&webDriverLog, "wd-log", "", "Path to chromedriver log file")
	// fetchCmd.Flags().StringVar(&webDriverPath, "wd-path", "/usr/bin/chromedriver", "Path to chromedriver")
	fetchCmd.Flags().BoolVar(&useAbsHref, "abspath", false, "Use absolute paths for a-hrefs/img-src")
	fetchCmd.Flags().BoolVar(&convertToMarkdown2, "md", false, "Alias for --markdown")
	fetchCmd.Flags().BoolVarP(&convertToMarkdown, "markdown", "m", false, "Convert HTML to Markdown")
	fetchCmd.Flags().BoolVar(&usePandoc, "pandoc", false, "Convert HTML to Markdown using pandoc")
	fetchCmd.Flags().StringVar(&outputPNG, "png", "", "Output screenshot to PNG file")
	fetchCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	fetchCmd.Flags().MarkHidden("md")

	rootCmd.AddCommand(fetchCmd)
}
