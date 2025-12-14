package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mbreese/mcpfurl/fetchurl"
	"github.com/spf13/cobra"
)

var crawlCmd = &cobra.Command{
	Use:   "crawl <url>",
	Short: "Crawl a site and fetch linked pages",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		convertToMarkdown = convertToMarkdown || convertToMarkdown2
		applyMCPConfig(cmd)
		applyMCPHTTPConfig(cmd)
		applyCacheConfig(cmd)

		var cacheExpires time.Duration
		var err error
		if cachePath != "" {
			if cacheExpiresStr == "" {
				log.Fatalf("Provide --cache-expires or set cache.expires in config when cache is enabled")
			}
			cacheExpires, err = fetchurl.ConvertTTLToDuration(cacheExpiresStr)
			if err != nil {
				log.Fatalf("Unable to parse cache-expires value: %s", cacheExpiresStr)
			}
		}

		startURL := args[0]

		var logger *slog.Logger
		if verbose {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		}

		fetcher, err := fetchurl.NewWebFetcher(fetchurl.WebFetcherOptions{
			ConvertAbsoluteHref: useAbsHref,
			Logger:              logger,
			UsePandoc:           usePandoc,
			CachePath:           cachePath,
			CacheExpires:        cacheExpires,
			AllowedURLGlobs:     httpAllowGlobs,
			DenyURLGlobs:        httpDenyGlobs,
			UrlSelectors:        selectors,
		})
		if err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}
		if err := fetcher.Start(); err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}
		defer fetcher.Stop()

		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, time.Duration(maxCrawlSeconds)*time.Second)
		defer cancel()

		pages, err := fetcher.Crawl(ctx, startURL, maxCrawlDepth, maxCrawlPages, sameBasePathOnly, crawlSelector)
		if err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}

		if convertToMarkdown {
			var b strings.Builder
			for i, page := range pages {
				md, err := fetcher.WebpageToMarkdownYaml(page)
				if err != nil {
					log.Fatalf("ERROR converting page to markdown: %v\n", err)
				}
				b.WriteString(md)
				if i < len(pages)-1 {
					b.WriteString("\n")
				}
			}
			fmt.Println(b.String())
			return
		}

		for _, page := range pages {
			fmt.Printf("URL: %s\n", page.CurrentURL)
			fmt.Printf("HTML:\n%s\n", page.Src)
		}
	},
}

var maxCrawlPages int
var maxCrawlDepth int
var maxCrawlSeconds int
var sameBasePathOnly bool
var crawlSelector string

func init() {
	crawlCmd.Flags().BoolVar(&useAbsHref, "abspath", false, "Use absolute paths for a-hrefs/img-src")
	crawlCmd.Flags().BoolVar(&convertToMarkdown2, "md", false, "Alias for --markdown")
	crawlCmd.Flags().BoolVarP(&convertToMarkdown, "markdown", "m", false, "Convert HTML to Markdown")
	crawlCmd.Flags().BoolVar(&usePandoc, "pandoc", false, "Convert HTML to Markdown using pandoc")
	crawlCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	crawlCmd.Flags().IntVar(&maxCrawlPages, "max-pages", 20, "Maximum pages to crawl")
	crawlCmd.Flags().IntVar(&maxCrawlDepth, "max-depth", 2, "Maximum crawl depth (root=0)")
	crawlCmd.Flags().IntVar(&maxCrawlSeconds, "timeout-seconds", 120, "Maximum seconds to crawl before timing out")
	crawlCmd.Flags().BoolVar(&sameBasePathOnly, "same-base-path", true, "Restrict crawling to the starting host and path prefix")
	crawlCmd.Flags().StringVar(&crawlSelector, "selector", "", "CSS selector to extract content for every crawled page")
	crawlCmd.Flags().StringVar(&cachePath, "cache", "", "Path to the SQLite cache database")
	crawlCmd.Flags().StringVar(&cacheExpiresStr, "cache-expires", "", "Cache expiration time")

	rootCmd.AddCommand(crawlCmd)
}
