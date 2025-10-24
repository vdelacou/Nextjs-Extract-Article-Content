package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"extract-html-scraper/internal/models"
	"extract-html-scraper/internal/scraper"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// LambdaHandler handles AWS Lambda events
type LambdaHandler struct {
	scraper *scraper.Scraper
}

func NewLambdaHandler() *LambdaHandler {
	return &LambdaHandler{
		scraper: scraper.NewScraper(),
	}
}

// Handler is the main Lambda handler function
func (h *LambdaHandler) Handler(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Set up base headers
	baseHeaders := map[string]string{
		"Content-Type":                 "application/json; charset=utf-8",
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "Content-Type,X-Api-Key,x-api-key",
		"Access-Control-Allow-Methods": "GET,OPTIONS",
	}

	// Handle preflight OPTIONS request
	if event.HTTPMethod == "OPTIONS" {
		return events.APIGatewayProxyResponse{
			StatusCode: 204,
			Headers:    baseHeaders,
			Body:       "",
		}, nil
	}

	// Log the request
	fmt.Printf("Request received: %+v\n", event)

	// Validate API key
	apiKey := event.Headers["x-api-key"]
	if apiKey == "" {
		apiKey = event.Headers["X-Api-Key"]
	}
	if apiKey == "" && event.QueryStringParameters != nil {
		apiKey = event.QueryStringParameters["key"]
	}

	validKey := os.Getenv("SCRAPE_API_KEY")
	if validKey == "" {
		fmt.Println("SCRAPE_API_KEY environment variable not set")
		return h.errorResponse(500, "Server misconfiguration", baseHeaders), nil
	}

	if apiKey == "" || apiKey != validKey {
		return h.errorResponse(401, "Invalid or missing API key", baseHeaders), nil
	}

	// Validate URL parameter
	targetURL := ""
	if event.QueryStringParameters != nil {
		targetURL = event.QueryStringParameters["url"]
	}

	if targetURL == "" {
		return h.errorResponse(400, "Missing \"url\" query parameter", baseHeaders), nil
	}

	// Validate URL format
	if _, err := url.Parse(targetURL); err != nil {
		return h.errorResponse(400, "Invalid URL format", baseHeaders), nil
	}

	fmt.Printf("Starting scrape for: %s\n", targetURL)

	// Calculate soft timeout
	remaining := 90000 // Default 90 seconds
	if ctx.Value("remainingTime") != nil {
		if remainingMs, ok := ctx.Value("remainingTime").(int); ok {
			remaining = remainingMs
		}
	}

	// 3s safety margin, max 70s cap
	softTimeoutMs := remaining - 3000
	if softTimeoutMs < 1000 {
		softTimeoutMs = 1000
	}
	if softTimeoutMs > 70000 {
		softTimeoutMs = 70000
	}

	// Create context with timeout
	scrapeCtx, cancel := context.WithTimeout(ctx, time.Duration(softTimeoutMs)*time.Millisecond)
	defer cancel()

	start := time.Now()

	// Perform scraping
	result, err := h.scraper.ScrapeSmartWithTimeout(scrapeCtx, targetURL, softTimeoutMs)

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

		body, _ := json.Marshal(blockedResponse)
		return events.APIGatewayProxyResponse{
			StatusCode: 451,
			Headers:    baseHeaders,
			Body:       string(body),
		}, nil
	}

	// Handle timeout
	if err != nil && strings.Contains(err.Error(), "context deadline exceeded") {
		return h.errorResponse(504, "Scrape took too long", baseHeaders), nil
	}

	// Handle other errors
	if err != nil {
		fmt.Printf("Error processing request: %v\n", err)
		return h.errorResponse(500, "Failed to scrape", baseHeaders), nil
	}

	// Add metadata to successful response
	result.Metadata = models.Metadata{
		URL:        targetURL,
		ScrapedAt:  time.Now(),
		DurationMs: duration.Milliseconds(),
	}

	// Return successful response
	body, err := json.Marshal(result)
	if err != nil {
		return h.errorResponse(500, "Failed to serialize response", baseHeaders), nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    baseHeaders,
		Body:       string(body),
	}, nil
}

// errorResponse creates an error response
func (h *LambdaHandler) errorResponse(statusCode int, message string, headers map[string]string) events.APIGatewayProxyResponse {
	errorResp := models.ErrorResponse{
		Error: message,
	}

	body, _ := json.Marshal(errorResp)
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       string(body),
	}
}

// main function
func main() {
	handler := NewLambdaHandler()
	lambda.Start(handler.Handler)
}
