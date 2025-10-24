package scraper

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"extract-html-scraper/internal/config"

	"github.com/chromedp/chromedp"
)

type BrowserClient struct {
	config  config.ScrapeConfig
	regexes map[string]*regexp.Regexp
}

func NewBrowserClient() *BrowserClient {
	cfg := config.DefaultScrapeConfig()
	regexes := config.CompileRegexes()

	return &BrowserClient{
		config:  cfg,
		regexes: regexes,
	}
}

// ScrapeWithBrowser uses chromedp to scrape content with fallback to alternate URLs
func (b *BrowserClient) ScrapeWithBrowser(ctx context.Context, targetURL string, timeoutMs int) (string, string, error) {
	opts := DefaultBrowserOptions()
	opts.UserAgent = b.config.UserAgent
	return b.scrapeWithOptions(ctx, targetURL, timeoutMs, opts)
}

// ScrapeWithBrowserOptimized is an optimized version that blocks more resources
func (b *BrowserClient) ScrapeWithBrowserOptimized(ctx context.Context, targetURL string, timeoutMs int) (string, string, error) {
	opts := OptimizedBrowserOptions()
	opts.UserAgent = b.config.UserAgent
	return b.scrapeWithOptions(ctx, targetURL, timeoutMs, opts)
}

// scrapeWithOptions is the unified scraping function using browser options
func (b *BrowserClient) scrapeWithOptions(ctx context.Context, targetURL string, timeoutMs int, opts BrowserOptions) (string, string, error) {
	// Create a new context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Build Chrome options
	chromeOpts := BuildChromeOptions(opts)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, chromeOpts...)
	defer cancel()

	// Create browser context
	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()

	// Set up request blocking
	err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Run(ctx, chromedp.Tasks{
				chromedp.Evaluate(GetRequestBlockingScript(opts), nil),
			})
		}),
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to set up request blocking: %w", err)
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

// navigateAndExtractOptimized uses domcontentloaded for faster loading
func (b *BrowserClient) navigateAndExtractOptimized(ctx context.Context, targetURL string) (string, string, error) {
	return b.navigateAndExtract(ctx, targetURL)
}
