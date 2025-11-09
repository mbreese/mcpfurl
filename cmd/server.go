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
		})
	},
}

var mcpHttpCmd = &cobra.Command{
	Use:   "mcp-http",
	Short: "Start an MCP server over HTTP",
	Run: func(cmd *cobra.Command, args []string) {
		applyMCPHTTPConfig(cmd)
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
		mcpserver.StartHTTP(mcpAddr, mcpPort, fetchurl.WebFetcherOptions{
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
		}, masterKey)
	},
}

var mcpPort int
var mcpAddr string
var masterKey string

func init() {
	mcpHttpCmd.Flags().IntVarP(&mcpPort, "port", "p", 8080, "Start the MCP server on this port")
	mcpHttpCmd.Flags().StringVar(&mcpAddr, "addr", "0.0.0.0", "Bind to this address")
	mcpHttpCmd.Flags().IntVar(&webDriverPort, "wd-port", 9515, "Use this port to communicate with chromedriver")
	// mcpHttpCmd.Flags().StringVar(&webDriverLog, "wd-log", "", "Path to chromedriver log file")
	mcpHttpCmd.Flags().StringVar(&webDriverPath, "wd-path", "/usr/bin/chromedriver", "Path to chromedriver")
	mcpHttpCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	mcpHttpCmd.Flags().BoolVar(&usePandoc, "pandoc", false, "Convert HTML to Markdown using pandoc")
	mcpHttpCmd.Flags().StringVar(&googleCx, "google-cx", "", "cx value for Google Custom Search")
	mcpHttpCmd.Flags().StringVar(&googleKey, "google-key", "", "API key for Google Custom Search")
	mcpHttpCmd.Flags().StringVar(&searchEngine, "search-engine", "google_custom", "Search engine to use (e.g. google_custom)")
	mcpHttpCmd.Flags().StringVar(&searchCachePath, "cache", "", "Path to the SQLite search cache database")
	mcpHttpCmd.Flags().StringVar(&searchCacheExpiresStr, "cache-expires", "", "Cache expiration time")
	mcpHttpCmd.Flags().StringVar(&masterKey, "master-key", "", "Require HTTP Authorization: Bearer <value> to access the MCP server")
	rootCmd.AddCommand(mcpHttpCmd)

	mcpCmd.Flags().IntVar(&webDriverPort, "wd-port", 9515, "Use this port to communicate with chromedriver")
	mcpCmd.Flags().StringVar(&webDriverPath, "wd-path", "/usr/bin/chromedriver", "Path to chromedriver")
	// mcpCmd.Flags().StringVar(&webDriverLog, "wd-log", "", "Path to chromedriver log file")
	mcpCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	mcpCmd.Flags().BoolVar(&usePandoc, "pandoc", false, "Convert HTML to Markdown using pandoc")
	mcpCmd.Flags().StringVar(&googleCx, "google-cx", "", "cx value for Google Custom Search")
	mcpCmd.Flags().StringVar(&googleKey, "google-key", "", "API key for Google Custom Search")
	mcpCmd.Flags().StringVar(&searchEngine, "search-engine", "google_custom", "Search engine to use (e.g. google_custom)")
	mcpCmd.Flags().StringVar(&searchCachePath, "cache", "", "Path to the SQLite search cache database")
	mcpCmd.Flags().StringVar(&searchCacheExpiresStr, "cache-expires", "", "Cache expiration time")
	rootCmd.AddCommand(mcpCmd)
}
