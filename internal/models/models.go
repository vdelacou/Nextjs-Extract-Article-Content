// Package models defines the data structures used for web scraping requests and responses.
// It includes types for scrape requests, responses, errors, and metadata.
package models

import "time"

// ScrapeRequest represents the incoming scrape request
type ScrapeRequest struct {
	URL string `json:"url"`
}

// Quality represents content quality metrics
type Quality struct {
	Score              int     `json:"score"`              // 0-100 confidence score
	TextToHTMLRatio    float64 `json:"textToHtmlRatio"`    // Higher is better
	ParagraphCount     int     `json:"paragraphCount"`     // Number of paragraphs
	AvgParagraphLength int     `json:"avgParagraphLength"` // Average characters per paragraph
	HasHeaders         bool    `json:"hasHeaders"`         // Contains headings
	LinkDensity        float64 `json:"linkDensity"`        // Links per 1000 chars (lower is better)
	WordCount          int     `json:"wordCount"`          // Estimated word count
}

// ScrapeResponse represents the successful scraping result
type ScrapeResponse struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Content     string   `json:"content,omitempty"`
	Images      []string `json:"images"`
	Metadata    Metadata `json:"metadata"`
	Author      string   `json:"author,omitempty"`
	PublishDate string   `json:"publishDate,omitempty"`
	Excerpt     string   `json:"excerpt,omitempty"`
	ReadingTime int      `json:"readingTime,omitempty"`
	Language    string   `json:"language,omitempty"`
	TextLength  int      `json:"textLength,omitempty"`
	Quality     Quality  `json:"quality,omitempty"`
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
