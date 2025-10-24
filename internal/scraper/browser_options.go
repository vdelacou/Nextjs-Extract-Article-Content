// Package scraper provides browser configuration options for Chrome automation.
package scraper

import (
	"github.com/chromedp/chromedp"
)

// BrowserOptions contains configuration for browser automation
type BrowserOptions struct {
	Optimized    bool
	BlockImages  bool
	BlockJS      bool
	BlockFonts   bool
	BlockCSS     bool
	WindowWidth  int
	WindowHeight int
	UserAgent    string
}

// DefaultBrowserOptions returns standard browser options
func DefaultBrowserOptions() BrowserOptions {
	return BrowserOptions{
		Optimized:    false,
		BlockImages:  false,
		BlockJS:      false,
		BlockFonts:   false,
		BlockCSS:     false,
		WindowWidth:  DefaultWindowWidth,
		WindowHeight: DefaultWindowHeight,
	}
}

// OptimizedBrowserOptions returns optimized browser options for faster scraping
func OptimizedBrowserOptions() BrowserOptions {
	return BrowserOptions{
		Optimized:    true,
		BlockImages:  true,
		BlockJS:      false, // Keep JS for dynamic content
		BlockFonts:   true,
		BlockCSS:     true,
		WindowWidth:  DefaultWindowWidth,
		WindowHeight: DefaultWindowHeight,
	}
}

// BuildChromeOptions creates Chrome options based on BrowserOptions
func BuildChromeOptions(opts BrowserOptions) []chromedp.ExecAllocatorOption {
	chromeOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.WindowSize(opts.WindowWidth, opts.WindowHeight),
	)

	// Add user agent if provided
	if opts.UserAgent != "" {
		chromeOpts = append(chromeOpts, chromedp.UserAgent(opts.UserAgent))
	}

	// Add optimization flags
	if opts.Optimized {
		if opts.BlockImages {
			chromeOpts = append(chromeOpts, chromedp.Flag("disable-images", true))
		}
		if opts.BlockJS {
			chromeOpts = append(chromeOpts, chromedp.Flag("disable-javascript", true))
		}
		chromeOpts = append(chromeOpts,
			chromedp.Flag("disable-plugins", true),
			chromedp.Flag("disable-extensions", true),
		)
	}

	return chromeOpts
}

// GetRequestBlockingScript returns JavaScript for blocking unwanted requests
func GetRequestBlockingScript(opts BrowserOptions) string {
	script := `
		const originalFetch = window.fetch;
		const originalXHR = window.XMLHttpRequest;
		
		// Block ads and trackers
		const blockedDomains = [
			'doubleclick', 'googlesyndication', 'google-analytics',
			'facebook.com/tr', 'taboola', 'outbrain', 'scorecardresearch',
			'chartbeat', 'amazon-adsystem'
		];
		
		// Override fetch
		window.fetch = function(...args) {
			const url = args[0];
			if (typeof url === 'string' && blockedDomains.some(domain => url.includes(domain))) {
				return Promise.reject(new Error('Blocked'));
			}
			return originalFetch.apply(this, args);
		};
		
		// Override XMLHttpRequest
		const originalOpen = XMLHttpRequest.prototype.open;
		XMLHttpRequest.prototype.open = function(method, url, ...args) {
			if (typeof url === 'string' && blockedDomains.some(domain => url.includes(domain))) {
				throw new Error('Blocked');
			}
			return originalOpen.apply(this, [method, url, ...args]);
		};
	`

	if opts.Optimized {
		script += `
		// Block resource types for optimized mode
		const originalCreateElement = document.createElement;
		document.createElement = function(tagName) {
			const element = originalCreateElement.call(this, tagName);
			if (['img', 'link', 'style'].includes(tagName.toLowerCase())) {
				element.style.display = 'none';
			}
			return element;
		};
		
		// Hide webdriver detection
		Object.defineProperty(navigator, 'webdriver', {
			get: () => false
		});
		`
	}

	return script
}
