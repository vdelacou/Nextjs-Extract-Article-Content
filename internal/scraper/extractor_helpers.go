// Package scraper provides helper functions for article content extraction.
package scraper

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// FindMetaTag searches for a meta tag with the given property or name
func FindMetaTag(doc *goquery.Document, property, name string) string {
	var value string

	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if value != "" {
			return // Already found
		}

		// Check property attribute
		if property != "" {
			if prop, exists := s.Attr("property"); exists && prop == property {
				if content, exists := s.Attr("content"); exists {
					value = strings.TrimSpace(content)
					return
				}
			}
		}

		// Check name attribute
		if name != "" {
			if n, exists := s.Attr("name"); exists && n == name {
				if content, exists := s.Attr("content"); exists {
					value = strings.TrimSpace(content)
					return
				}
			}
		}
	})

	return value
}

// ExtractTextFromElements extracts text content preserving structure from HTML elements
func ExtractTextFromElements(selection *goquery.Selection, elements string) string {
	var content strings.Builder

	selection.Find(elements).Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}

		tagName := goquery.NodeName(s)
		switch tagName {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			if content.Len() > 0 {
				content.WriteString(DoubleNewline)
			}
			content.WriteString(text)
			content.WriteString(SingleNewline)
		case "p", "li", "blockquote":
			if content.Len() > 0 {
				content.WriteString(SingleNewline)
			}
			content.WriteString(text)
		}
	})

	return content.String()
}

// ExtractFallbackText extracts all text content when structured extraction fails
func ExtractFallbackText(selection *goquery.Selection) string {
	// Remove non-content elements
	selection.Find(NonContentTags).Remove()

	// Extract all text
	text := strings.TrimSpace(selection.Text())
	return text
}

// FindContentContainer finds the main content container using common selectors
func FindContentContainer(doc *goquery.Document) *goquery.Selection {
	selectors := strings.Split(ContentSelectors, ", ")

	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		if doc.Find(selector).Length() > 0 {
			return doc.Find(selector).First()
		}
	}

	// Fallback to body
	return doc.Find("body")
}

// ExtractDescriptionFromParagraph extracts description from first suitable paragraph
func ExtractDescriptionFromParagraph(doc *goquery.Document) string {
	var description string

	doc.Find("p").First().Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if len(text) > MinDescriptionLen && len(text) < MaxDescriptionLen {
			description = text
		}
	})

	return description
}
