package mcpserver

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mbreese/mcpfurl/fetchurl"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type WebFetchParams struct {
	URL string `json:"url" jsonschema:"The URL of the webpage to fetch"`
}
type WebSummaryParams struct {
	URL   string `json:"url" jsonschema:"The URL of the webpage to summarize"`
	Short bool   `json:"short" jsonschema:"Return a short summary"`
}

type WebSearchParams struct {
	Query          string `json:"query" jsonschema:"The web search to perform"`
	OutputMarkdown bool   `json:"markdown_output,omitempty" jsonschema:"Output the results in Markdown format"`
}

type WebFetchOutput struct {
	Content string `json:"content" jsonschema:"The content of the webpage converted to Markdown format"`
	Error   string `json:"error,omitempty" jsonschema:"Any error messages"`
}

type WebSummaryOutput struct {
	TargetURL  string `json:"target_url"  jsonschema:"The original target URL"`
	CurrentURL string `json:"current_url" jsonschema:"The final URL after any redirects"`
	Title      string `json:"title"       jsonschema:"The page title"`
	Text       string `json:"text"        jsonschema:"Full page content as Markdown"`
	Summary    string `json:"summary"     jsonschema:"LLM-generated summary of the page"`
	Error      string `json:"error,omitempty" jsonschema:"Any error messages"`
}

type WebSearchOutput struct {
	Query           string                  `json:"query" jsonschema:"The query for this search"`
	ResultsMarkdown string                  `json:"markdown_results,omitempty" jsonschema:"The search results in Markdown format"`
	Results         []fetchurl.SearchResult `json:"results,omitempty" jsonschema:"The search results in JSON format"`
	Error           string                  `json:"error,omitempty" jsonschema:"Any error messages"`
}

type ImageFetchParams struct {
	URL string `json:"url" jsonschema:"The URL of the image/PDF to fetch"`
}

type ImageFetchOutput struct {
	Filename    string `json:"filename,omitempty" jsonschema:"The detected filename from the URL path"`
	ContentType string `json:"content_type,omitempty" jsonschema:"The Content-Type header returned by the server"`
	DataBase64  string `json:"data_base64" jsonschema:"Base64 encoded binary of the downloaded resource"`
	Error       string `json:"error,omitempty" jsonschema:"Any error messages"`
}

type MCPServerOptions struct {
	Addr           string
	Port           int
	MasterKey      string
	FetchDesc      string
	ImageDesc      string
	SearchDesc     string
	SummaryDesc    string
	DisableSearch  bool
	DisableFetch   bool
	DisableImage   bool
	DisableSummary bool
	EnableAPI      bool // expose REST API endpoints under /api/
	CrawlResources []CrawlResourceConfig
}

var fetcher *fetchurl.WebFetcher

// var server *mcp.Server
var logger *slog.Logger

type CrawlResourceConfig struct {
	URL          string
	Depth        int
	MaxPages     int
	Selector     string
	SameBasePath bool
}

func fetchPage(ctx context.Context, req *mcp.CallToolRequest, args WebFetchParams) (*mcp.CallToolResult, *WebFetchOutput, error) {
	if args.URL == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Missing URL"},
			},
		}, &WebFetchOutput{Error: "Missing URL"}, nil
	}
	webpage, err := fetcher.FetchURL(ctx, args.URL, "")
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error fetching URL: %s => %v", args.URL, err)},
			},
		}, &WebFetchOutput{Error: fmt.Sprintf("Error fetching URL: %s => %v", args.URL, err)}, nil
	}

	if markdown, err := fetcher.WebpageToMarkdownYaml(webpage); err == nil {
		return nil, &WebFetchOutput{Content: markdown}, nil
	}

	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Error converting webpage: %s", err)},
		},
	}, &WebFetchOutput{Error: fmt.Sprintf("Error converting webpage: %s", err)}, nil
}

func webSearch(ctx context.Context, req *mcp.CallToolRequest, args WebSearchParams) (*mcp.CallToolResult, *WebSearchOutput, error) {
	if args.Query == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Missing argument: \"query\""},
			},
		}, &WebSearchOutput{Query: args.Query, Error: "Missing argument: \"query\""}, nil
	}

	logger.Info(fmt.Sprintf("Search: %s", args.Query))

	results, err := fetcher.SearchJSON(ctx, args.Query)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
		}, &WebSearchOutput{Query: args.Query, Error: fmt.Sprintf("Error: %v", err)}, nil
	}

	if args.OutputMarkdown {
		ret := "# Search results\n\n"
		for _, result := range results {
			ret += fmt.Sprintf("Title: %s\nLink: %s\nSnippet: %s\n\n---\n\n", result.Title, result.Link, result.Snippet)
		}
		return nil, &WebSearchOutput{Query: args.Query, ResultsMarkdown: ret}, nil
	}

	return nil, &WebSearchOutput{Query: args.Query, Results: results}, nil
}

