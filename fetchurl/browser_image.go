package fetchurl

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// BrowserDownloadResource fetches a binary resource using headless Chrome.
// This bypasses bot detection (reCAPTCHA, Cloudflare) that blocks plain HTTP requests.
// The browser navigates to the URL (handling any challenges), then uses the
// fetch API from within the browser context to download the resource with
// the browser's established session cookies.
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

	// Navigate to the URL — headless Chrome handles reCAPTCHA/bot detection.
	if err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		return nil, fmt.Errorf("browser navigation to %s: %w", targetURL, err)
	}

	// Use the browser's fetch API to download the resource with session cookies.
	// This works because the browser has already passed any bot challenges.
	// Unmarshal into a map so we don't fight CDP's JSON wrapping of awaited promises.
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
					for (let i = 0; i < bytes.byteLength; i++) {
						binary += String.fromCharCode(bytes[i]);
					}
					return {
						data: btoa(binary),
						type: resp.headers.get('content-type') || '',
						status: resp.status
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

	// Validate it's actually an image, not an HTML error page.
	detected := http.DetectContentType(body)
	if len(body) > 0 && !isImageContentType(detected) {
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
