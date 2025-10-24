package models

import "time"

// ScrapeRequest represents the incoming Lambda event
type ScrapeRequest struct {
	URL string `json:"url"`
}

// ScrapeResponse represents the successful scraping result
type ScrapeResponse struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Content     string   `json:"content,omitempty"`
	Images      []string `json:"images"`
	Metadata    Metadata `json:"metadata"`
}

// BlockedResponse represents when scraping is blocked
type BlockedResponse struct {
	Error    string   `json:"error"`
	Provider string   `json:"provider"`
	Domain   string   `json:"domain"`
	Metadata Metadata `json:"metadata"`
}

// ErrorResponse represents error responses
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// Metadata contains request metadata
type Metadata struct {
	URL        string    `json:"url"`
	ScrapedAt  time.Time `json:"scrapedAt"`
	DurationMs int64     `json:"durationMs"`
}

// ImageCandidate represents a potential image with scoring data
type ImageCandidate struct {
	URL       string
	Width     int
	Height    int
	InArticle bool
	BadHint   bool
	Source    string
	Score     float64
	Area      int
}

// ImageConfig contains configuration for image extraction
type ImageConfig struct {
	MinShortSide   int
	MinArea        int
	MinAspect      float64
	MaxAspect      float64
	RatioWhitelist []float64
	RatioTol       float64
	AdSizes        map[string]bool
	BadHintRegex   string
}

// ScrapeConfig contains general scraping configuration
type ScrapeConfig struct {
	UserAgent      string
	TimeoutMs      int
	SizeLimitBytes int
	MaxRetries     int
	ChromeMajor    int
}
