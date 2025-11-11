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
	AllowedURLGlobs     []string
	BlockedURLGlobs     []string
	// WebDriverLogging    string
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
	var cache *SearchCache

	if opts.SearchEngine == "google_custom" {
		if opts.GoogleSearchCx != "" && opts.GoogleSearchKey != "" {
			search = NewGoogleCustomSearch(opts.GoogleSearchCx, opts.GoogleSearchKey)
		} else {
			opts.Logger.Info("missing Google cx and/or api key values, disabling search")
		}

		if search != nil {
			if opts.SearchCachePath != "" {
				searchCache, err := NewSearchCache(opts.SearchCachePath, opts.SearchCacheExpires)
				if err != nil {
					return nil, err
				}
				cache = searchCache
			}
		}
	} else {
		opts.Logger.Info("No valid search_engine configured.")
	}

	if len(opts.AllowedURLGlobs) > 0 {
		opts.AllowedURLGlobs = append([]string(nil), opts.AllowedURLGlobs...)
	}
	if len(opts.BlockedURLGlobs) > 0 {
		opts.BlockedURLGlobs = append([]string(nil), opts.BlockedURLGlobs...)
	}

	return &WebFetcher{
		opts:   opts,
		search: search,
		cache:  cache,
	}, nil
}

func (w *WebFetcher) HasSearch() bool {
	return w.search != nil
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

	args := []string{
		"--headless",
		"--disable-dev-shm-usage",
		"--disable-gpu",
		"--no-sandbox",
	}

	// if w.opts.WebDriverLogging != "" {
	// 	args = append(args, fmt.Sprintf("--log-path=%s", w.opts.WebDriverLogging))
	// 	args = append(args, "--log-level=DEBUG")
	// }

	w.opts.Logger.Debug(fmt.Sprintf("%v", args))

	caps.AddChrome(chrome.Capabilities{
		Args: args,
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

func (w *WebFetcher) FetchURL(ctx context.Context, targetURL string, selector string) (*FetchedWebPage, error) {
	// hold onto the lock for this instance. If we need to make
	// more than one request at a time, it will require more than
	// one Webdriver session (and port) and thus a WebFetcher.
	//
	// So, each WebFetcher can only make one request at a time.

	w.lock.Lock()
	defer w.lock.Unlock()

	if err := ensureURLAllowed(targetURL, w.opts.AllowedURLGlobs, w.opts.BlockedURLGlobs); err != nil {
		return nil, err
	}

	type tmpResult struct {
		webPage   *FetchedWebPage
		resultErr error
	}

	c1 := make(chan tmpResult)

	go func() {
		select {
		case <-ctx.Done():
			fmt.Println("Context cancelled, stopping Selenium operation.")
			w.lock.Unlock()
			c1 <- tmpResult{nil, fmt.Errorf("context timed out or cancelled")}
			return
		default:
			w.opts.Logger.Info(fmt.Sprintf("Fetching URL: %s", targetURL))

			if w.done {
				c1 <- tmpResult{nil, fmt.Errorf("service already stopped")}
				return
			}
			// Navigate to a URL
			if err := w.wd.Get(targetURL); err != nil {
				fmt.Printf("Error: %v\n\n", err)
				c1 <- tmpResult{nil, fmt.Errorf("failed to load page: %v", err)}
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
				c1 <- tmpResult{nil, fmt.Errorf("failed waiting for page to load: %v", err)}
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
					c1 <- tmpResult{nil, fmt.Errorf("failed to execute JS: %v", err)}
					return
				}
			}
			// fmt.Printf("Page title (via JS): %v\n", result)

			title, err := w.wd.Title()
			if err != nil {
				c1 <- tmpResult{nil, fmt.Errorf("failed to get title: %v", err)}
				return
			}

			// Retrieve body text
			var body selenium.WebElement
			if selector == "" || strings.ToLower(selector) == "body" {
				body, err = w.wd.FindElement(selenium.ByTagName, "body")
				if err != nil {
					c1 <- tmpResult{nil, fmt.Errorf("failed to find body: %v", err)}
					return
				}
			} else if selector[0] == '#' {
				body, err = w.wd.FindElement(selenium.ByID, selector[1:])
				if err != nil {
					c1 <- tmpResult{nil, fmt.Errorf("failed to find %s: %v", selector, err)}
					return
				}
			} else if selector[0] == '.' {
				elements, err := w.wd.FindElements(selenium.ByClassName, selector[1:])
				if err != nil || len(elements) == 0 {
					c1 <- tmpResult{nil, fmt.Errorf("failed to find %s: %v", selector, err)}
					return
				}
				body = elements[0]
			}

			htmlSrc, err := body.GetAttribute("innerHTML")
			if err != nil {
				c1 <- tmpResult{nil, fmt.Errorf("failed to get body html: %v", err)}
				return
			}

			currentURL, err := w.wd.CurrentURL()
			if err != nil {
				c1 <- tmpResult{nil, fmt.Errorf("failed to get currentURL: %v", err)}
				return
			}
			w.opts.Logger.Debug("Done")
			webpage := &FetchedWebPage{Title: title, TargetURL: targetURL, CurrentURL: currentURL, Src: htmlSrc}
			c1 <- tmpResult{webpage, nil}
			return
		}
	}()

	// waiting for go routine to return
	retResult := <-c1

	return retResult.webPage, retResult.resultErr

}

func (w *WebFetcher) SearchJSON(ctx context.Context, query string) ([]SearchResult, error) {
	if w.search == nil {
		return nil, fmt.Errorf("search engine is not configured")
	}

	if w.cache != nil {
		if results, ok, err := w.cache.Get(ctx, query); err == nil && ok {
			w.opts.Logger.Debug("Returning search results from cache")
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
