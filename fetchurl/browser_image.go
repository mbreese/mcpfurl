package fetchurl

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
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
	log := slog.Default()

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
	hostPage := targetURL
	if j := strings.Index(targetURL, "/bin/"); j > 0 {
		// PMC URL like .../articles/PMC1234567/bin/fig.jpg → use article page
		hostPage = targetURL[:j] + "/"
	} else if i := nthIndex(targetURL, '/', 3); i > 0 {
		hostPage = targetURL[:i] + "/"
	}

	log.Info("browser_image: navigating to host page", "hostPage", hostPage, "targetURL", targetURL)

	if err := chromedp.Run(ctx,
		chromedp.Navigate(hostPage),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		log.Error("browser_image: navigation failed", "hostPage", hostPage, "error", err)
		return nil, fmt.Errorf("browser navigation to %s: %w", hostPage, err)
	}

	// Log the current URL after navigation (may have been redirected).
	var currentURL string
	_ = chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Info("browser_image: page loaded", "currentURL", currentURL)

	// Step 2: From the page context (with cookies), fetch the image via sync XHR.
	log.Info("browser_image: fetching image via sync XHR", "targetURL", targetURL)

	var resultJSON string
	if err := chromedp.Run(ctx,
		chromedp.EvaluateAsDevTools(fmt.Sprintf(`
			(function() {
				try {
					var xhr = new XMLHttpRequest();
					xhr.open('GET', %q, false);
					xhr.responseType = 'arraybuffer';
					xhr.send();
					if (xhr.status < 200 || xhr.status >= 300) {
						return JSON.stringify({error: 'HTTP ' + xhr.status});
					}
					var bytes = new Uint8Array(xhr.response);
					var binary = '';
					var chunkSize = 8192;
					for (var i = 0; i < bytes.length; i += chunkSize) {
						binary += String.fromCharCode.apply(null, bytes.subarray(i, i + chunkSize));
					}
					return JSON.stringify({
						data: btoa(binary),
						type: xhr.getResponseHeader('content-type') || '',
						size: bytes.length
					});
				} catch(e) {
					return JSON.stringify({error: e.message});
				}
			})()
		`, targetURL), &resultJSON),
	); err != nil {
		log.Error("browser_image: EvaluateAsDevTools failed", "error", err)
		return nil, fmt.Errorf("browser fetch of %s: %w", targetURL, err)
	}

	log.Info("browser_image: XHR result", "resultLen", len(resultJSON), "result", resultJSON)

	var result struct {
		Data  string `json:"data"`
		Type  string `json:"type"`
		Size  int    `json:"size"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return nil, fmt.Errorf("parsing browser fetch result: %w (%s)", err, resultJSON)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("browser fetch: %s", result.Error)
	}
	dataStr := result.Data
	contentType := result.Type

	log.Info("browser_image: data received", "dataLen", len(dataStr), "size", result.Size, "contentType", contentType)

	if dataStr == "" {
		return nil, fmt.Errorf("browser fetch: empty data (size=%d, type=%s) for %s",
			result.Size, contentType, targetURL)
	}

	body, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return nil, fmt.Errorf("decoding browser fetch response: %w", err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("browser fetch: empty decoded body for %s", targetURL)
	}

	// Validate it's actually an image, not an HTML error page.
	detected := http.DetectContentType(body)
	log.Info("browser_image: content type check", "detected", detected, "bodyLen", len(body))
	if !isImageContentType(detected) {
		return nil, fmt.Errorf("browser fetch returned %s, not an image", detected)
	}

	filename := path.Base(targetURL)
	if filename == "/" || filename == "." {
		filename = ""
	}

	log.Info("browser_image: success", "filename", filename, "bodyLen", len(body), "contentType", contentType)

	return &DownloadedResource{
		Filename:    filename,
		ContentType: contentType,
		Body:        body,
	}, nil
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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
