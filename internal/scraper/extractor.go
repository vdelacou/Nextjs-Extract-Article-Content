package scraper

import (
	"strings"

	"extract-html-scraper/internal/models"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
)

type ArticleExtractor struct {
	sanitizer *bluemonday.Policy
}

func NewArticleExtractor() *ArticleExtractor {
	// Configure bluemonday for HTML sanitization
	policy := bluemonday.StrictPolicy()

	return &ArticleExtractor{
		sanitizer: policy,
	}
}

// ExtractArticle extracts title, description, content, and images from HTML
func (ae *ArticleExtractor) ExtractArticle(html, baseURL string) models.ScrapeResponse {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return models.ScrapeResponse{
			Images: []string{},
		}
	}

	// Extract components concurrently would be ideal, but for simplicity we'll do sequentially
	// since they're already quite fast with goquery

	title := ae.extractTitle(doc)
	description := ae.extractDescription(doc)
	content := ae.extractContent(doc)

	// Extract images using the optimized image extractor
	imageExtractor := NewImageExtractor()
	images := imageExtractor.ExtractImagesFromHTML(html, baseURL)

	return models.ScrapeResponse{
		Title:       title,
		Description: description,
		Content:     content,
		Images:      images,
	}
}

// extractTitle extracts the page title with fallback strategies
func (ae *ArticleExtractor) extractTitle(doc *goquery.Document) string {
	var title string

	// Try Open Graph title first
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if property, exists := s.Attr("property"); exists && property == "og:title" {
			if content, exists := s.Attr("content"); exists {
				title = strings.TrimSpace(content)
			}
		}
	})

	// Try Twitter card title
	if title == "" {
		doc.Find("meta").Each(func(i int, s *goquery.Selection) {
			if name, exists := s.Attr("name"); exists && name == "twitter:title" {
				if content, exists := s.Attr("content"); exists {
					title = strings.TrimSpace(content)
				}
			}
		})
	}

	// Try h1 tag
	if title == "" {
		doc.Find("h1").First().Each(func(i int, s *goquery.Selection) {
			title = strings.TrimSpace(s.Text())
		})
	}

	// Try title tag as last resort
	if title == "" {
		doc.Find("title").Each(func(i int, s *goquery.Selection) {
			title = strings.TrimSpace(s.Text())
		})
	}

	return ae.sanitizeText(title)
}

// extractDescription extracts the page description with fallback strategies
func (ae *ArticleExtractor) extractDescription(doc *goquery.Document) string {
	var description string

	// Try Open Graph description first
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if property, exists := s.Attr("property"); exists && property == "og:description" {
			if content, exists := s.Attr("content"); exists {
				description = strings.TrimSpace(content)
			}
		}
	})

	// Try Twitter card description
	if description == "" {
		doc.Find("meta").Each(func(i int, s *goquery.Selection) {
			if name, exists := s.Attr("name"); exists && name == "twitter:description" {
				if content, exists := s.Attr("content"); exists {
					description = strings.TrimSpace(content)
				}
			}
		})
	}

	// Try meta description
	if description == "" {
		doc.Find("meta").Each(func(i int, s *goquery.Selection) {
			if name, exists := s.Attr("name"); exists && name == "description" {
				if content, exists := s.Attr("content"); exists {
					description = strings.TrimSpace(content)
				}
			}
		})
	}

	// Try to extract from first paragraph
	if description == "" {
		doc.Find("p").First().Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if len(text) > 50 && len(text) < 300 {
				description = text
			}
		})
	}

	return ae.sanitizeText(description)
}

// extractContent extracts the main article content
func (ae *ArticleExtractor) extractContent(doc *goquery.Document) string {
	var content strings.Builder

	// Try to find article or main content
	contentSelectors := []string{
		"article",
		"main",
		"[role='main']",
		".content",
		".post-content",
		".entry-content",
		".article-content",
		".story-content",
	}

	var contentElement *goquery.Selection
	for _, selector := range contentSelectors {
		if doc.Find(selector).Length() > 0 {
			contentElement = doc.Find(selector).First()
			break
		}
	}

	// If no specific content container found, try body
	if contentElement == nil {
		contentElement = doc.Find("body")
	}

	// Extract text content, preserving some structure
	contentElement.Find("p, h1, h2, h3, h4, h5, h6, li, blockquote").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			// Add some basic structure
			tagName := goquery.NodeName(s)
			switch tagName {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				if content.Len() > 0 {
					content.WriteString("\n\n")
				}
				content.WriteString(text)
				content.WriteString("\n")
			case "p", "li", "blockquote":
				if content.Len() > 0 {
					content.WriteString("\n")
				}
				content.WriteString(text)
			}
		}
	})

	// If no structured content found, extract all text
	if content.Len() == 0 {
		contentElement.Find("*").Each(func(i int, s *goquery.Selection) {
			// Skip script, style, and other non-content elements
			tagName := goquery.NodeName(s)
			if tagName == "script" || tagName == "style" || tagName == "nav" || tagName == "header" || tagName == "footer" {
				s.Remove()
				return
			}
		})

		text := strings.TrimSpace(contentElement.Text())
		content.WriteString(text)
	}

	result := content.String()

	// Clean up whitespace
	result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	result = strings.TrimSpace(result)

	return ae.sanitizeText(result)
}

// sanitizeText sanitizes text content
func (ae *ArticleExtractor) sanitizeText(text string) string {
	if text == "" {
		return ""
	}

	// Use bluemonday to sanitize HTML if present
	sanitized := ae.sanitizer.Sanitize(text)

	// Additional cleanup
	sanitized = strings.TrimSpace(sanitized)

	// Remove excessive whitespace
	sanitized = strings.ReplaceAll(sanitized, "  ", " ")
	sanitized = strings.ReplaceAll(sanitized, "\n\n\n", "\n\n")

	return sanitized
}

// ExtractArticleSimple is a simpler version for basic content extraction
func (ae *ArticleExtractor) ExtractArticleSimple(html, baseURL string) models.ScrapeResponse {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return models.ScrapeResponse{
			Images: []string{},
		}
	}

	// Simple title extraction
	title := ""
	doc.Find("title").Each(func(i int, s *goquery.Selection) {
		title = strings.TrimSpace(s.Text())
	})

	// Simple description extraction
	description := ""
	doc.Find("meta[name='description']").Each(func(i int, s *goquery.Selection) {
		if content, exists := s.Attr("content"); exists {
			description = strings.TrimSpace(content)
		}
	})

	// Simple content extraction - just get all text
	content := ""
	doc.Find("body").Each(func(i int, s *goquery.Selection) {
		// Remove script and style elements
		s.Find("script, style, nav, header, footer").Remove()
		content = strings.TrimSpace(s.Text())
	})

	// Extract images
	imageExtractor := NewImageExtractor()
	images := imageExtractor.ExtractImagesFromHTML(html, baseURL)

	return models.ScrapeResponse{
		Title:       ae.sanitizeText(title),
		Description: ae.sanitizeText(description),
		Content:     ae.sanitizeText(content),
		Images:      images,
	}
}
