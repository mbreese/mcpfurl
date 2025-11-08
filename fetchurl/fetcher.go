package fetchurl

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
)

type WebFetcher struct {
	service *selenium.Service
	wd      selenium.WebDriver
	opts    WebFetcherOptions
	done    bool
	lock    sync.Mutex
	search  SearchEngine
	cache   *SearchCache
}

type WebFetcherOptions struct {
	ChromeDriverPath    string
	WebDriverPort       int
	ConvertAbsoluteHref bool
	UsePandoc           bool
	PageLoadTimeoutSecs int
	MaxDownloadBytes    int
	Logger              *slog.Logger
	SearchEngine        string
	GoogleSearchCx      string
	GoogleSearchKey     string
	SearchCachePath     string
	SearchCacheExpires  time.Duration
}

type FetchedWebPage struct {
	TargetURL  string
	CurrentURL string
	Title      string
	Src        string
}

type FetchedWebPageResult struct {
	Page *FetchedWebPage
	Err  error
}

func NewWebFetcher(opts WebFetcherOptions) (*WebFetcher, error) {
	if opts.WebDriverPort == 0 {
		opts.WebDriverPort = 9515
	}
	if opts.ChromeDriverPath == "" {
		opts.ChromeDriverPath = "/usr/bin/chromedriver"
	}
	if opts.PageLoadTimeoutSecs == 0 {
		opts.PageLoadTimeoutSecs = 10
	}

	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}

	var search SearchEngine
	if opts.SearchEngine == "google_custom" {
		if opts.GoogleSearchCx == "" || opts.GoogleSearchKey == "" {
			return nil, fmt.Errorf("missing Google cx and/or api key values")
		}
		search = NewGoogleCustomSearch(opts.GoogleSearchCx, opts.GoogleSearchKey)
	}

	var cache *SearchCache
	if opts.SearchCachePath != "" {
		searchCache, err := NewSearchCache(opts.SearchCachePath, opts.SearchCacheExpires)
		if err != nil {
			return nil, err
		}
		cache = searchCache
	}

	return &WebFetcher{
		opts:   opts,
		search: search,
		cache:  cache,
	}, nil
}

func (w *WebFetcher) Stop() {
	if w.wd != nil {
		w.wd.Quit()
	}
	if w.service != nil {
		w.service.Stop()
	}
	if w.cache != nil {
		w.cache.Close()
	}

	w.done = true
	w.opts.Logger.Info("Stopped fetcher service / webdriver")
}

func (w *WebFetcher) Start() error {
	if w.done {
		return fmt.Errorf("service already stopped")
	}
	w.lock.Lock()

	// Set Chrome options (headless)
	caps := selenium.Capabilities{
		"browserName": "chrome",
	}

	caps.AddChrome(chrome.Capabilities{
		Args: []string{
			"--headless",
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--disable-gpu",
		},
	})

	// Try to connect to an exiting service first
	var wd selenium.WebDriver
	var err error

	wd, err = selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", w.opts.WebDriverPort))
	if err != nil {
		w.opts.Logger.Debug(fmt.Sprintf("Starting new webdriver service: http://localhost:%d/wd/hub", w.opts.WebDriverPort))
		// If we are here, we need to start the service ourselves...
		service, err := selenium.NewChromeDriverService(w.opts.ChromeDriverPath, w.opts.WebDriverPort)
		if err != nil {
			w.lock.Unlock()
			return fmt.Errorf("error starting ChromeDriver server: %v", err)
		}
		w.service = service
		wd, err = selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", w.opts.WebDriverPort))
		if err != nil {
			w.lock.Unlock()
			return fmt.Errorf("failed to open session: %v", err)
		}
		w.opts.Logger.Debug("Starting selenium remote created")
	} else {
		w.opts.Logger.Debug("Starting selenium remote created")
		w.opts.Logger.Debug(fmt.Sprintf("Connected to existing webdriver instance: http://localhost:%d/wd/hub", w.opts.WebDriverPort))
	}

	wd.SetPageLoadTimeout(time.Duration(w.opts.PageLoadTimeoutSecs) * time.Second)
	w.wd = wd
	w.lock.Unlock()

	return nil
}

// This function wraps the Main FetchURL outputs into one object that's suitable
// for use in a channel.
//
// This is helpful when there could be multiple calls to the same WebFetcher, which
// is protected by the mutex lock.

func (w *WebFetcher) FetchURLResult(ctx context.Context, targetURL string, selector string) *FetchedWebPageResult {
	page, err := w.FetchURL(ctx, targetURL, selector)
	return &FetchedWebPageResult{page, err}
}

