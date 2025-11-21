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

var summarizeCmd = &cobra.Command{
	Use:   "summary <url> [selector]",
	Short: "Summarize a web page using an LLM",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		applyMCPConfig(cmd)
		applyMCPHTTPConfig(cmd)
		applyGoogleCustomConfig(cmd)
		applyCacheConfig(cmd)
		applySummaryConfig(cmd)

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
			// WebDriverPort:    webDriverPort,
			// ChromeDriverPath: webDriverPath,
			Logger:           logger,
			AllowedURLGlobs:  httpAllowGlobs,
			DenyURLGlobs:     httpDenyGlobs,
			SummarizeBaseURL: summaryBaseURL,
			SummarizeApiKey:  summaryAPIKey,
			SummarizeModel:   summaryLLMModel,
			SummarizeShort:   summaryShort,
			UrlSelectors:     selectors,
		})
		if err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}
		if err := fetcher.Start(); err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}

		defer fetcher.Stop()

		ctx := context.Background()
		summary, err := fetcher.SummarizeURL(ctx, url, selector, false)
		if err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}

		fmt.Println(summary.ToYaml())
	},
}

// var webDriverLog string
var summaryLLMModel string
var summaryAPIKey string
var summaryBaseURL string
var summaryShort bool

func init() {
	// summarizeCmd.Flags().IntVar(&webDriverPort, "wd-port", 9515, "Use this port to communicate with chromedriver")
	// fetchCmd.Flags().StringVar(&webDriverLog, "wd-log", "", "Path to chromedriver log file")
	summarizeCmd.Flags().StringVar(&summaryLLMModel, "llm-model", "", "LLM Model name")
	summarizeCmd.Flags().StringVar(&summaryAPIKey, "llm-api-key", "", "LLM API Key (will also read LLM_API_KEY env var)")
	summarizeCmd.Flags().StringVar(&summaryBaseURL, "llm-base-url", "", "LLM Base URL")
	// summarizeCmd.Flags().StringVar(&webDriverPath, "wd-path", "/usr/bin/chromedriver", "Path to chromedriver")
	summarizeCmd.Flags().BoolVar(&summaryShort, "llm-short", false, "Return a short summary (default: auto length)")
	summarizeCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	fetchCmd.Flags().MarkHidden("md")

	rootCmd.AddCommand(summarizeCmd)
}
