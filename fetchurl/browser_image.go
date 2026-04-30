package fetchurl

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// BrowserDownloadResource fetches a binary resource using headless Chrome.
// This bypasses bot detection (reCAPTCHA, Cloudflare) that blocks plain HTTP requests.
// Uses the Fetch domain to intercept the network response and capture the body
// directly, avoiding issues with JS-based re-fetching of image URLs.
func (w *WebFetcher) BrowserDownloadResource(ctx context.Context, targetURL string) (*DownloadedResource, error) {
	if targetURL == "" {
		return nil, fmt.Errorf("missing URL")
	}

	if allowed, err := ensureURLAllowed(targetURL, w.opts.AllowedURLGlobs, w.opts.DenyURLGlobs); err != nil {
		return nil, err
	} else if !allowed {
		return nil, err
	}

	timeout := time.Duration(w.opts.PageLoadTimeoutSecs) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(w.browserCtx, timeout)
	defer cancel()

	ctx, cancel2 := chromedp.NewContext(ctx)
	defer cancel2()

	// Capture the response metadata and body via network events.
	var mu sync.Mutex
	var respBody []byte
	var respCT string
	var requestID network.RequestID
	done := make(chan struct{})

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			// Match the main document request (not sub-resources).
			if e.Type == network.ResourceTypeDocument || e.Type == network.ResourceTypeImage {
				mu.Lock()
				if requestID == "" {
					requestID = e.RequestID
					if e.Response != nil {
						respCT = e.Response.MimeType
					}
				}
				mu.Unlock()
			}
		case *network.EventLoadingFinished:
			mu.Lock()
			rid := requestID
			mu.Unlock()
			if rid != "" && e.RequestID == rid {
				go func() {
					body, err := network.GetResponseBody(e.RequestID).Do(ctx)
					mu.Lock()
					if err == nil {
						respBody = body
					}
					mu.Unlock()
					close(done)
				}()
			}
		}
	})

	// Navigate to the URL — headless Chrome handles bot detection.
	// Don't wait for DOM — image URLs have no DOM.
	if err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.Navigate(targetURL),
	); err != nil {
		return nil, fmt.Errorf("browser navigation to %s: %w", targetURL, err)
	}

	// Wait for the response body to be captured or timeout.
	select {
	case <-done:
	case <-ctx.Done():
		return nil, fmt.Errorf("browser fetch timed out for %s", targetURL)
	}

	mu.Lock()
	body := respBody
	contentType := respCT
	mu.Unlock()

	if len(body) == 0 {
		return nil, fmt.Errorf("browser fetch: no response body captured for %s", targetURL)
	}

	// Validate it's actually an image, not an HTML error page.
	detected := http.DetectContentType(body)
	if !isImageContentType(detected) {
		return nil, fmt.Errorf("browser fetch returned %s, not an image", detected)
	}

	filename := path.Base(targetURL)
	if filename == "/" || filename == "." {
		filename = ""
	}

	return &DownloadedResource{
		Filename:    filename,
		ContentType: contentType,
		Body:        body,
	}, nil
}

func isImageContentType(ct string) bool {
	return len(ct) >= 6 && ct[:6] == "image/"
}
