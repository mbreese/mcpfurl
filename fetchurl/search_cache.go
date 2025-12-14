package fetchurl

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type CacheDB struct {
	db  *sql.DB
	ttl time.Duration
}

func ConvertTTLToDuration(ttl string) (time.Duration, error) {
	normalized := strings.TrimSpace(strings.ToLower(ttl))
	if normalized == "" {
		return 0, fmt.Errorf("ttl cannot be empty")
	}

	convert := func(value, suffix string) (time.Duration, error) {
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return 0, fmt.Errorf("invalid ttl value: %s", ttl)
		}
		return time.ParseDuration(fmt.Sprintf("%s%s", value, suffix))
	}

	switch {
	case strings.HasSuffix(normalized, "hours"):
		value := strings.TrimSuffix(normalized, "hours")
		return convert(value, "h")
	case strings.HasSuffix(normalized, "h"):
		return time.ParseDuration(normalized)
	case strings.HasSuffix(normalized, "min"):
		value := strings.TrimSuffix(normalized, "min")
		return convert(value, "m")
	case strings.HasSuffix(normalized, "m"):
		return time.ParseDuration(normalized)
	case strings.HasSuffix(normalized, "secs"):
		value := strings.TrimSuffix(normalized, "secs")
		return convert(value, "s")
	case strings.HasSuffix(normalized, "s"):
		return time.ParseDuration(normalized)
	case strings.HasSuffix(normalized, "days"):
		value := strings.TrimSuffix(normalized, "days")
		days, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid ttl value: %s", ttl)
		}
		return time.Duration(days * 24 * float64(time.Hour)), nil
	case strings.HasSuffix(normalized, "d"):
		value := strings.TrimSuffix(normalized, "d")
		days, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid ttl value: %s", ttl)
		}
		return time.Duration(days * 24 * float64(time.Hour)), nil
	default:
		if d, err := time.ParseDuration(normalized); err == nil {
			return d, nil
		}
		return 0, fmt.Errorf("unsupported ttl format: %s", ttl)
	}
}

func NewCacheDB(path string, ttl time.Duration) (*CacheDB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	schema := `
CREATE TABLE IF NOT EXISTS search_cache (
	query TEXT PRIMARY KEY,
	result_json BLOB NOT NULL,
	fetched_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_search_cache_fetched ON search_cache(fetched_at);
CREATE TABLE IF NOT EXISTS web_cache (
	url TEXT NOT NULL,
	selector TEXT NOT NULL DEFAULT '',
	content BLOB NOT NULL,
	fetched_at TIMESTAMP NOT NULL,
	PRIMARY KEY (url, selector)
);
CREATE INDEX IF NOT EXISTS idx_web_cache_fetched ON web_cache(fetched_at);`

	if _, execErr := db.Exec(schema); execErr != nil {
		db.Close()
		return nil, fmt.Errorf("initializing cache database: %w", execErr)
	}

	c := &CacheDB{db: db, ttl: ttl}

	if err := c.Cleanup(); err != nil {
		return nil, err
	}
	return c, nil

}

func (c *CacheDB) Cleanup() error {
	duration := fmt.Sprintf("-%d seconds", int(c.ttl.Seconds()))
	if _, err := c.db.Exec(`DELETE FROM search_cache WHERE fetched_at < datetime('now', ?)`, duration); err != nil {
		c.db.Close()
		return fmt.Errorf("cleaning search cache: %w", err)
	}
	if _, err := c.db.Exec(`DELETE FROM web_cache WHERE fetched_at < datetime('now', ?)`, duration); err != nil {
		c.db.Close()
		return fmt.Errorf("cleaning web cache: %w", err)
	}
	return nil

}

func (c *CacheDB) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *CacheDB) Get(ctx context.Context, query string) ([]SearchResult, bool, error) {
	if c == nil || c.db == nil {
		return nil, false, fmt.Errorf("cache not initialized")
	}

	var payload []byte
	var fetched time.Time

	err := c.db.QueryRowContext(ctx, `SELECT result_json, fetched_at FROM search_cache WHERE query = ?`, query).
		Scan(&payload, &fetched)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	if time.Since(fetched) > c.ttl {
		return nil, false, nil
	}

	var results []SearchResult
	if err := json.Unmarshal(payload, &results); err != nil {
		return nil, false, err
	}

	return results, true, nil
}

func (c *CacheDB) Put(ctx context.Context, query string, results []SearchResult) error {
	if c == nil || c.db == nil {
		return fmt.Errorf("cache not initialized")
	}

	payload, err := json.Marshal(results)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, `
INSERT INTO search_cache (query, result_json, fetched_at)
VALUES (?, ?, ?)
ON CONFLICT(query) DO UPDATE SET
	result_json = excluded.result_json,
	fetched_at = excluded.fetched_at
`, query, payload, time.Now().UTC())
	return err
}

func (c *CacheDB) GetWebPage(ctx context.Context, url string, selector string) (*FetchedWebPage, bool, error) {
	if c == nil || c.db == nil {
		return nil, false, fmt.Errorf("cache not initialized")
	}

	var payload []byte
	var fetched time.Time

	err := c.db.QueryRowContext(ctx, `SELECT content, fetched_at FROM web_cache WHERE url = ? AND selector = ?`, url, selector).
		Scan(&payload, &fetched)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	if time.Since(fetched) > c.ttl {
		return nil, false, nil
	}

	var page FetchedWebPage
	if err := json.Unmarshal(payload, &page); err != nil {
		return nil, false, err
	}

	return &page, true, nil
}

func (c *CacheDB) PutWebPage(ctx context.Context, url string, selector string, page *FetchedWebPage) error {
	if c == nil || c.db == nil {
		return fmt.Errorf("cache not initialized")
	}
	if page == nil {
		return fmt.Errorf("page cannot be nil")
	}

	payload, err := json.Marshal(page)
	if err != nil {
		return err
	}

	_, err = c.db.ExecContext(ctx, `
INSERT INTO web_cache (url, selector, content, fetched_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(url, selector) DO UPDATE SET
	content = excluded.content,
	fetched_at = excluded.fetched_at
`, url, selector, payload, time.Now().UTC())
	return err
}
