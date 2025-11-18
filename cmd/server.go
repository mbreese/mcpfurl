package cmd

import (
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/mbreese/mcpfurl/fetchurl"
	"github.com/mbreese/mcpfurl/mcpserver"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start an MCP server over stdio",
	Run: func(cmd *cobra.Command, args []string) {
		applyMCPConfig(cmd)
		applyMCPHTTPConfig(cmd)
		applyGoogleCustomConfig(cmd)
		applyCacheConfig(cmd)

		searchCacheExpires, err := fetchurl.ConvertTTLToDuration(searchCacheExpiresStr)
		if err != nil {
			log.Fatalf("Unable to parse cache-expires value: %s", searchCacheExpiresStr)
		}

		var logger *slog.Logger
		if verbose {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		}
		mcpserver.StartStdio(fetchurl.WebFetcherOptions{
			WebDriverPort:    webDriverPort,
			ChromeDriverPath: webDriverPath,
			// WebDriverLogging:   webDriverLog,
			Logger:             logger,
			MaxDownloadBytes:   fetchurl.DefaultMaxDownloadBytes,
			UsePandoc:          usePandoc,
			GoogleSearchCx:     googleCx,
			GoogleSearchKey:    googleKey,
			SearchEngine:       searchEngine,
			SearchCachePath:    searchCachePath,
			SearchCacheExpires: searchCacheExpires,
			AllowedURLGlobs:    httpAllowGlobs,
			DenyURLGlobs:       httpDenyGlobs,
			SummarizeBaseURL:   summaryBaseURL,
			SummarizeApiKey:    summaryAPIKey,
			SummarizeModel:     summaryLLMModel,
		}, mcpserver.MCPServerOptions{
			FetchDesc:   defaultFetchDesc,
			ImageDesc:   defaultImageDesc,
			SearchDesc:  defaultSearchDesc,
			SummaryDesc: defaultSummaryDesc,
		})
	},
}

var mcpHttpCmd = &cobra.Command{
	Use:   "mcp-http",
	Short: "Start an MCP server over HTTP",
	Run: func(cmd *cobra.Command, args []string) {
		applyMCPConfig(cmd)
		applyMCPHTTPConfig(cmd)
		applyGoogleCustomConfig(cmd)
		applyCacheConfig(cmd)

		var searchCacheExpires time.Duration
		if searchCachePath != "" && searchCacheExpiresStr != "" {
			var err error
			searchCacheExpires, err = fetchurl.ConvertTTLToDuration(searchCacheExpiresStr)
			if err != nil {
				log.Fatalf("Unable to parse cache-expires value: %s", searchCacheExpiresStr)
			}
		}

		var logger *slog.Logger
		if verbose {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		}
		mcpserver.StartHTTP(fetchurl.WebFetcherOptions{
			WebDriverPort:    webDriverPort,
			ChromeDriverPath: webDriverPath,
			// WebDriverLogging:   webDriverLog,
			Logger:             logger,
			MaxDownloadBytes:   fetchurl.DefaultMaxDownloadBytes,
			UsePandoc:          usePandoc,
			GoogleSearchCx:     googleCx,
			GoogleSearchKey:    googleKey,
			SearchEngine:       searchEngine,
			SearchCachePath:    searchCachePath,
			SearchCacheExpires: searchCacheExpires,
			AllowedURLGlobs:    httpAllowGlobs,
			DenyURLGlobs:       httpDenyGlobs,
			SummarizeBaseURL:   summaryBaseURL,
			SummarizeApiKey:    summaryAPIKey,
			SummarizeModel:     summaryLLMModel,
		}, mcpserver.MCPServerOptions{
			Addr:        mcpAddr,
			Port:        mcpPort,
			MasterKey:   masterKey,
			FetchDesc:   defaultFetchDesc,
			ImageDesc:   defaultImageDesc,
			SearchDesc:  defaultSearchDesc,
			SummaryDesc: defaultSummaryDesc,
		})
	},
}

var mcpPort int
var mcpAddr string
var masterKey string
var httpAllowGlobs []string
var httpDenyGlobs []string

var defaultFetchDesc string
var defaultImageDesc string
var defaultSearchDesc string
var defaultSummaryDesc string

var disableFetch bool
var disableImage bool
var disableSearch bool
var disableSummary bool