func summarizePage(ctx context.Context, req *mcp.CallToolRequest, args WebSummaryParams) (*mcp.CallToolResult, *WebSummaryOutput, error) {
	if args.URL == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Missing argument: \"url\""},
			},
		}, &WebSummaryOutput{Error: "Missing argument: \"url\""}, nil
	}

	webpage, err := fetcher.SummarizeURL(ctx, args.URL, "", args.Short)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error fetching URL: %s => %v", args.URL, err)},
			},
		}, &WebSummaryOutput{Error: fmt.Sprintf("Error fetching URL: %s => %v", args.URL, err)}, nil
	}

	return nil, &WebSummaryOutput{
		TargetURL:  webpage.TargetURL,
		CurrentURL: webpage.CurrentURL,
		Title:      webpage.Title,
		Text:       webpage.Text,
		Summary:    webpage.Summary,
	}, nil

}

func fetchImage(ctx context.Context, req *mcp.CallToolRequest, args ImageFetchParams) (*mcp.CallToolResult, *ImageFetchOutput, error) {
	if args.URL == "" {
		return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Missing URL"},
				},
			}, &ImageFetchOutput{
				Error: "Missing URL",
			}, nil
	}

	logger.Info(fmt.Sprintf("Downloading asset: %s", args.URL))
	resource, err := fetcher.DownloadResource(ctx, args.URL)
	if err != nil {
		return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: err.Error()},
				},
			}, &ImageFetchOutput{
				Error: err.Error(),
			}, nil
	}

	return nil, &ImageFetchOutput{
		Filename:    resource.Filename,
		ContentType: resource.ContentType,
		DataBase64:  base64.StdEncoding.EncodeToString(resource.Body),
	}, nil
}

func browserFetchImage(ctx context.Context, req *mcp.CallToolRequest, args ImageFetchParams) (*mcp.CallToolResult, *ImageFetchOutput, error) {
	if args.URL == "" {
		return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Missing URL"},
				},
			}, &ImageFetchOutput{
				Error: "Missing URL",
			}, nil
	}

	logger.Info(fmt.Sprintf("Browser downloading asset: %s", args.URL))
	resource, err := fetcher.BrowserDownloadResource(ctx, args.URL)
	if err != nil {
		return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: err.Error()},
				},
			}, &ImageFetchOutput{
				Error: err.Error(),
			}, nil
	}

	return nil, &ImageFetchOutput{
		Filename:    resource.Filename,
		ContentType: resource.ContentType,
		DataBase64:  base64.StdEncoding.EncodeToString(resource.Body),
	}, nil
}


func addCrawlResources(server *mcp.Server, fetcher *fetchurl.WebFetcher, crawlCfg []CrawlResourceConfig) {
	if fetcher == nil || len(crawlCfg) == 0 {
		return
	}

	cache := make(map[string]string) // uri -> markdown

	for _, cfg := range crawlCfg {
		if cfg.URL == "" {
			continue
		}
		depth := cfg.Depth
		if depth <= 0 {
			depth = 1
		}
		maxPages := cfg.MaxPages
		if maxPages <= 0 {
			maxPages = 20
		}
		timeout := time.Duration(maxPages*2) * time.Second
		if timeout < 30*time.Second {
			timeout = 30 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		pages, err := fetcher.Crawl(ctx, cfg.URL, depth, maxPages, cfg.SameBasePath, cfg.Selector)
		cancel()
		if err != nil {
			logger.Warn("crawl resource failed", slog.String("url", cfg.URL), slog.Any("error", err))
			continue
		}

		for _, page := range pages {
			markdown, err := fetcher.WebpageToMarkdownYaml(page)
			if err != nil {
				logger.Warn("markdown conversion failed", slog.String("url", page.CurrentURL), slog.Any("error", err))
				continue
			}
			uri := page.CurrentURL
			if _, exists := cache[uri]; exists {
				continue
			}
			cache[uri] = markdown
			resource := &mcp.Resource{
				URI:         uri,
				Name:        uri,
				MIMEType:    "text/markdown",
				Description: fmt.Sprintf("Crawled page from %s", html.EscapeString(cfg.URL)),
			}
			server.AddResource(resource, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				content, ok := cache[req.Params.URI]
				if !ok {
					return nil, mcp.ResourceNotFoundError(req.Params.URI)
				}
				return &mcp.ReadResourceResult{
					Contents: []*mcp.ResourceContents{{
						URI:      req.Params.URI,
						MIMEType: "text/markdown",
						Text:     content,
					}},
				}, nil
			})
		}
	}
}

