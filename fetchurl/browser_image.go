package fetchurl

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/fetch"
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
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(w.browserCtx, timeout)
	defer cancel()

	ctx, cancel2 := chromedp.NewContext(ctx)
	defer cancel2()

	// Capture the response body via the Fetch domain's GetResponseBody.
	var mu sync.Mutex
	var respBody []byte
	var respCT string
	var captured bool

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			if e.Response != nil && strings.Contains(e.Response.URL, path.Base(targetURL)) {
				mu.Lock()
				if !captured {
					respCT = e.Response.MimeType
				}
				mu.Unlock()
			}
		case *network.EventLoadingFinished:
			mu.Lock()
			if !captured {
				go func() {
					body, err := network.GetResponseBody(e.RequestID).Do(ctx)
					if err == nil {
						mu.Lock()
						respBody = body
						captured = true
						mu.Unlock()
					}
				}()
			}
			mu.Unlock()
		}
	})

	// Navigate to the URL — headless Chrome handles reCAPTCHA/bot detection.
	if err := chromedp.Run(ctx,
		network.Enable(),
		fetch.Enable(),
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		// Check if we captured data before the error (e.g. image URLs have no body element).
		mu.Lock()
		if captured && len(respBody) > 0 {
			mu.Unlock()
		} else {
			mu.Unlock()
			return nil, fmt.Errorf("browser navigation to %s: %w", targetURL, err)
		}
	}

	// Give a moment for LoadingFinished to fire if it hasn't yet.
	if !captured {
		time.Sleep(2 * time.Second)
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
