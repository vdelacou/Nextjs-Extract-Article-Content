package scraper

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"extract-html-scraper/internal/models"
)

// Scraper orchestrates the scraping process with HTTP-first, browser-fallback strategy
type Scraper struct {
	httpClient    *HTTPClient
	browserClient *BrowserClient
	extractor     *ArticleExtractor
}

func NewScraper() *Scraper {
	return &Scraper{
		httpClient:    NewHTTPClient(),
		browserClient: NewBrowserClient(),
		extractor:     NewArticleExtractor(),
	}
}

// ScrapeSmart implements the hybrid scraping strategy: HTTP first, browser fallback
func (s *Scraper) ScrapeSmart(ctx context.Context, targetURL string) (models.ScrapeResponse, error) {
	// Validate URL
	if _, err := url.Parse(targetURL); err != nil {
		return models.ScrapeResponse{}, fmt.Errorf("invalid URL: %w", err)
	}

	// Phase 1: Try HTTP fetching with alternate URLs (18s budget)
	httpCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()

	html, finalURL, err := s.httpClient.FetchWithAlternatesGroup(httpCtx, targetURL)
	if err == nil {
		// Success with HTTP - extract content
		result := s.extractor.ExtractArticle(html, finalURL)
		return result, nil
	}

	// Phase 2: Browser fallback (40s budget)
	browserCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()

	html, finalURL, err = s.browserClient.ScrapeWithBrowserOptimized(browserCtx, targetURL, 40000)
	if err == nil {
		// Success with browser - extract content
		result := s.extractor.ExtractArticle(html, finalURL)
		return result, nil
	}

	// Check if it's a Cloudflare block
	if s.isCloudflareBlock(err) {
		domain, _ := url.Parse(targetURL)
		return models.ScrapeResponse{
				Images: []string{},
			}, &CloudflareBlockError{
				Domain: domain.Hostname(),
				Err:    err,
			}
	}

	return models.ScrapeResponse{}, fmt.Errorf("scraping failed: %w", err)
}

// isCloudflareBlock checks if the error indicates Cloudflare blocking
func (s *Scraper) isCloudflareBlock(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return containsAny(errStr, []string{
		"CF_BLOCKED",
		"cloudflare",
		"HTTP 403",
		"all alternate URLs failed",
	})
}

// containsAny checks if a string contains any of the substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					indexOf(s, substr) >= 0))
}

// indexOf finds the index of substr in s (case-insensitive)
func indexOf(s, substr string) int {
	sLower := toLower(s)
	substrLower := toLower(substr)

	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return i
		}
	}
	return -1
}

// toLower converts string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i, b := range []byte(s) {
		if b >= 'A' && b <= 'Z' {
			result[i] = b + 32
		} else {
			result[i] = b
		}
	}
	return string(result)
}

// CloudflareBlockError represents a Cloudflare blocking error
type CloudflareBlockError struct {
	Domain string
	Err    error
}

func (e *CloudflareBlockError) Error() string {
	return fmt.Sprintf("blocked by Cloudflare on domain %s: %v", e.Domain, e.Err)
}

// ScrapeSmartWithTimeout runs ScrapeSmart with a timeout
func (s *Scraper) ScrapeSmartWithTimeout(ctx context.Context, targetURL string, timeoutMs int) (models.ScrapeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	return s.ScrapeSmart(ctx, targetURL)
}