func (w *WebFetcher) FetchURL(ctx context.Context, targetURL string, selector string) (*FetchedWebPage, error) {
	// hold onto the lock for this instance. If we need to make
	// more than one request at a time, it will require more than
	// one Webdriver session (and port) and thus a WebFetcher.
	//
	// So, each WebFetcher can only make one request at a time.

	w.lock.Lock()
	defer w.lock.Unlock()

	var resultErr error
	var result *FetchedWebPage

	go func() {
		select {
		case <-ctx.Done():
			fmt.Println("Context cancelled, stopping Selenium operation.")
			w.lock.Unlock()
			resultErr = fmt.Errorf("context timed out or cancelled")
			result = nil
			return
		default:
			w.opts.Logger.Info(fmt.Sprintf("Fetching URL: %s", targetURL))

			if w.done {
				resultErr = fmt.Errorf("service already stopped")
				result = nil
				return
			}
			// Navigate to a URL
			if err := w.wd.Get(targetURL); err != nil {
				resultErr = fmt.Errorf("failed to load page: %v", err)
				result = nil
				return
			}

			// Wait for JS to execute or page to load
			w.opts.Logger.Debug("Waiting for page to load")
			err := w.wd.WaitWithTimeout(func(driver selenium.WebDriver) (bool, error) {
				result, err := driver.ExecuteScript("return document.readyState;", nil)
				if err != nil {
					return false, err
				}
				if result == "complete" {
					return true, nil
				}
				return false, nil
			}, time.Duration(w.opts.PageLoadTimeoutSecs)*time.Second)
			if err != nil {
				resultErr = fmt.Errorf("failed waiting for page to load: %v", err)
				result = nil
				return
			}
			w.opts.Logger.Debug("Page loaded")

			// if we want to convert Hrefs to absolute paths, run this script
			if w.opts.ConvertAbsoluteHref {
				w.opts.Logger.Debug("Converting a-href/img-src to absolute")
				_, err := w.wd.ExecuteScript(`
		const links = document.body.querySelectorAll('a');
		const images = document.body.querySelectorAll('img');
		links.forEach(link => {
			const abshref = link.href;
			link.setAttribute('href', abshref);
			});
		images.forEach(img => {
			const abssrc = img.src;
			img.setAttribute('src', abssrc);
			});
    	`, nil)
				if err != nil {
					resultErr = fmt.Errorf("failed to execute JS: %v", err)
					result = nil
					return
				}
			}
			// fmt.Printf("Page title (via JS): %v\n", result)

			title, err := w.wd.Title()
			if err != nil {
				resultErr = fmt.Errorf("failed to get title: %v", err)
				result = nil
				return
			}

			// Retrieve body text
			var body selenium.WebElement
			if selector == "" || strings.ToLower(selector) == "body" {
				body, err = w.wd.FindElement(selenium.ByTagName, "body")
				if err != nil {
					resultErr = fmt.Errorf("failed to find body: %v", err)
					result = nil
					return
				}
			} else if selector[0] == '#' {
				body, err = w.wd.FindElement(selenium.ByID, selector[1:])
				if err != nil {
					resultErr = fmt.Errorf("failed to find %s: %v", selector, err)
					result = nil
					return
				}
			} else if selector[0] == '.' {
				elements, err := w.wd.FindElements(selenium.ByClassName, selector[1:])
				if err != nil || len(elements) == 0 {
					resultErr = fmt.Errorf("failed to find %s: %v", selector, err)
					result = nil
					return
				}
				body = elements[0]
			}

			htmlSrc, err := body.GetAttribute("innerHTML")
			if err != nil {
				resultErr = fmt.Errorf("failed to get body html: %v", err)
				result = nil
				return
			}

			currentURL, err := w.wd.CurrentURL()
			if err != nil {
				resultErr = fmt.Errorf("failed to get currentURL: %v", err)
				result = nil
				return
			}
			w.opts.Logger.Debug("Done")
			result = &FetchedWebPage{Title: title, TargetURL: targetURL, CurrentURL: currentURL, Src: htmlSrc}
			return
		}
	}()

	return result, resultErr

}

func (w *WebFetcher) SearchJSON(ctx context.Context, query string) ([]SearchResult, error) {
	if w.search == nil {
		return nil, fmt.Errorf("search engine is not configured")
	}

	if w.cache != nil {
		if results, ok, err := w.cache.Get(ctx, query); err == nil && ok {
			w.opts.Logger.Debug("Search cache hit")
			return results, nil
		} else if err != nil {
			w.opts.Logger.Warn("search cache get failed", "error", err)
		}
	}

	results, err := w.search.SearchJSON(ctx, query)
	if err != nil {
		return nil, err
	}

	if w.cache != nil {
		if err := w.cache.Put(ctx, query, results); err != nil {
			w.opts.Logger.Warn("search cache put failed", "error", err)
		}
	}

	return results, nil
}