func init() {
	mcpHttpCmd.Flags().IntVarP(&mcpPort, "port", "p", 8080, "Start the MCP server on this port")
	mcpHttpCmd.Flags().StringVar(&mcpAddr, "addr", "0.0.0.0", "Bind to this address")
	mcpHttpCmd.Flags().IntVar(&webDriverPort, "wd-port", 9515, "Use this port to communicate with chromedriver")
	// mcpHttpCmd.Flags().StringVar(&webDriverLog, "wd-log", "", "Path to chromedriver log file")
	mcpHttpCmd.Flags().StringVar(&webDriverPath, "wd-path", "/usr/bin/chromedriver", "Path to chromedriver")
	mcpHttpCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	mcpHttpCmd.Flags().BoolVar(&usePandoc, "pandoc", false, "Convert HTML to Markdown using pandoc")
	mcpHttpCmd.Flags().BoolVar(&disableFetch, "disable-fetch", false, "Disable the Fetch function")
	mcpHttpCmd.Flags().BoolVar(&disableImage, "disable-image", false, "Disable the Image function")
	mcpHttpCmd.Flags().BoolVar(&disableSearch, "disable-search", false, "Disable the Search function")
	mcpHttpCmd.Flags().BoolVar(&disableSummary, "disable-summary", false, "Disable the Summary function")
	mcpHttpCmd.Flags().StringVar(&googleCx, "google-cx", "", "cx value for Google Custom Search")
	mcpHttpCmd.Flags().StringVar(&googleKey, "google-key", "", "API key for Google Custom Search")
	mcpHttpCmd.Flags().StringVar(&searchEngine, "search-engine", "google_custom", "Search engine to use (e.g. google_custom)")
	mcpHttpCmd.Flags().StringVar(&searchCachePath, "cache", "", "Path to the SQLite search cache database")
	mcpHttpCmd.Flags().StringVar(&searchCacheExpiresStr, "cache-expires", "", "Cache expiration time")
	mcpHttpCmd.Flags().StringVar(&masterKey, "master-key", "", "Require HTTP Authorization: Bearer <value> to access the MCP server")
	mcpHttpCmd.Flags().StringSliceVar(&httpAllowGlobs, "allow", nil, "Glob(s) of URLs the HTTP server may fetch (overrides config when set)")
	mcpHttpCmd.Flags().StringSliceVar(&httpDenyGlobs, "deny", nil, "Glob(s) of URLs the HTTP server must block (overrides config when set)")
	mcpHttpCmd.Flags().StringVar(&summaryLLMModel, "llm-model", "", "LLM Model name")
	mcpHttpCmd.Flags().StringVar(&summaryAPIKey, "llm-api-key", "", "LLM API Key (will also read LLM_API_KEY env var)")
	mcpHttpCmd.Flags().StringVar(&summaryBaseURL, "llm-base-url", "", "LLM Base URL")
	rootCmd.AddCommand(mcpHttpCmd)

	mcpCmd.Flags().IntVar(&webDriverPort, "wd-port", 9515, "Use this port to communicate with chromedriver")
	mcpCmd.Flags().StringVar(&webDriverPath, "wd-path", "/usr/bin/chromedriver", "Path to chromedriver")
	// mcpCmd.Flags().StringVar(&webDriverLog, "wd-log", "", "Path to chromedriver log file")
	mcpCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	mcpCmd.Flags().BoolVar(&usePandoc, "pandoc", false, "Convert HTML to Markdown using pandoc")
	mcpCmd.Flags().BoolVar(&disableFetch, "disable-fetch", false, "Disable the Fetch function")
	mcpCmd.Flags().BoolVar(&disableImage, "disable-image", false, "Disable the Image function")
	mcpCmd.Flags().BoolVar(&disableSearch, "disable-search", false, "Disable the Search function")
	mcpCmd.Flags().BoolVar(&disableSummary, "disable-summary", false, "Disable the Summary function")
	mcpCmd.Flags().StringVar(&googleCx, "google-cx", "", "cx value for Google Custom Search")
	mcpCmd.Flags().StringVar(&googleKey, "google-key", "", "API key for Google Custom Search")
	mcpCmd.Flags().StringVar(&searchEngine, "search-engine", "google_custom", "Search engine to use (e.g. google_custom)")
	mcpCmd.Flags().StringVar(&searchCachePath, "cache", "", "Path to the SQLite search cache database")
	mcpCmd.Flags().StringVar(&searchCacheExpiresStr, "cache-expires", "", "Cache expiration time")
	mcpCmd.Flags().StringVar(&summaryLLMModel, "llm-model", "", "LLM Model name")
	mcpCmd.Flags().StringVar(&summaryAPIKey, "llm-api-key", "", "LLM API Key (will also read LLM_API_KEY env var)")
	mcpCmd.Flags().StringVar(&summaryBaseURL, "llm-base-url", "", "LLM Base URL")
	rootCmd.AddCommand(mcpCmd)
}

// func httpAllowPatterns(cmd *cobra.Command) []string {
// 	if cmd.Flags().Changed("allow") {
// 		return clonePatterns(httpAllowGlobs)
// 	}
// 	return fetcherAllowPatterns()
// }

// func httpDisallowPatterns(cmd *cobra.Command) []string {
// 	if cmd.Flags().Changed("disallow") {
// 		return clonePatterns(httpDisallowGlobs)
// 	}
// 	return fetcherDenyPatterns()
// }
