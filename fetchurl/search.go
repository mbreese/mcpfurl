package fetchurl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type SearchEngineConfig struct {
	URL    string
	Params map[string]string
}

type GenericSearchEngine struct {
	config *SearchEngineConfig
}

type SearchResult struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
	// ThumbnailLink   string items[i]["pagemap"]["cse_thumbnail"][0]["src"]
	// ThumbnailWidth  string items[i]["pagemap"]["cse_thumbnail"][0]["width"]
	// ThumbnailHeight string items[i]["pagemap"]["cse_thumbnail"][0]["height"]
}

// type SearchResults struct {
// 	Results []SearchResult `json:"results"`
// }

type SearchEngine interface {
	SearchJSON(ctx context.Context, query string) ([]SearchResult, error)
}

func NewGoogleCustomSearch(cx string, key string) *GenericSearchEngine {
	m := make(map[string]string)
	m["cx"] = cx
	m["key"] = key
	m["queryKey"] = "q"
	cfg := SearchEngineConfig{
		URL:    "https://customsearch.googleapis.com/customsearch/v1",
		Params: m,
	}
	return &GenericSearchEngine{config: &cfg}
}

func (s *GenericSearchEngine) SearchJSON(ctx context.Context, query string) ([]SearchResult, error) {

	urlQuery := fmt.Sprintf("%s?%s=%s", s.config.URL, s.config.Params["queryKey"], url.QueryEscape(query))

	for k, v := range s.config.Params {
		urlQuery += fmt.Sprintf("&%s=%s", url.QueryEscape(k), url.QueryEscape(v))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlQuery, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cgmcp-webfetch-search/0.1")

	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("api status: %s", resp.Status)
	}

	var data map[string]interface{}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var results []SearchResult
	items, ok := data["items"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unable to process JSON result")
	}
	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if kind, ok := obj["kind"].(string); !ok || kind != "customsearch#result" {
			continue
		}

		title, _ := obj["title"].(string)
		link, _ := obj["link"].(string)
		snippet, _ := obj["snippet"].(string)

		results = append(results, SearchResult{Title: title, Link: link, Snippet: snippet})

	}

	return results, nil

}
