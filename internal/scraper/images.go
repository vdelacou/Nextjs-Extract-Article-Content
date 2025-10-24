package scraper

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"extract-html-scraper/internal/config"
	"extract-html-scraper/internal/models"

	"github.com/PuerkitoBio/goquery"
)

type ImageExtractor struct {
	config  models.ImageConfig
	regexes map[string]*regexp.Regexp
}

func NewImageExtractor() *ImageExtractor {
	cfg := config.DefaultImageConfig()
	regexes := config.CompileRegexes()

	return &ImageExtractor{
		config: models.ImageConfig{
			MinShortSide:   cfg.MinShortSide,
			MinArea:        cfg.MinArea,
			MinAspect:      cfg.MinAspect,
			MaxAspect:      cfg.MaxAspect,
			RatioWhitelist: cfg.RatioWhitelist,
			RatioTol:       cfg.RatioTol,
			AdSizes:        cfg.AdSizes,
			BadHintRegex:   cfg.BadHintRegex,
		},
		regexes: regexes,
	}
}

// ExtractImagesFromHTML extracts and scores images from HTML content
func (ie *ImageExtractor) ExtractImagesFromHTML(html, baseURL string) []string {
	// Parse HTML once with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return []string{}
	}

	// Extract candidates concurrently
	candidatesChan := make(chan []models.ImageCandidate, 2)
	var wg sync.WaitGroup

	// Extract og:image concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		ogImage := ie.extractOgImage(doc, baseURL)
		if ogImage != nil {
			candidatesChan <- []models.ImageCandidate{*ogImage}
		} else {
			candidatesChan <- []models.ImageCandidate{}
		}
	}()

	// Extract img tags concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		imgCandidates := ie.extractImgTags(doc, baseURL)
		candidatesChan <- imgCandidates
	}()

	// Wait for both extractions to complete
	go func() {
		wg.Wait()
		close(candidatesChan)
	}()

	// Collect all candidates
	var allCandidates []models.ImageCandidate
	for candidates := range candidatesChan {
		allCandidates = append(allCandidates, candidates...)
	}

	// Filter and score candidates
	filtered := ie.filterAndScoreCandidates(allCandidates)

	// Sort by score and area
	ie.sortCandidates(filtered)

	// Return top 3 unique URLs
	return ie.getTopImages(filtered, 3)
}

// extractOgImage extracts Open Graph image metadata
func (ie *ImageExtractor) extractOgImage(doc *goquery.Document, baseURL string) *models.ImageCandidate {
	var ogImageURL string
	var width, height int

	// Find og:image meta tag
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		property, exists := s.Attr("property")
		if !exists {
			return
		}

		switch property {
		case "og:image", "og:image:secure_url":
			if content, exists := s.Attr("content"); exists {
				ogImageURL = content
			}
		case "og:image:width":
			if content, exists := s.Attr("content"); exists {
				if w, err := strconv.Atoi(content); err == nil {
					width = w
				}
			}
		case "og:image:height":
			if content, exists := s.Attr("content"); exists {
				if h, err := strconv.Atoi(content); err == nil {
					height = h
				}
			}
		}
	})

	if ogImageURL == "" {
		return nil
	}

	// Convert to absolute URL
	absURL, err := ie.toAbsoluteURL(ogImageURL, baseURL)
	if err != nil {
		return nil
	}

	// Check if it's an image file
	if !ie.regexes["imageExt"].MatchString(absURL) {
		return nil
	}

	// If dimensions not found in meta tags, try to extract from URL
	if width == 0 || height == 0 {
		urlWidth, urlHeight := ie.parseDimensionsFromURL(absURL)
		if width == 0 {
			width = urlWidth
		}
		if height == 0 {
			height = urlHeight
		}
	}

	return &models.ImageCandidate{
		URL:       absURL,
		Width:     width,
		Height:    height,
		InArticle: true, // og:image is considered in-article
		BadHint:   false,
		Source:    "og",
	}
}

// extractImgTags extracts all img tags from the document
func (ie *ImageExtractor) extractImgTags(doc *goquery.Document, baseURL string) []models.ImageCandidate {
	var candidates []models.ImageCandidate

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		candidate := ie.extractImgTag(s, baseURL)
		if candidate != nil {
			candidates = append(candidates, *candidate)
		}
	})

	return candidates
}

