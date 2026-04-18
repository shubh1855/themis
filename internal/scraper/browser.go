package scraper

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

var (
	browser *rod.Browser
	page    *rod.Page
	mu      sync.Mutex
)

// BrowserView opens a visible browser window, navigates to the URL, and extracts text.
func BrowserView(url string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if browser == nil {
		u := launcher.New().
			Headless(false). // Make it visible to the user
			MustLaunch()
		browser = rod.New().ControlURL(u).MustConnect()
	}

	var err error
	if page == nil {
		page = browser.MustPage(url)
	} else {
		err = page.Navigate(url)
		if err != nil {
			return "", err
		}
	}

	// Wait for the page to load
	page.WaitLoad()

	// Give JS frameworks a moment to render
	time.Sleep(2 * time.Second)

	body, err := page.Element("body")
	if err != nil {
		return "Page loaded, but could not read body text.", nil
	}
	
	text, err := body.Text()
	if err != nil {
		return "Page loaded, but error reading text.", nil
	}

	if len(text) > 4000 {
		text = text[:4000] + "\n...(truncated)"
	}
	return text, nil
}

// BrowserRunJS executes a JavaScript snippet in the current browser page.
func BrowserRunJS(script string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if page == nil {
		return "", fmt.Errorf("no active browser page, use browser_view first")
	}

	res, err := page.Eval(script)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", res.Value), nil
}

// BrowserClose closes the current visible browser instance.
func BrowserClose() string {
	mu.Lock()
	defer mu.Unlock()

	if browser != nil {
		browser.MustClose()
		browser = nil
		page = nil
		return "browser closed"
	}
	return "browser was not open"
}
