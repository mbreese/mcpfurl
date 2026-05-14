package fetchurl

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowserDownloadFile fetches a binary resource using headless Chrome.
// Unlike BrowserDownloadResource, this does not validate that the response
// is an image — it accepts any content type (PDF, ZIP, etc.).
func (w *WebFetcher) BrowserDownloadFile(ctx context.Context, targetURL string) (*DownloadedResource, error) {
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

	// Navigate to the host's root page to establish cookies/pass challenges.
	hostPage := targetURL
	if i := nthIndex(targetURL, '/', 3); i > 0 {
		hostPage = targetURL[:i] + "/"
	}

	log.Info("browser_file_download: navigating to host page", "hostPage", hostPage, "targetURL", targetURL)

	if err := chromedp.Run(ctx,
		chromedp.Navigate(hostPage),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		log.Error("browser_file_download: navigation failed", "hostPage", hostPage, "error", err)
		return nil, fmt.Errorf("browser navigation to %s: %w", hostPage, err)
	}

	var currentURL string
	_ = chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Info("browser_file_download: page loaded", "currentURL", currentURL)

	// From the page context (with cookies), fetch the file via sync XHR.
	log.Info("browser_file_download: fetching file via sync XHR", "targetURL", targetURL)

	var resultJSON string
	if err := chromedp.Run(ctx,
		chromedp.EvaluateAsDevTools(fmt.Sprintf(`
			(function() {
				try {
					var xhr = new XMLHttpRequest();
					xhr.open('GET', %q, false);
					xhr.overrideMimeType('text/plain; charset=x-user-defined');
					xhr.send();
					if (xhr.status < 200 || xhr.status >= 300) {
						return JSON.stringify({error: 'HTTP ' + xhr.status});
					}
					var raw = xhr.responseText;
					var binary = '';
					for (var i = 0; i < raw.length; i++) {
						binary += String.fromCharCode(raw.charCodeAt(i) & 0xff);
					}
					return JSON.stringify({
						data: btoa(binary),
						type: xhr.getResponseHeader('content-type') || '',
						size: raw.length
					});
				} catch(e) {
					return JSON.stringify({error: e.message});
				}
			})()
		`, targetURL), &resultJSON),
	); err != nil {
		log.Error("browser_file_download: EvaluateAsDevTools failed", "error", err)
		return nil, fmt.Errorf("browser fetch of %s: %w", targetURL, err)
	}

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

	if result.Data == "" {
		return nil, fmt.Errorf("browser fetch: empty data (size=%d, type=%s) for %s",
			result.Size, result.Type, targetURL)
	}

	body, err := base64.StdEncoding.DecodeString(result.Data)
	if err != nil {
		return nil, fmt.Errorf("decoding browser fetch response: %w", err)
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("browser fetch: empty decoded body for %s", targetURL)
	}

	filename := path.Base(targetURL)
	if filename == "/" || filename == "." {
		filename = ""
	}

	log.Info("browser_file_download: success", "filename", filename, "bodyLen", len(body), "contentType", result.Type)

	return &DownloadedResource{
		Filename:    filename,
		ContentType: result.Type,
		Body:        body,
	}, nil
}