// extractImgTag extracts a single img tag
func (ie *ImageExtractor) extractImgTag(s *goquery.Selection, baseURL string) *models.ImageCandidate {
	// Get src attribute or data-src variants
	src := ""
	if srcAttr, exists := s.Attr("src"); exists {
		src = srcAttr
	} else if dataSrc, exists := s.Attr("data-src"); exists {
		src = dataSrc
	} else if dataOriginal, exists := s.Attr("data-original"); exists {
		src = dataOriginal
	} else if dataLazySrc, exists := s.Attr("data-lazy-src"); exists {
		src = dataLazySrc
	}

	// Try srcset if no src found
	if src == "" {
		if srcset, exists := s.Attr("srcset"); exists {
			src = ie.pickFromSrcset(srcset)
		}
	}

	if src == "" {
		return nil
	}

	// Convert to absolute URL
	absURL, err := ie.toAbsoluteURL(src, baseURL)
	if err != nil {
		return nil
	}

	// Check if it's an image file
	if !ie.regexes["imageExt"].MatchString(absURL) {
		return nil
	}

	// Extract dimensions
	width, height := ie.extractDimensions(s)

	// If dimensions not found in attributes, try URL
	if width == 0 || height == 0 {
		urlWidth, urlHeight := ie.parseDimensionsFromURL(absURL)
		if width == 0 {
			width = urlWidth
		}
		if height == 0 {
			height = urlHeight
		}
	}

	// Check if in article scope
	inArticle := ie.isInArticleScope(s)

	// Check for bad hints
	badHint := ie.hasBadHint(s, absURL)

	return &models.ImageCandidate{
		URL:       absURL,
		Width:     width,
		Height:    height,
		InArticle: inArticle,
		BadHint:   badHint,
		Source:    "img",
	}
}

// extractDimensions extracts width and height from img tag
func (ie *ImageExtractor) extractDimensions(s *goquery.Selection) (int, int) {
	width := 0
	height := 0

	// Try width attribute
	if wAttr, exists := s.Attr("width"); exists {
		if w, err := strconv.Atoi(strings.TrimSpace(wAttr)); err == nil {
			width = w
		}
	}

	// Try height attribute
	if hAttr, exists := s.Attr("height"); exists {
		if h, err := strconv.Atoi(strings.TrimSpace(hAttr)); err == nil {
			height = h
		}
	}

	// Try style attribute
	if style, exists := s.Attr("style"); exists {
		widthMatch := ie.regexes["widthStyle"].FindStringSubmatch(style)
		if len(widthMatch) > 1 {
			if w, err := strconv.ParseFloat(widthMatch[1], 64); err == nil {
				width = int(w)
			}
		}

		heightMatch := ie.regexes["heightStyle"].FindStringSubmatch(style)
		if len(heightMatch) > 1 {
			if h, err := strconv.ParseFloat(heightMatch[1], 64); err == nil {
				height = int(h)
			}
		}
	}

	return width, height
}

// parseDimensionsFromURL extracts dimensions from URL patterns
func (ie *ImageExtractor) parseDimensionsFromURL(url string) (int, int) {
	// Try pattern like 300x400
	matches := ie.regexes["dimensionsFromUrl"].FindStringSubmatch(url)
	if len(matches) > 2 {
		if w, err := strconv.Atoi(matches[1]); err == nil {
			if h, err := strconv.Atoi(matches[2]); err == nil {
				return w, h
			}
		}
	}

	// Try separate width and height parameters
	widthMatch := ie.regexes["widthFromUrl"].FindStringSubmatch(url)
	heightMatch := ie.regexes["heightFromUrl"].FindStringSubmatch(url)

	width := 0
	height := 0

	if len(widthMatch) > 1 {
		if w, err := strconv.Atoi(widthMatch[1]); err == nil {
			width = w
		}
	}

	if len(heightMatch) > 1 {
		if h, err := strconv.Atoi(heightMatch[1]); err == nil {
			height = h
		}
	}

	return width, height
}

