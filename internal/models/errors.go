// Package models defines typed errors for better error handling and context.
package models

import "fmt"

// CloudflareBlockError represents a Cloudflare blocking error
type CloudflareBlockError struct {
	Domain string
	Err    error
}

func (e *CloudflareBlockError) Error() string {
	return fmt.Sprintf("blocked by Cloudflare on domain %s: %v", e.Domain, e.Err)
}

// TimeoutError represents a timeout error
type TimeoutError struct {
	Operation string
	Timeout   string
	Err       error
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout during %s after %s: %v", e.Operation, e.Timeout, e.Err)
}

// InvalidURLError represents an invalid URL error
type InvalidURLError struct {
	URL string
	Err error
}

func (e *InvalidURLError) Error() string {
	return fmt.Sprintf("invalid URL %s: %v", e.URL, e.Err)
}

// HTTPError represents an HTTP-related error
type HTTPError struct {
	StatusCode int
	URL        string
	Err        error
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d for URL %s: %v", e.StatusCode, e.URL, e.Err)
}

// ContentExtractionError represents an error during content extraction
type ContentExtractionError struct {
	Step string
	Err  error
}

func (e *ContentExtractionError) Error() string {
	return fmt.Sprintf("content extraction failed at %s: %v", e.Step, e.Err)
}