func createMCPServer(mcpOpts MCPServerOptions, fetcher *fetchurl.WebFetcher) *mcp.Server {
	if mcpOpts.FetchDesc == "" {
		mcpOpts.FetchDesc = "Fetch a webpage and return the content in Markdown format"
	}
	if mcpOpts.SearchDesc == "" {
		mcpOpts.SearchDesc = "Perform a web search and return the results in Markdown format"
	}
	if mcpOpts.ImageDesc == "" {
		mcpOpts.ImageDesc = "Download an image or binary file and return it as base64 data"
	}
	if mcpOpts.SummaryDesc == "" {
		mcpOpts.SummaryDesc = "Summarize a webpage and return the summary in Markdown format"
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "mcpfurl", Version: "v0.0.1"}, nil)

	if !mcpOpts.DisableFetch {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "web_fetch",
			Description: mcpOpts.FetchDesc,
		}, fetchPage)
	}

	if !mcpOpts.DisableImage {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "image_fetch",
			Description: mcpOpts.ImageDesc,
		}, fetchImage)

		mcp.AddTool(server, &mcp.Tool{
			Name:        "browser_image_fetch",
			Description: "Download an image using headless Chrome (bypasses bot detection/reCAPTCHA). Returns base64 data.",
		}, browserFetchImage)
	}

	if !mcpOpts.DisableSummary {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "web_summary",
			Description: mcpOpts.SummaryDesc,
		}, summarizePage)
	}

	if !mcpOpts.DisableSearch {
		if fetcher != nil && fetcher.HasSearch() {
			// only expose the web_search tool if we have a valid search
			mcp.AddTool(server, &mcp.Tool{
				Name:        "web_search",
				Description: mcpOpts.SearchDesc,
			}, webSearch)
		}
	}

	addCrawlResources(server, fetcher, mcpOpts.CrawlResources)

	return server
}

func StartStdio(opts fetchurl.WebFetcherOptions, mcpOpts MCPServerOptions) {
	opts.ConvertAbsoluteHref = true
	logger = opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	var err error
	if fetcher, err = fetchurl.NewWebFetcher(opts); err != nil {
		log.Fatalf("ERROR: %v\n", err)
	}
	if err := fetcher.Start(); err != nil {
		log.Fatalf("ERROR: %v\n", err)
	}
	defer fetcher.Stop()
	server := createMCPServer(mcpOpts, fetcher)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

// ── REST API handlers ─────────────────────────────────────────────────────

// apiWebFetch handles GET /api/fetch?url=...
// Returns the webpage content as markdown.
func apiWebFetch(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, `{"error":"missing url parameter"}`, http.StatusBadRequest)
		return
	}
	if fetcher == nil {
		http.Error(w, `{"error":"fetcher not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	logger.Info(fmt.Sprintf("API web_fetch: %s", url))
	page, err := fetcher.FetchURL(r.Context(), url, "")
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	md, _ := fetcher.WebpageToMarkdownYaml(page)
	writeJSON(w, http.StatusOK, map[string]any{
		"target_url":  page.TargetURL,
		"current_url": page.CurrentURL,
		"title":       page.Title,
		"content":     md,
	})
}

// apiWebSummary handles GET /api/summary?url=...&short=true
// Returns a summarized version of the webpage.
func apiWebSummary(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, `{"error":"missing url parameter"}`, http.StatusBadRequest)
		return
	}
	if fetcher == nil {
		http.Error(w, `{"error":"fetcher not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	short := r.URL.Query().Get("short") == "true"
	logger.Info(fmt.Sprintf("API web_summary: %s (short=%v)", url, short))
	page, err := fetcher.SummarizeURL(r.Context(), url, "", short)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"target_url":  page.TargetURL,
		"current_url": page.CurrentURL,
		"title":       page.Title,
		"summary":     page.Summary,
	})
}