// pickFromSrcset selects the best image from srcset
func (ie *ImageExtractor) pickFromSrcset(srcset string) string {
	items := strings.Split(srcset, ",")
	var candidates []struct {
		url string
		w   int
	}

	for _, item := range items {
		item = strings.TrimSpace(item)
		matches := ie.regexes["srcsetItem"].FindStringSubmatch(item)
		if len(matches) > 2 {
			if w, err := strconv.Atoi(matches[2]); err == nil {
				candidates = append(candidates, struct {
					url string
					w   int
				}{matches[1], w})
			}
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// Find closest to 1000px width, preferring larger images
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		candidateDiff := absInt(candidate.w - 1000)
		bestDiff := absInt(best.w - 1000)
		if candidateDiff < bestDiff ||
			(candidateDiff == bestDiff && candidate.w > best.w) {
			best = candidate
		}
	}

	return best.url
}

// isInArticleScope checks if the img tag is within article or main tags
func (ie *ImageExtractor) isInArticleScope(s *goquery.Selection) bool {
	// Check if any parent is article or main
	return s.ParentsFiltered("article, main").Length() > 0
}

// hasBadHint checks if the image has bad hints (ads, icons, etc.)
func (ie *ImageExtractor) hasBadHint(s *goquery.Selection, url string) bool {
	// Check URL for bad patterns
	if ie.regexes["badHint"].MatchString(url) {
		return true
	}

	// Check img tag attributes and classes
	html, _ := s.Html()
	return ie.regexes["badHint"].MatchString(html)
}

// filterAndScoreCandidates filters and scores image candidates
func (ie *ImageExtractor) filterAndScoreCandidates(candidates []models.ImageCandidate) []models.ImageCandidate {
	var filtered []models.ImageCandidate

	for _, c := range candidates {
		if !ie.passesFilters(c) {
			continue
		}

		// Calculate score
		c.Score = ie.calculateScore(c)
		c.Area = c.Width * c.Height
		filtered = append(filtered, c)
	}

	return filtered
}

// passesFilters checks if a candidate passes all filters
func (ie *ImageExtractor) passesFilters(c models.ImageCandidate) bool {
	if c.Width > 0 && c.Height > 0 {
		shortSide := min(c.Width, c.Height)
		area := c.Width * c.Height

		// Size filters
		if shortSide < ie.config.MinShortSide {
			return false
		}
		if area < ie.config.MinArea {
			return false
		}

		// Aspect ratio filter
		if !ie.hasGoodAspectRatio(c.Width, c.Height) {
			return false
		}

		// Ad size filter
		if ie.isAdSize(c.Width, c.Height) {
			return false
		}

		// Bad hint filter with exceptions
		if c.BadHint && !(shortSide >= 400 && area >= 300000) {
			return false
		}
	} else if c.BadHint {
		return false
	}

	return true
}

// hasGoodAspectRatio checks if the aspect ratio is acceptable
func (ie *ImageExtractor) hasGoodAspectRatio(width, height int) bool {
	if width == 0 || height == 0 {
		return false
	}

	aspect := float64(width) / float64(height)

	// Check if within general bounds
	if aspect >= ie.config.MinAspect && aspect <= ie.config.MaxAspect {
		return true
	}

	// Check whitelist ratios
	for _, ratio := range ie.config.RatioWhitelist {
		if abs(aspect-ratio) <= ie.config.RatioTol {
			return true
		}
	}

	return false
}

// isAdSize checks if dimensions match common ad sizes
func (ie *ImageExtractor) isAdSize(width, height int) bool {
	if width == 0 || height == 0 {
		return false
	}

	sizeKey := fmt.Sprintf("%dx%d", width, height)
	return ie.config.AdSizes[sizeKey]
}

// calculateScore calculates the score for a candidate
func (ie *ImageExtractor) calculateScore(c models.ImageCandidate) float64 {
	score := 0.0
	area := float64(c.Width * c.Height)

	// Article boost
	if c.InArticle {
		score += 2.0
	}

	// OG image boost
	if c.Source == "og" {
		score += 1.0
	}

	// Aspect ratio bonus
	if c.Width > 0 && c.Height > 0 {
		aspect := float64(c.Width) / float64(c.Height)
		for _, ratio := range ie.config.RatioWhitelist {
			if abs(aspect-ratio) <= ie.config.RatioTol {
				score += 1.0
				break
			}
		}
	}

	// Area bonus (logarithmic)
	if area > 0 {
		score += log10(max(1, area))
	}

	return score
}

// sortCandidates sorts candidates by score and area
func (ie *ImageExtractor) sortCandidates(candidates []models.ImageCandidate) {
	// Simple bubble sort for small arrays
	n := len(candidates)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if candidates[j].Score < candidates[j+1].Score ||
				(candidates[j].Score == candidates[j+1].Score && candidates[j].Area < candidates[j+1].Area) {
				candidates[j], candidates[j+1] = candidates[j+1], candidates[j]
			}
		}
	}
}

// getTopImages returns the top N unique image URLs
func (ie *ImageExtractor) getTopImages(candidates []models.ImageCandidate, limit int) []string {
	seen := make(map[string]bool)
	var result []string

	for _, c := range candidates {
		if !seen[c.URL] {
			seen[c.URL] = true
			result = append(result, c.URL)
			if len(result) >= limit {
				break
			}
		}
	}

	return result
}

// toAbsoluteURL converts a relative URL to absolute
func (ie *ImageExtractor) toAbsoluteURL(relativeURL, baseURL string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	rel, err := url.Parse(relativeURL)
	if err != nil {
		return "", err
	}

	abs := base.ResolveReference(rel)
	return abs.String(), nil
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func log10(x float64) float64 {
	// Simple log10 approximation for scoring
	if x <= 0 {
		return 0
	}

	// Count digits
	count := 0
	for x >= 10 {
		x /= 10
		count++
	}

	return float64(count)
}
