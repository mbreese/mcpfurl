package fetchurl

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"
)

// Crawl visits pages starting from startURL up to maxDepth (root = 0) and maxPages.
// If sameHostOnly is true, only links on the starting host and under the starting path are followed.
// selector is applied to all fetched pages (use "" for default/selector inference).
func (w *WebFetcher) Crawl(ctx context.Context, startURL string, maxDepth int, maxPages int, sameHostOnly bool, selector string) ([]*FetchedWebPage, error) {
	if maxPages <= 0 {
		return nil, fmt.Errorf("maxPages must be > 0")
	}
	if maxDepth < 0 {
		return nil, fmt.Errorf("maxDepth must be >= 0")
	}

	start, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("invalid start url: %w", err)
	}

	normalize := func(base *url.URL, raw string) (string, error) {
		u, err := base.Parse(strings.TrimSpace(raw))
		if err != nil {
			return "", err
		}
		if u.Scheme == "" {
			u.Scheme = base.Scheme
		}
		u.Fragment = ""
		u.Host = strings.ToLower(u.Host)
		u.Scheme = strings.ToLower(u.Scheme)
		if u.Path == "" {
			u.Path = "/"
		}
		u.Path = path.Clean(u.Path)
		if u.Path == "." {
			u.Path = "/"
		}
		return u.String(), nil
	}

	startNorm, err := normalize(start, start.String())
	if err != nil {
		return nil, err
	}
	allowedPath := ""
	if sameHostOnly {
		allowedPath = strings.TrimSuffix(path.Clean(start.Path), "/")
		if allowedPath != "" && allowedPath != "/" {
			allowedPath = allowedPath + "/"
		} else {
			allowedPath = "/"
		}
	}

	type qItem struct {
		url   string
		depth int
	}

	queue := []qItem{{url: startNorm, depth: 0}}
	visited := make(map[string]bool)
	var pages []*FetchedWebPage

	for len(queue) > 0 && len(pages) < maxPages {
		if err := ctx.Err(); err != nil {
			return pages, err
		}
		item := queue[0]
		queue = queue[1:]
		if visited[item.url] {
			continue
		}
		if item.depth > maxDepth {
			continue
		}

		page, err := w.FetchURL(ctx, item.url, selector)
		if err != nil {
			w.opts.Logger.Warn("crawl fetch failed", "url", item.url, "error", err)
			continue
		}
		visited[item.url] = true
		pages = append(pages, page)

		allowedHost := ""
		if sameHostOnly {
			allowedHost = start.Host
		}

		links, err := extractLinks(page.Src, item.url, allowedHost, allowedPath)
		if err != nil {
			w.opts.Logger.Warn("crawl link extraction failed", "url", item.url, "error", err)
			continue
		}

		base, _ := url.Parse(page.CurrentURL)
		for _, raw := range links {
			if len(queue)+len(pages) >= maxPages {
				break
			}
			norm, err := normalize(base, raw)
			if err != nil {
				continue
			}
			if visited[norm] {
				continue
			}
			queue = append(queue, qItem{url: norm, depth: item.depth + 1})
		}
	}

	return pages, nil
}

func extractLinks(htmlSrc string, baseURL string, allowedHost string, allowedPath string) ([]string, error) {
	doc, err := html.Parse(strings.NewReader(htmlSrc))
	if err != nil {
		return nil, err
	}

	var links []string
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if strings.ToLower(attr.Key) == "href" && attr.Val != "" {
					link := strings.TrimSpace(attr.Val)
					if shouldKeepLink(link, baseURL, allowedHost, allowedPath) {
						links = append(links, link)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(doc)
	return links, nil
}

func shouldKeepLink(raw string, baseURL string, allowedHost string, allowedPath string) bool {
	if raw == "" {
		return false
	}
	if strings.HasPrefix(raw, "mailto:") || strings.HasPrefix(raw, "javascript:") || strings.HasPrefix(raw, "tel:") {
		return false
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	u, err := base.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if allowedHost != "" {
		if u.Host != "" && strings.ToLower(u.Host) != strings.ToLower(allowedHost) {
			return false
		}
	}
	if allowedPath != "" {
		p := path.Clean(u.Path)
		if p != "/" && strings.HasSuffix(u.Path, "/") {
			p = p + "/"
		}
		normalized := p
		if normalized == "" {
			normalized = "/"
		}
		if !strings.HasPrefix(normalized, allowedPath) {
			return false
		}
	}
	return true
}
