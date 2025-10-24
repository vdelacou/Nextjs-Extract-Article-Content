package scraper

import (
	"strings"
)

// ContentQuality represents quality metrics for extracted content
type ContentQuality struct {
	Score              int     `json:"score"`              // 0-100 confidence score
	TextToHTMLRatio    float64 `json:"textToHtmlRatio"`    // Higher is better
	ParagraphCount     int     `json:"paragraphCount"`     // Number of paragraphs
	AvgParagraphLength int     `json:"avgParagraphLength"` // Average characters per paragraph
	HasHeaders         bool    `json:"hasHeaders"`         // Contains headings
	LinkDensity        float64 `json:"linkDensity"`        // Links per 1000 chars (lower is better)
	WordCount          int     `json:"wordCount"`          // Estimated word count
}

// ScoreContentQuality analyzes content and returns quality metrics
func ScoreContentQuality(content, originalHTML string) ContentQuality {
	if content == "" {
		return ContentQuality{Score: 0}
	}

	// Basic metrics
	wordCount, paragraphCount, avgParagraphLength := CalculateContentMetrics(content)

	// Check for headers
	hasHeaders := strings.Contains(content, "\n") &&
		(strings.Contains(strings.ToLower(content), "heading") ||
			strings.Count(content, "\n") > paragraphCount/2) // Rough heuristic

	// Calculate text-to-HTML ratio
	textToHTMLRatio := 0.0
	if len(originalHTML) > 0 {
		textToHTMLRatio = float64(len(content)) / float64(len(originalHTML))
	}

	// Calculate link density (rough estimation)
	linkCount := strings.Count(content, "http") + strings.Count(content, "www.")
	charCount := len(content)
	linkDensity := 0.0
	if charCount > 0 {
		linkDensity = float64(linkCount) / float64(charCount) * 1000
	}

	// Calculate overall score (0-100)
	score := calculateOverallScore(wordCount, paragraphCount, avgParagraphLength,
		hasHeaders, textToHTMLRatio, linkDensity)

	return ContentQuality{
		Score:              score,
		TextToHTMLRatio:    textToHTMLRatio,
		ParagraphCount:     paragraphCount,
		AvgParagraphLength: avgParagraphLength,
		HasHeaders:         hasHeaders,
		LinkDensity:        linkDensity,
		WordCount:          wordCount,
	}
}

// calculateOverallScore computes a 0-100 quality score
func calculateOverallScore(wordCount, paragraphCount, avgParagraphLength int,
	hasHeaders bool, textToHTMLRatio, linkDensity float64) int {

	score := 0

	// Word count scoring (0-25 points)
	if wordCount >= 500 {
		score += 25
	} else if wordCount >= 200 {
		score += 20
	} else if wordCount >= 100 {
		score += 15
	} else if wordCount >= 50 {
		score += 10
	}

	// Paragraph count scoring (0-20 points)
	if paragraphCount >= 5 {
		score += 20
	} else if paragraphCount >= 3 {
		score += 15
	} else if paragraphCount >= 2 {
		score += 10
	} else if paragraphCount >= 1 {
		score += 5
	}

	// Average paragraph length scoring (0-20 points)
	if avgParagraphLength >= 200 {
		score += 20
	} else if avgParagraphLength >= 100 {
		score += 15
	} else if avgParagraphLength >= 50 {
		score += 10
	} else if avgParagraphLength >= 20 {
		score += 5
	}

	// Structure scoring (0-15 points)
	if hasHeaders {
		score += 15
	}

	// Text-to-HTML ratio scoring (0-10 points)
	if textToHTMLRatio >= 0.3 {
		score += 10
	} else if textToHTMLRatio >= 0.2 {
		score += 7
	} else if textToHTMLRatio >= 0.1 {
		score += 5
	}

	// Link density penalty (0-10 points deducted)
	if linkDensity <= 5 {
		score += 10
	} else if linkDensity <= 10 {
		score += 5
	} else if linkDensity > 20 {
		score -= 10
	}

	// Ensure score is within bounds
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return score
}
