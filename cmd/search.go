package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/mbreese/mcpfurl/fetchurl"
	"github.com/mbreese/mcpfurl/mcpserver"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search query",
	Short: "Run a web search and return the results in Markdown",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		applyGoogleCustomConfig(cmd)

		convertToMarkdown = convertToMarkdown || convertToMarkdown2

		if googleCx == "" || googleKey == "" {
			log.Fatalf("Provide --google-cx/--google-key or set google_custom.cx/google_custom.key in your config.")
		}

		searchCacheExpires, err := fetchurl.ConvertTTLToDuration(searchCacheExpiresStr)
		if err != nil {
			log.Fatalf("Unable to parse cache-expires value")
		}

		if len(args) < 1 {
			log.Fatalf("You must specify a URL")
		}

		query := args[0]

		// fmt.Fprintf(os.Stderr, "Fetching URL: %s\n", args[0])
		var logger *slog.Logger
		if verbose {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		}
		var fetcher *fetchurl.WebFetcher
		fetcher, err = fetchurl.NewWebFetcher(fetchurl.WebFetcherOptions{
			ConvertAbsoluteHref: useAbsHref,
			WebDriverPort:       webDriverPort,
			ChromeDriverPath:    webDriverPath,
			Logger:              logger,
			SearchEngine:        searchEngine,
			GoogleSearchCx:      googleCx,
			GoogleSearchKey:     googleKey,
			SearchCachePath:     searchCachePath,
			SearchCacheExpires:  searchCacheExpires,
		})
		if err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}
		if err := fetcher.Start(); err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}

		defer fetcher.Stop()

		ctx := context.Background()
		results, err := fetcher.SearchJSON(ctx, query)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		if convertToMarkdown {
			ret := "# Search results\n\n"
			for _, result := range results {
				ret += fmt.Sprintf("* [%s](%s) - %s\n", result.Title, result.Link, result.Snippet)
			}
			fmt.Println(ret)
		} else {
			wso := mcpserver.WebSearchOutput{Query: query, Results: results}
			if raw, err := json.Marshal(wso); err == nil {
				fmt.Println(string(raw))
			} else {
				log.Fatalf("error encoding results: %v", err)
			}
		}
	},
}

var googleCx string
var googleKey string
var searchEngine = "google_custom"
var searchCachePath string
var searchCacheExpiresStr string

func init() {
	searchCmd.Flags().IntVar(&webDriverPort, "wd-port", 9515, "Use this port to communicate with chromedriver")
	searchCmd.Flags().StringVar(&webDriverPath, "wd-path", "/usr/bin/chromedriver", "Path to chromedriver")
	searchCmd.Flags().StringVar(&googleCx, "google-cx", "", "cx value for Google Custom Search")
	searchCmd.Flags().StringVar(&googleKey, "google-key", "", "API key for Google Custom Search")
	searchCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	searchCmd.Flags().BoolVar(&convertToMarkdown2, "md", false, "Alias for --markdown")
	searchCmd.Flags().BoolVarP(&convertToMarkdown, "markdown", "m", false, "Convert HTML to Markdown")
	searchCmd.Flags().StringVar(&searchCachePath, "search-cache", "", "Path to the SQLite search cache database")
	searchCmd.Flags().StringVar(&searchCacheExpiresStr, "search-expires", "", "Max time to keep a search result (ex: 12h, 30m, or 120s)")
	searchCmd.Flags().MarkHidden("md")

	rootCmd.AddCommand(searchCmd)
}
