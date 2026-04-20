package scraper

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

var (
	browser *rod.Browser
	page    *rod.Page
	mu      sync.Mutex
	consoleErrors []string
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
		
		injectVirtualCursor(page)

		go page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
			if e.Type == proto.RuntimeConsoleAPICalledTypeWarning || e.Type == proto.RuntimeConsoleAPICalledTypeError {
				msg := ""
				for _, arg := range e.Args {
					msg += arg.Value.String() + " "
				}
				mu.Lock()
				consoleErrors = append(consoleErrors, fmt.Sprintf("[%s]: %s", e.Type, msg))
				mu.Unlock()
			}
		})()
		
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

		injectVirtualCursor(page)

		go page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
			if e.Type == proto.RuntimeConsoleAPICalledTypeWarning || e.Type == proto.RuntimeConsoleAPICalledTypeError {
				msg := ""
				for _, arg := range e.Args {
					msg += arg.Value.String() + " "
				}
				mu.Lock()
				consoleErrors = append(consoleErrors, fmt.Sprintf("[%s]: %s", e.Type, msg))
				mu.Unlock()
			}
		})()
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

	res, err := proto.RuntimeEvaluate{
		Expression:                  script,
		ReturnByValue:               true,
		AwaitPromise:                true,
		UserGesture:                 true,
		ReplMode:                    true,
		AllowUnsafeEvalBlockedByCSP: true,
	}.Call(page)
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", nil
	}
	if res.ExceptionDetails != nil {
		return "", &rod.EvalError{RuntimeExceptionDetails: res.ExceptionDetails}
	}
	if res.Result == nil {
		return "", nil
	}
	obj := res.Result
	if obj.UnserializableValue != "" {
		return string(obj.UnserializableValue), nil
	}
	if obj.Value.Nil() {
		return "undefined", nil
	}
	return obj.Value.String(), nil
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

func BrowserScreenshot(path string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if page == nil {
		return "", fmt.Errorf("no active browser page")
	}
	data, err := page.Screenshot(true, nil)
	if err != nil {
		return "", fmt.Errorf("screenshot: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write screenshot: %w", err)
	}
	
	// Collect stored errors
	errLogs := ""
	if len(consoleErrors) > 0 {
		errLogs = "\n\nConsole Errors:\n" + strings.Join(consoleErrors, "\n")
		consoleErrors = nil // clear after reporting
	}
	
	return fmt.Sprintf("Screenshot saved to %s (IMAGE_OUTPUT_IMAGE).%s", path, errLogs), nil
}

func injectVirtualCursor(page *rod.Page) {
	script := `() => {
		const init = () => {
			if (!document.body) { requestAnimationFrame(init); return; }
			if (document.getElementById('themis-cursor')) return;
			const cursor = document.createElement('div');
		cursor.id = 'themis-cursor';
		cursor.style.width = '20px';
		cursor.style.height = '20px';
		cursor.style.background = 'rgba(0, 240, 255, 0.4)';
		cursor.style.border = '2px solid #00F0FF';
		cursor.style.borderRadius = '50%';
		cursor.style.position = 'fixed';
		cursor.style.zIndex = '2147483647';
		cursor.style.pointerEvents = 'none';
		cursor.style.boxShadow = '0 0 10px #00F0FF';
		cursor.style.transition = 'top 0.05s linear, left 0.05s linear';
		document.body.appendChild(cursor);
		
		document.addEventListener('mousemove', (e) => {
			const c = document.getElementById('themis-cursor');
			if (c) {
				c.style.left = (e.clientX - 10) + 'px';
				c.style.top = (e.clientY - 10) + 'px';
			}
		}, true);
		
		document.addEventListener('mousedown', (e) => {
			const ripple = document.createElement('div');
			ripple.style.position = 'fixed';
			ripple.style.left = (e.clientX - 15) + 'px';
			ripple.style.top = (e.clientY - 15) + 'px';
			ripple.style.width = '30px';
			ripple.style.height = '30px';
			ripple.style.border = '2px solid #00F0FF';
			ripple.style.borderRadius = '50%';
			ripple.style.zIndex = '2147483646';
			ripple.style.pointerEvents = 'none';
			ripple.style.transition = 'all 0.4s ease-out';
			document.body.appendChild(ripple);
			
			requestAnimationFrame(() => {
				ripple.style.transform = 'scale(2.5)';
				ripple.style.opacity = '0';
			});
			setTimeout(() => ripple.remove(), 400);
		}, true);
		};
		init();
	}`
	_, _ = page.EvalOnNewDocument(script)
	_, _ = page.Eval(script)
}

func BrowserHighlight(selector string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if page == nil {
		return "", fmt.Errorf("no active browser page")
	}
	el, err := page.Timeout(5 * time.Second).Element(selector)
	if err != nil {
		return "", fmt.Errorf("element not found: %w", err)
	}

	_, err = el.Eval(`() => {
		const el = this;
		el.style.outline = "4px solid #00F0FF";
		el.style.outlineOffset = "2px";
		el.style.boxShadow = "0 0 15px #00F0FF";
		el.style.transition = "all 0.3s ease";
		
		const label = document.createElement('div');
		label.innerText = 'THEMIS FOCUS';
		const rect = el.getBoundingClientRect();
		label.style.cssText = 'position:fixed; background:#00F0FF; color:#000; font-size:10px; font-weight:bold; padding:2px 4px; z-index:2147483647; border-radius:2px; top:' + Math.max(0, rect.top - 20) + 'px; left:' + rect.left + 'px;';
		label.className = 'themis-focus-label';
		document.body.appendChild(label);
		
		setTimeout(() => {
			el.style.outline = "";
			el.style.outlineOffset = "";
			el.style.boxShadow = "";
			el.style.transition = "";
			if (label.parentNode) label.remove();
		}, 800);
	}`)
	if err != nil {
		return "", fmt.Errorf("highlight failed: %w", err)
	}
	time.Sleep(600 * time.Millisecond) // Let the user see it
	return "highlighted: " + selector, nil
}