// apiImageFetch handles GET /api/image?url=...
// Returns the raw binary image (proxied).
func apiImageFetch(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, `{"error":"missing url parameter"}`, http.StatusBadRequest)
		return
	}
	if fetcher == nil {
		http.Error(w, `{"error":"fetcher not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	logger.Info(fmt.Sprintf("API image_fetch: %s", url))
	resource, err := fetcher.DownloadResource(r.Context(), url)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	if resource.ContentType != "" {
		w.Header().Set("Content-Type", resource.ContentType)
	}
	if resource.Filename != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", resource.Filename))
	}
	w.Write(resource.Body)
}

// apiBrowserImageFetch handles GET /api/browser-image?url=...
// Uses headless Chrome to bypass bot detection, returns raw binary image.
func apiBrowserImageFetch(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, `{"error":"missing url parameter"}`, http.StatusBadRequest)
		return
	}
	if fetcher == nil {
		http.Error(w, `{"error":"fetcher not initialized"}`, http.StatusServiceUnavailable)
		return
	}
	logger.Info(fmt.Sprintf("API browser_image_fetch: %s", url))
	resource, err := fetcher.BrowserDownloadResource(r.Context(), url)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	if resource.ContentType != "" {
		w.Header().Set("Content-Type", resource.ContentType)
	}
	if resource.Filename != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", resource.Filename))
	}
	w.Write(resource.Body)
}

// apiWebSearch handles GET /api/search?q=...
// Returns search results as JSON.
func apiWebSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, `{"error":"missing q parameter"}`, http.StatusBadRequest)
		return
	}
	if fetcher == nil || !fetcher.HasSearch() {
		http.Error(w, `{"error":"search not configured"}`, http.StatusServiceUnavailable)
		return
	}
	logger.Info(fmt.Sprintf("API web_search: %s", query))
	results, err := fetcher.SearchJSON(r.Context(), query)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query":   query,
		"results": results,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func StartHTTP(opts fetchurl.WebFetcherOptions, mcpOpts MCPServerOptions) {
	opts.ConvertAbsoluteHref = true
	logger = opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	var err error
	if fetcher, err = fetchurl.NewWebFetcher(opts); err != nil {
		logger.Error(fmt.Sprintf("Error creating webfetcher: %v", err))
		return
	}
	if err := fetcher.Start(); err != nil {
		logger.Error(fmt.Sprintf("Error starting webfetcher: %v", err))
		return
	}
	defer fetcher.Stop()

	server := createMCPServer(mcpOpts, fetcher)

	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			return server
		}, nil,
	)

	authWrapper := func(next http.Handler) http.Handler {
		if mcpOpts.MasterKey == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			expected := "Bearer " + mcpOpts.MasterKey
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
				logger.Warn("Unauthorized request", slog.String("remote_addr", r.RemoteAddr), slog.String("path", r.URL.Path))
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	mux := http.NewServeMux()
	mux.Handle("/", authWrapper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello!\n"))
	})))
	mux.Handle("/mcp", authWrapper(handler))
	// REST API endpoints — same functionality as MCP tools, less protocol overhead.
	if mcpOpts.EnableAPI {
		logger.Info("REST API enabled at /api/*")
		mux.Handle("/api/fetch", authWrapper(http.HandlerFunc(apiWebFetch)))
		mux.Handle("/api/summary", authWrapper(http.HandlerFunc(apiWebSummary)))
		mux.Handle("/api/image", authWrapper(http.HandlerFunc(apiImageFetch)))
		mux.Handle("/api/browser-image", authWrapper(http.HandlerFunc(apiBrowserImageFetch)))
		mux.Handle("/api/search", authWrapper(http.HandlerFunc(apiWebSearch)))
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", mcpOpts.Addr, mcpOpts.Port),
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		logger.Info("Received shutdown signal")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error(fmt.Sprintf("HTTP server shutdown error: %v", err))
		}
	}()

	logger.Info(fmt.Sprintf("Starting mcpfurl MCP server on %s:%d", mcpOpts.Addr, mcpOpts.Port))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error(fmt.Sprintf("HTTP server error: %v", err))
	}
	logger.Info("HTTP server stopped")
}

// func verifyToken(ctx context.Context, token string, req *http.Request) (*auth.TokenInfo, error) {
// 	return &auth.TokenInfo{}, nil
// }
