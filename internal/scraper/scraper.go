// Package scraper provides the core web scraping functionality with a hybrid approach:
// HTTP-first scraping with browser automation fallback. It includes smart content
// extraction, image processing, and Cloudflare detection capabilities.
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
	httpCtx, cancel := context.WithTimeout(ctx, HTTPTimeout)
	defer cancel()

	html, finalURL, err := s.httpClient.FetchWithAlternatesGroup(httpCtx, targetURL)
	if err == nil {
		// Success with HTTP - extract content
		result := s.extractor.ExtractArticle(html, finalURL)
		return result, nil
	}

	// Phase 2: Browser fallback (40s budget)
	browserCtx, cancel := context.WithTimeout(ctx, BrowserTimeout)
	defer cancel()

	html, finalURL, err = s.browserClient.ScrapeWithBrowserOptimized(browserCtx, targetURL, int(BrowserTimeout.Milliseconds()))
	if err == nil {
		// Success with browser - extract content
		result := s.extractor.ExtractArticle(html, finalURL)
		return result, nil
	}

	// Check if it's a Cloudflare block
	if IsCloudflareBlock(err) {
		domain, _ := url.Parse(targetURL)
		return models.ScrapeResponse{
				Images: []string{},
			}, &models.CloudflareBlockError{
				Domain: domain.Hostname(),
				Err:    err,
			}
	}

	return models.ScrapeResponse{}, fmt.Errorf("scraping failed: %w", err)
}

// ScrapeSmartWithTimeout runs ScrapeSmart with a timeout
func (s *Scraper) ScrapeSmartWithTimeout(ctx context.Context, targetURL string, timeoutMs int) (models.ScrapeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	return s.ScrapeSmart(ctx, targetURL)
}
