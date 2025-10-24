package scraper

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"extract-html-scraper/internal/config"
	"extract-html-scraper/internal/models"

	"github.com/chromedp/chromedp"
)

type BrowserClient struct {
	config  models.ScrapeConfig
	regexes map[string]*regexp.Regexp
}

func NewBrowserClient() *BrowserClient {
	cfg := config.DefaultScrapeConfig()
	regexes := config.CompileRegexes()

	return &BrowserClient{
		config: models.ScrapeConfig{
			UserAgent:      cfg.UserAgent,
			TimeoutMs:      cfg.TimeoutMs,
			SizeLimitBytes: cfg.SizeLimitBytes,
			MaxRetries:     cfg.MaxRetries,
			ChromeMajor:    cfg.ChromeMajor,
		},
		regexes: regexes,
	}
}

// ScrapeWithBrowser uses chromedp to scrape content with fallback to alternate URLs
func (b *BrowserClient) ScrapeWithBrowser(ctx context.Context, targetURL string, timeoutMs int) (string, string, error) {
	// Create a new context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Configure chromedp options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.UserAgent(b.config.UserAgent),
		chromedp.WindowSize(1366, 900),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	// Create browser context
	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()

	// Set up request interception to block ads and unnecessary resources
	err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Enable request interception
			return chromedp.Run(ctx, chromedp.Tasks{
				chromedp.Evaluate(`
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
				`, nil),
			})
		}),
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to set up request interception: %w", err)
	}

	// Try primary URL first
	html, finalURL, err := b.navigateAndExtract(ctx, targetURL)
	if err == nil && !b.LooksLikeCFBlock(html) {
		return html, finalURL, nil
	}

	// Generate alternate URLs and try them
	alternates, err := b.GenerateAlternateURLs(targetURL)
	if err != nil {
		return "", "", err
	}

	for _, altURL := range alternates {
		html, finalURL, err := b.navigateAndExtract(ctx, altURL)
		if err == nil && !b.LooksLikeCFBlock(html) {
			return html, finalURL, nil
		}
	}

	return "", "", fmt.Errorf("all URLs failed or were blocked by Cloudflare")
}

// navigateAndExtract navigates to a URL and extracts HTML content
func (b *BrowserClient) navigateAndExtract(ctx context.Context, targetURL string) (string, string, error) {
	var html string
	var finalURL string

	err := chromedp.Run(ctx, chromedp.Tasks{
		// Navigate to the URL
		chromedp.Navigate(targetURL),

		// Wait for network to be idle
		chromedp.WaitReady("body"),

		// Get the final URL after redirects
		chromedp.Location(&finalURL),

		// Get the HTML content
		chromedp.OuterHTML("html", &html),
	})

	if err != nil {
		return "", "", fmt.Errorf("navigation failed: %w", err)
	}

	return html, finalURL, nil
}

// LooksLikeCFBlock checks if HTML content indicates Cloudflare blocking
func (b *BrowserClient) LooksLikeCFBlock(html string) bool {
	htmlLower := strings.ToLower(html)
	return b.regexes["cfBlock"].MatchString(htmlLower)
}

// GenerateAlternateURLs creates alternative URLs for AMP/mobile fallback
func (b *BrowserClient) GenerateAlternateURLs(originalURL string) ([]string, error) {
	// Reuse the same logic from HTTP client
	httpClient := NewHTTPClient()
	return httpClient.GenerateAlternateURLs(originalURL)
}

// ScrapeWithBrowserOptimized is an optimized version that blocks more resources
func (b *BrowserClient) ScrapeWithBrowserOptimized(ctx context.Context, targetURL string, timeoutMs int) (string, string, error) {
	// Create a new context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Configure chromedp options with more aggressive blocking
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.Flag("disable-images", true),
		chromedp.Flag("disable-javascript", false), // Keep JS for dynamic content
		chromedp.Flag("disable-plugins", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.UserAgent(b.config.UserAgent),
		chromedp.WindowSize(1366, 900),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	// Create browser context
	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()

	// Set up comprehensive request blocking
	err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Run(ctx, chromedp.Tasks{
				// Block resource types
				chromedp.Evaluate(`
					// Block images, fonts, stylesheets, and media
					const originalCreateElement = document.createElement;
					document.createElement = function(tagName) {
						const element = originalCreateElement.call(this, tagName);
						if (['img', 'link', 'style'].includes(tagName.toLowerCase())) {
							element.style.display = 'none';
						}
						return element;
					};
					
					// Block fetch requests for unwanted resources
					const originalFetch = window.fetch;
					window.fetch = function(...args) {
						const url = args[0];
						if (typeof url === 'string') {
							const blockedPatterns = [
								/\.(jpg|jpeg|png|gif|webp|svg|ico)$/i,
								/\.(woff|woff2|ttf|eot)$/i,
								/\.css$/i,
								/doubleclick|googlesyndication|google-analytics/i,
								/facebook\.com\/tr|taboola|outbrain/i
							];
							
							if (blockedPatterns.some(pattern => pattern.test(url))) {
								return Promise.reject(new Error('Blocked resource'));
							}
						}
						return originalFetch.apply(this, args);
					};
					
					// Hide webdriver detection
					Object.defineProperty(navigator, 'webdriver', {
						get: () => false
					});
				`, nil),
			})
		}),
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to set up resource blocking: %w", err)
	}

	// Try primary URL first
	html, finalURL, err := b.navigateAndExtractOptimized(ctx, targetURL)
	if err == nil && !b.LooksLikeCFBlock(html) {
		return html, finalURL, nil
	}

	// Generate alternate URLs and try them
	alternates, err := b.GenerateAlternateURLs(targetURL)
	if err != nil {
		return "", "", err
	}

	for _, altURL := range alternates {
		html, finalURL, err := b.navigateAndExtractOptimized(ctx, altURL)
		if err == nil && !b.LooksLikeCFBlock(html) {
			return html, finalURL, nil
		}
	}

	return "", "", fmt.Errorf("all URLs failed or were blocked by Cloudflare")
}

// navigateAndExtractOptimized uses domcontentloaded for faster loading
func (b *BrowserClient) navigateAndExtractOptimized(ctx context.Context, targetURL string) (string, string, error) {
	var html string
	var finalURL string

	err := chromedp.Run(ctx, chromedp.Tasks{
		// Navigate to the URL with faster wait condition
		chromedp.Navigate(targetURL),

		// Wait for DOM content loaded (faster than networkidle)
		chromedp.WaitReady("body", chromedp.ByQuery),

		// Get the final URL after redirects
		chromedp.Location(&finalURL),

		// Get the HTML content
		chromedp.OuterHTML("html", &html),
	})

	if err != nil {
		return "", "", fmt.Errorf("navigation failed: %w", err)
	}

	return html, finalURL, nil
}
