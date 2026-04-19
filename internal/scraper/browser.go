package scraper

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

var (
	browser *rod.Browser
	page    *rod.Page
	mu      sync.Mutex
)

func hasDisplay() bool {
	switch runtime.GOOS {
	case "darwin", "windows":
		return true
	default:
		return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
	}
}

func launchBrowser() (*rod.Browser, error) {
	headless := !hasDisplay()

	build := func(headless bool) *launcher.Launcher {
		l := launcher.New().Headless(headless)
		if runtime.GOOS == "linux" {
			l = l.Set("no-sandbox").Set("disable-dev-shm-usage")
		}
		return l
	}

	url, err := build(headless).Launch()
	if err != nil && !headless {
		url, err = build(true).Launch()
	}
	if err != nil {
		return nil, fmt.Errorf("launch chromium: %w (is a Chromium/Chrome binary available? rod will auto-download on first use)", err)
	}

	b := rod.New().ControlURL(url)
	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("connect to chromium: %w", err)
	}
	return b, nil
}

func BrowserView(url string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if browser == nil {
		b, err := launchBrowser()
		if err != nil {
			return "", err
		}
		browser = b
	}

	if page == nil {
		p, err := browser.Page(proto.TargetCreateTarget{URL: url})
		if err != nil {
			return "", fmt.Errorf("open page: %w", err)
		}
		page = p
	} else {
		if err := page.Navigate(url); err != nil {
			return "", fmt.Errorf("navigate: %w", err)
		}
	}

	if err := page.WaitLoad(); err != nil {
		return "", fmt.Errorf("wait load: %w", err)
	}

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

// BrowserOpen navigates to a URL and shows it in the rod browser window without
// blocking to extract page text. Used for auto-previewing web dev servers.
func BrowserOpen(url string) error {
	mu.Lock()
	defer mu.Unlock()

	if browser == nil {
		b, err := launchBrowser()
		if err != nil {
			return err
		}
		browser = b
	}

	if page == nil {
		p, err := browser.Page(proto.TargetCreateTarget{URL: url})
		if err != nil {
			return fmt.Errorf("open page: %w", err)
		}
		page = p
	} else {
		if err := page.Navigate(url); err != nil {
			return fmt.Errorf("navigate: %w", err)
		}
	}
	// Don't wait for full load — just fire and return so the user sees the
	// browser window appear immediately.
	return nil
}

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
	if res == nil {
		return "", nil
	}
	if res.UnserializableValue != "" {
		return string(res.UnserializableValue), nil
	}
	if res.Value.Nil() {
		return "undefined", nil
	}
	return res.Value.String(), nil
}

func BrowserClose() string {
	mu.Lock()
	defer mu.Unlock()

	if browser != nil {
		if err := browser.Close(); err != nil {
			browser = nil
			page = nil
			return "browser close error: " + err.Error()
		}
		browser = nil
		page = nil
		return "browser closed"
	}
	return "browser was not open"
}
