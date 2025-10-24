package scraper

// ExtractionOptions defines configurable options for article extraction
type ExtractionOptions struct {
	PreserveHTML      bool   `json:"preserveHtml"`
	IncludeMetadata   bool   `json:"includeMetadata"`
	MinTextLength     int    `json:"minTextLength"`
	MinParagraphChars int    `json:"minParagraphChars"`
	RemoveComments    bool   `json:"removeComments"`
	OutputFormat      string `json:"outputFormat"` // "text", "markdown", "html"
}

// DefaultExtractionOptions returns sensible defaults for extraction
func DefaultExtractionOptions() ExtractionOptions {
	return ExtractionOptions{
		PreserveHTML:      false,
		IncludeMetadata:   true,
		MinTextLength:     100,
		MinParagraphChars: 40,
		RemoveComments:    true,
		OutputFormat:      "text",
	}
}

// HTMLExtractionOptions returns options for HTML output
func HTMLExtractionOptions() ExtractionOptions {
	opts := DefaultExtractionOptions()
	opts.PreserveHTML = true
	opts.OutputFormat = "html"
	return opts
}

// MarkdownExtractionOptions returns options for markdown output
func MarkdownExtractionOptions() ExtractionOptions {
	opts := DefaultExtractionOptions()
	opts.OutputFormat = "markdown"
	return opts
}
