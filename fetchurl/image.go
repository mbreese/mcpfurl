package fetchurl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
)

const DefaultMaxDownloadBytes = 16 * 1024 * 1024

// DownloadedResource represents a binary payload fetched via HTTP.
type DownloadedResource struct {
	Filename    string
	ContentType string
	Body        []byte
}

// DownloadResource retrieves the remote resource at the given URL and returns its metadata and body.
func (w *WebFetcher) DownloadResource(ctx context.Context, targetURL string) (*DownloadedResource, error) {
	if targetURL == "" {
		return nil, fmt.Errorf("missing URL")
	}
	if w == nil {
		return nil, fmt.Errorf("web fetcher is not initialized")
	}
	if err := ensureURLAllowed(targetURL, w.opts.AllowedURLGlobs, w.opts.BlockedURLGlobs); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error building request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error downloading %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code %d downloading %s", resp.StatusCode, targetURL)
	}

	limit := w.opts.MaxDownloadBytes
	var body []byte
	if limit > 0 {
		body, err = io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
		if err != nil {
			return nil, fmt.Errorf("error reading response body: %w", err)
		}
		if len(body) > limit {
			return nil, fmt.Errorf("resource exceeds %d bytes limit", limit)
		}
	} else {
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response body: %w", err)
		}
	}

	filename := path.Base(resp.Request.URL.Path)
	if filename == "/" || filename == "." {
		filename = ""
	}

	return &DownloadedResource{
		Filename:    filename,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        body,
	}, nil
}