func smoothGlide(el *rod.Element) error {
	box, err := el.Shape()
	if err != nil {
		return err
	}
	bx := box.Box()
	targetX := bx.X + bx.Width/2
	targetY := bx.Y + bx.Height/2
	if err := el.Page().Mouse.MoveLinear(proto.Point{X: targetX, Y: targetY}, 12); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	return nil
}

func BrowserClick(selector string) (string, error) {
	// First highlight the element so the judge sees it.
	// We call BrowserHighlight which acquires its own lock, so we don't lock here yet.
	_, _ = BrowserHighlight(selector)

	mu.Lock()
	defer mu.Unlock()
	if page == nil {
		return "", fmt.Errorf("no active browser page")
	}
	el, err := page.Timeout(5 * time.Second).Element(selector)
	if err != nil {
		return "", fmt.Errorf("element not found: %w", err)
	}
	_ = el.ScrollIntoView()
	time.Sleep(100 * time.Millisecond)
	
	if err := smoothGlide(el); err != nil {
		return "", fmt.Errorf("mouse glide failed: %w", err)
	}

	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return "", fmt.Errorf("click failed: %w", err)
	}
	time.Sleep(1 * time.Second) // wait for potential navigation
	return "Clicked " + selector, nil
}

func BrowserType(selector, text string) (string, error) {
	_, _ = BrowserHighlight(selector)

	mu.Lock()
	defer mu.Unlock()
	if page == nil {
		return "", fmt.Errorf("no active browser page")
	}
	el, err := page.Timeout(5 * time.Second).Element(selector)
	if err != nil {
		return "", fmt.Errorf("element not found: %w", err)
	}
	_ = el.ScrollIntoView()
	time.Sleep(100 * time.Millisecond)

	if err := smoothGlide(el); err != nil {
		return "", fmt.Errorf("mouse glide failed: %w", err)
	}

	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return "", fmt.Errorf("click focus failed: %w", err)
	}

	_ = el.Input("")
	time.Sleep(100 * time.Millisecond)

	for _, c := range text {
		time.Sleep(time.Duration(40 + (c % 80)) * time.Millisecond)
		if err := page.InsertText(string(c)); err != nil {
			return "", fmt.Errorf("type failed: %w", err)
		}
	}

	return "Typed into " + selector, nil
}

func BrowserHover(selector string) (string, error) {
	_, _ = BrowserHighlight(selector)

	mu.Lock()
	defer mu.Unlock()
	if page == nil {
		return "", fmt.Errorf("no active browser page")
	}
	el, err := page.Timeout(5 * time.Second).Element(selector)
	if err != nil {
		return "", fmt.Errorf("element not found: %w", err)
	}
	_ = el.ScrollIntoView()
	time.Sleep(100 * time.Millisecond)

	if err := smoothGlide(el); err != nil {
		return "", fmt.Errorf("mouse glide failed: %w", err)
	}

	time.Sleep(400 * time.Millisecond) // Let hover render
	return "Hovered over " + selector, nil
}

func BrowserInspect(selector string) (string, error) {
	_, _ = BrowserHighlight(selector)

	mu.Lock()
	defer mu.Unlock()
	if page == nil {
		return "", fmt.Errorf("no active browser page")
	}
	
	// Optional: Get full AX Tree, but for simplicity we will just extract
	// the accessible name, role and bounding box for the target element.
	
	el, err := page.Timeout(5 * time.Second).Element(selector)
	if err != nil {
		return "", fmt.Errorf("element not found: %w", err)
	}
	
	// A rudimentary fetch of name/role via accessibility node if supported easily,
	// or standard properties.
	node, err := el.Describe(1, true)
	if err != nil {
		return "", fmt.Errorf("describe element failed: %w", err)
	}

	bx, _ := el.Shape()
	box := bx.Box()

	desc := fmt.Sprintf("Element Inspector:\nTag: %s\nBox: X:%.1f Y:%.1f W:%.1f H:%.1f\n", 
		node.NodeName, box.X, box.Y, box.Width, box.Height)

	// Attempt to get text content or value
	if val, err := el.Text(); err == nil && val != "" {
		desc += "Text: " + val + "\n"
	}
	
	return desc, nil
}

func BrowserScroll(direction string, amount int) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if page == nil {
		return "", fmt.Errorf("no active browser page")
	}
	y := 0
	if direction == "up" {
		y = -amount
	} else {
		y = amount
	}
	
	script := fmt.Sprintf("window.scrollBy({top: %d, behavior: 'smooth'})", y)
	if _, err := page.Eval(script); err != nil {
		return "", fmt.Errorf("scroll failed: %w", err)
	}

	time.Sleep(500 * time.Millisecond)
	return fmt.Sprintf("Scrolled %s by %d px", direction, amount), nil
}
