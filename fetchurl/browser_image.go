package fetchurl

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowserDownloadResource fetches a binary resource using headless Chrome.
// This bypasses bot detection (reCAPTCHA, Cloudflare) that blocks plain HTTP requests.
//
// Strategy: first navigate to the target host to establish session cookies and
// pass any bot challenges, then use XMLHttpRequest (synchronous-capable) from
// that page context to download the actual resource.
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

	// Step 1: Navigate to the host's root page to establish cookies/pass challenges.
	// We can't navigate to the image URL directly because raw images have no DOM.
	// Parse the origin from the target URL.
	hostPage := targetURL
	if j := strings.Index(targetURL, "/bin/"); j > 0 {
		// PMC URL like .../articles/PMC1234567/bin/fig.jpg → use article page
		hostPage = targetURL[:j] + "/"
	} else if i := nthIndex(targetURL, '/', 3); i > 0 {
		hostPage = targetURL[:i] + "/"
	}

	if err := chromedp.Run(ctx,
		chromedp.Navigate(hostPage),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		return nil, fmt.Errorf("browser navigation to %s: %w", hostPage, err)
	}

	// Step 2: From the page context (with cookies), fetch the image via XHR.
	var result map[string]interface{}
	if err := chromedp.Run(ctx,
		chromedp.EvaluateAsDevTools(fmt.Sprintf(`
			(async () => {
				try {
					const resp = await fetch(%q, {credentials: 'include'});
					if (!resp.ok) return {error: 'HTTP ' + resp.status};
					const buf = await resp.arrayBuffer();
					const bytes = new Uint8Array(buf);
					let binary = '';
					const chunkSize = 8192;
					for (let i = 0; i < bytes.length; i += chunkSize) {
						binary += String.fromCharCode.apply(null, bytes.subarray(i, i + chunkSize));
					}
					return {
						data: btoa(binary),
						type: resp.headers.get('content-type') || '',
						size: bytes.length
					};
				} catch(e) {
					return {error: e.message};
				}
			})()
		`, targetURL), &result),
	); err != nil {
		return nil, fmt.Errorf("browser fetch of %s: %w", targetURL, err)
	}

	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("browser fetch: %s", errMsg)
	}
	dataStr, _ := result["data"].(string)
	contentType, _ := result["type"].(string)

	body, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return nil, fmt.Errorf("decoding browser fetch response: %w", err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("browser fetch: empty response body for %s", targetURL)
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

// nthIndex returns the index of the nth occurrence of sep in s, or -1.
// If sep is a string, it finds the first occurrence of that substring.
func nthIndex(s string, ch byte, n int) int {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			if count == n {
				return i
			}
			count++
		}
	}
	return -1
}
