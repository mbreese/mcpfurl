package mcpserver

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
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
	Summary string `json:"content" jsonschema:"The summary of a webpage in Markdown format"`
	Error   string `json:"error,omitempty" jsonschema:"Any error messages"`
}

type WebSearchOutput struct {
	Query           string                  `json:"query" jsonschema:"The query for this search"`
	ResultsMarkdown string                  `json:"markdown,omitempty" jsonschema:"The search results in Markdown format"`
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
}

var fetcher *fetchurl.WebFetcher

// var server *mcp.Server
var logger *slog.Logger

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
			ret += fmt.Sprintf("* [%s](%s) - %s\n", result.Title, result.Link, result.Snippet)
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

	return nil, &WebSummaryOutput{Summary: webpage.ToYaml()}, nil

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
