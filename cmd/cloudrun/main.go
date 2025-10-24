package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"extract-html-scraper/internal/models"
	"extract-html-scraper/internal/scraper"
)

// CloudRunHandler handles Google Cloud Run requests
type CloudRunHandler struct {
	scraper *scraper.Scraper
}

func NewCloudRunHandler() *CloudRunHandler {
	return &CloudRunHandler{
		scraper: scraper.NewScraper(),
	}
}

// Handler is the main Cloud Run handler function
func (h *CloudRunHandler) Handler(w http.ResponseWriter, r *http.Request) {
	// Set up CORS headers
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-Api-Key,x-api-key")
	w.Header().Set("Access-Control-Allow-Methods", "GET,OPTIONS")

	// Handle preflight OPTIONS request
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Only allow GET requests
	if r.Method != "GET" {
		h.errorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Log the request
	fmt.Printf("Request received: %s %s\n", r.Method, r.URL.String())

	// API key validation is now handled by API Gateway

	// Validate URL parameter
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		h.errorResponse(w, http.StatusBadRequest, "Missing \"url\" query parameter")
		return
	}

	// Validate URL format
	if _, err := url.Parse(targetURL); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid URL format")
		return
	}

	fmt.Printf("Starting scrape for: %s\n", targetURL)

	// Calculate timeout (Cloud Run has 5 minute max)
	timeoutStr := r.URL.Query().Get("timeout")
	timeoutMs := 300000 // Default 5 minutes
	if timeoutStr != "" {
		if parsedTimeout, err := strconv.Atoi(timeoutStr); err == nil {
			timeoutMs = parsedTimeout
		}
	}

	// Cap at 4 minutes to be safe
	if timeoutMs > 240000 {
		timeoutMs = 240000
	}
	if timeoutMs < 1000 {
		timeoutMs = 1000
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	start := time.Now()

	// Perform scraping
	result, err := h.scraper.ScrapeSmartWithTimeout(ctx, targetURL, timeoutMs)

	duration := time.Since(start)
	fmt.Printf("✓ Scraped in %dms\n", duration.Milliseconds())

	// Handle Cloudflare blocking
	if cfErr, ok := err.(*scraper.CloudflareBlockError); ok {
		blockedResponse := models.BlockedResponse{
			Error:    "Blocked by site protection",
			Provider: "cloudflare",
			Domain:   cfErr.Domain,
			Metadata: models.Metadata{
				URL:        targetURL,
				ScrapedAt:  time.Now(),
				DurationMs: duration.Milliseconds(),
			},
		}

		w.WriteHeader(http.StatusUnavailableForLegalReasons)
		json.NewEncoder(w).Encode(blockedResponse)
		return
	}

	// Handle timeout
	if err != nil && strings.Contains(err.Error(), "context deadline exceeded") {
		h.errorResponse(w, http.StatusGatewayTimeout, "Scrape took too long")
		return
	}

	// Handle other errors
	if err != nil {
		fmt.Printf("Error processing request: %v\n", err)
		h.errorResponse(w, http.StatusInternalServerError, "Failed to scrape")
		return
	}

	// Add metadata to successful response
	result.Metadata = models.Metadata{
		URL:        targetURL,
		ScrapedAt:  time.Now(),
		DurationMs: duration.Milliseconds(),
	}

	// Return successful response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// errorResponse creates an error response
func (h *CloudRunHandler) errorResponse(w http.ResponseWriter, statusCode int, message string) {
	errorResp := models.ErrorResponse{
		Error: message,
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResp)
}

// main function
func main() {
	handler := NewCloudRunHandler()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting server on port %s\n", port)
	http.HandleFunc("/", handler.Handler)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
		os.Exit(1)
	}
}
