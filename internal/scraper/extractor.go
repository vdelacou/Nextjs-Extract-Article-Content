package scraper

import (
	"strings"

	"extract-html-scraper/internal/models"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
	"github.com/microcosm-cc/bluemonday"
)

type ArticleExtractor struct {
	sanitizer     *bluemonday.Policy
	htmlSanitizer *bluemonday.Policy
}

func NewArticleExtractor() *ArticleExtractor {
	// Configure bluemonday for HTML sanitization
	policy := bluemonday.StrictPolicy()

	// Configure HTML sanitizer for preserving structure
	htmlPolicy := bluemonday.UGCPolicy()
	htmlPolicy.AllowElements("p", "br", "h1", "h2", "h3", "h4", "h5", "h6", "strong", "em", "blockquote", "ul", "ol", "li")

	return &ArticleExtractor{
		sanitizer:     policy,
		htmlSanitizer: htmlPolicy,
	}
}

// ExtractArticleWithOptions extracts content with configurable options
func (ae *ArticleExtractor) ExtractArticleWithOptions(html, baseURL string, options ExtractionOptions) models.ScrapeResponse {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return models.ScrapeResponse{
			Images: []string{},
		}
	}

	title := ae.extractTitle(doc)
	description := ae.extractDescription(doc)

	var content string
	if options.PreserveHTML {
		content = ae.extractContentAsHTML(doc)
	} else {
		content = ae.extractContent(doc)
	}

	// Extract images using the optimized image extractor
	imageExtractor := NewImageExtractor()
	images := imageExtractor.ExtractImagesFromHTML(html, baseURL)

	// Extract metadata if requested
	var metadata models.ScrapeResponse
	if options.IncludeMetadata {
		metadata = ae.extractMetadataFromReadability(html)
	}

	// Calculate content quality metrics
	quality := ScoreContentQuality(content, html)

	response := models.ScrapeResponse{
		Title:       title,
		Description: description,
		Content:     content,
		Images:      images,
		Quality: models.Quality{
			Score:              quality.Score,
			TextToHTMLRatio:    quality.TextToHTMLRatio,
			ParagraphCount:     quality.ParagraphCount,
			AvgParagraphLength: quality.AvgParagraphLength,
			HasHeaders:         quality.HasHeaders,
			LinkDensity:        quality.LinkDensity,
			WordCount:          quality.WordCount,
		},
	}

	// Add metadata fields if requested
	if options.IncludeMetadata {
		response.Author = metadata.Author
		response.PublishDate = metadata.PublishDate
		response.Excerpt = metadata.Excerpt
		response.ReadingTime = metadata.ReadingTime
		response.Language = metadata.Language
		response.TextLength = metadata.TextLength
	}

	return response
}

// ExtractArticle extracts title, description, content, and images from HTML (backward compatibility)
func (ae *ArticleExtractor) ExtractArticle(html, baseURL string) models.ScrapeResponse {
	return ae.ExtractArticleWithOptions(html, baseURL, DefaultExtractionOptions())
}

// extractContentAsHTML extracts content preserving HTML structure
func (ae *ArticleExtractor) extractContentAsHTML(doc *goquery.Document) string {
	// First, try to use readability algorithm for better content extraction
	html, err := doc.Html()
	if err == nil {
		// Parse with readability, passing URL for better context
		article, err := readability.FromReader(strings.NewReader(html), nil)
		if err == nil && article.Content != "" {
			// Sanitize HTML content while preserving structure
			return ae.htmlSanitizer.Sanitize(article.Content)
		}
	}

	// Fallback to original selector-based approach if readability fails
	return ae.extractContentFallbackAsHTML(doc)
}

// extractContentFallbackAsHTML provides HTML-based content extraction fallback
func (ae *ArticleExtractor) extractContentFallbackAsHTML(doc *goquery.Document) string {
	// Find the main content container
	contentElement := FindContentContainer(doc)

	// Get HTML content and sanitize it
	htmlContent, err := contentElement.Html()
	if err != nil {
		return ""
	}

	return ae.htmlSanitizer.Sanitize(htmlContent)
}

// sanitizeText sanitizes text content
func (ae *ArticleExtractor) sanitizeText(text string) string {
	if text == "" {
		return ""
	}

	// Use bluemonday to sanitize HTML if present
	sanitized := ae.sanitizer.Sanitize(text)

	// Additional cleanup using our helper
	return CleanWhitespace(sanitized)
}

