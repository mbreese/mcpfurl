package fetchurl

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// BrowserDownloadFile fetches a binary resource using headless Chrome.
// Unlike BrowserDownloadResource, this does not validate that the response
// is an image — it accepts any content type (PDF, ZIP, etc.).
//
// If warmupURL is non-empty, Chrome navigates there first to establish
// cookies and pass bot-detection challenges before fetching the target file.
// Otherwise it navigates to the host root of targetURL.
func (w *WebFetcher) BrowserDownloadFile(ctx context.Context, targetURL, warmupURL string) (*DownloadedResource, error) {
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

	tabCtx, tabCancel := chromedp.NewContext(w.browserCtx)
	defer tabCancel()
	ctx, cancel := context.WithTimeout(tabCtx, timeout)
	defer cancel()

	// Inject anti-detection scripts before any navigation.
	if err := chromedp.Run(ctx, stealthSetup()); err != nil {
		return nil, fmt.Errorf("stealth setup: %w", err)
	}

	// Navigate to a warmup page to establish cookies/pass challenges.
	// Use the provided warmup URL, or fall back to the host root.
	navPage := warmupURL
	if navPage == "" {
		navPage = targetURL
		if i := nthIndex(targetURL, '/', 3); i > 0 {
			navPage = targetURL[:i] + "/"
		}
	}

	log.Info("browser_file_download: navigating to warmup page", "navPage", navPage, "targetURL", targetURL)

	if err := chromedp.Run(ctx,
		chromedp.Navigate(navPage),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		log.Error("browser_file_download: navigation failed", "navPage", navPage, "error", err)
		return nil, fmt.Errorf("browser navigation to %s: %w", navPage, err)
	}

	var currentURL string
	_ = chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Info("browser_file_download: page loaded", "currentURL", currentURL)

	// Wait for JS challenges (Cloudflare, CAPTCHAs, proof-of-work) to complete.
	// These typically run JavaScript that eventually redirects to the real page.
	// Poll the URL every 2 seconds for up to 30 seconds, stopping early once
	// the URL has been stable for 4 seconds.
	stableURL := currentURL
	stableSince := time.Now()
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			log.Warn("browser_file_download: context cancelled while waiting for JS challenges")
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}

		var newURL string
		if err := chromedp.Run(ctx, chromedp.Location(&newURL)); err != nil {
			break
		}

		if newURL != stableURL {
			log.Info("browser_file_download: URL changed (JS redirect)", "from", stableURL, "to", newURL)
			stableURL = newURL
			stableSince = time.Now()
		} else if time.Since(stableSince) >= 4*time.Second {
			log.Info("browser_file_download: URL stable, proceeding", "url", stableURL)
			break
		}
	}

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

	// If the response is HTML, we likely hit a challenge page or landing page
	// rather than the actual file.
	if strings.HasPrefix(result.Type, "text/html") {
		return nil, fmt.Errorf("browser fetch returned HTML instead of a file (content-type: %s) for %s — the page may require manual interaction", result.Type, targetURL)
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