// extractTitle extracts the page title with fallback strategies
func (ae *ArticleExtractor) extractTitle(doc *goquery.Document) string {
	// Try Open Graph title first
	if title := FindMetaTag(doc, OGTitle, ""); title != "" {
		return ae.sanitizeText(title)
	}

	// Try Twitter card title
	if title := FindMetaTag(doc, "", TwitterTitle); title != "" {
		return ae.sanitizeText(title)
	}

	// Try h1 tag
	var title string
	doc.Find("h1").First().Each(func(i int, s *goquery.Selection) {
		title = strings.TrimSpace(s.Text())
	})
	if title != "" {
		return ae.sanitizeText(title)
	}

	// Try title tag as last resort
	doc.Find("title").Each(func(i int, s *goquery.Selection) {
		title = strings.TrimSpace(s.Text())
	})

	return ae.sanitizeText(title)
}

// extractDescription extracts the page description with fallback strategies
func (ae *ArticleExtractor) extractDescription(doc *goquery.Document) string {
	// Try Open Graph description first
	if desc := FindMetaTag(doc, OGDescription, ""); desc != "" {
		return ae.sanitizeText(desc)
	}

	// Try Twitter card description
	if desc := FindMetaTag(doc, "", TwitterDesc); desc != "" {
		return ae.sanitizeText(desc)
	}

	// Try meta description
	if desc := FindMetaTag(doc, "", MetaDesc); desc != "" {
		return ae.sanitizeText(desc)
	}

	// Try to extract from first paragraph
	if desc := ExtractDescriptionFromParagraph(doc); desc != "" {
		return ae.sanitizeText(desc)
	}

	return ""
}

// extractContent extracts the main article content using readability algorithm
func (ae *ArticleExtractor) extractContent(doc *goquery.Document) string {
	// First, try to use readability algorithm for better content extraction
	html, err := doc.Html()
	if err == nil {
		// Parse with readability, passing URL for better context
		article, err := readability.FromReader(strings.NewReader(html), nil)
		if err == nil && article.Content != "" {
			// Convert readability's HTML content to structured text
			return ae.convertHTMLToStructuredText(article.Content)
		}
	}

	// Fallback to original selector-based approach if readability fails
	return ae.extractContentFallback(doc)
}

// convertHTMLToStructuredText converts HTML content to structured text
func (ae *ArticleExtractor) convertHTMLToStructuredText(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return ae.sanitizeText(htmlContent)
	}

	// Extract structured text
	content := ExtractTextFromElements(doc.Selection, TextElements)

	// If no structured content found, extract all text
	if content == "" {
		content = ExtractFallbackText(doc.Selection)
	}

	// Clean up whitespace and remove noise
	content = CleanTextContent(content)
	return ae.sanitizeText(content)
}

// extractContentFallback provides the original selector-based content extraction
func (ae *ArticleExtractor) extractContentFallback(doc *goquery.Document) string {
	// Find the main content container
	contentElement := FindContentContainer(doc)

	// Extract structured text from the container
	content := ExtractTextFromElements(contentElement, TextElements)

	// If no structured content found, extract all text
	if content == "" {
		content = ExtractFallbackText(contentElement)
	}

	// Clean up whitespace and remove noise
	content = CleanTextContent(content)
	return ae.sanitizeText(content)
}

// extractMetadataFromReadability extracts additional metadata using readability
func (ae *ArticleExtractor) extractMetadataFromReadability(html string) models.ScrapeResponse {
	article, err := readability.FromReader(strings.NewReader(html), nil)
	if err != nil {
		return models.ScrapeResponse{}
	}

	// Calculate reading time (average 200 words per minute, but we'll use character count)
	readingTime := 0
	if article.Length > 0 {
		// Estimate reading time based on character count (roughly 5 chars per word, 200 words per minute)
		readingTime = int(article.Length / 1000) // characters / 1000 chars per minute
		if readingTime < 1 {
			readingTime = 1
		}
	}

	// Convert publish date to string
	publishDate := ""
	if article.PublishedTime != nil {
		publishDate = article.PublishedTime.Format("2006-01-02T15:04:05Z")
	}

	return models.ScrapeResponse{
		Author:      article.Byline,
		PublishDate: publishDate,
		Excerpt:     article.Excerpt,
		ReadingTime: readingTime,
		Language:    article.Language,
		TextLength:  article.Length,
	}
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
