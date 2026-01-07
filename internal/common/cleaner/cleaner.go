package cleaner

import (
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// Cleaner sanitizes HTML content using Bluemonday
type Cleaner struct {
	policy *bluemonday.Policy
}

// NewCleaner creates a new HTML cleaner with a safe policy
func NewCleaner() *Cleaner {
	// Create a policy that allows basic formatting but strips dangerous elements
	policy := bluemonday.NewPolicy()

	// Allow basic text formatting
	policy.AllowElements("p", "br", "div", "span")
	policy.AllowElements("strong", "b", "em", "i", "u")
	policy.AllowElements("ul", "ol", "li")
	policy.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")

	// Allow links but strip javascript:
	policy.AllowAttrs("href").OnElements("a")
	policy.AllowRelativeURLs(true)
	policy.RequireParseableURLs(true)
	policy.AllowURLSchemes("http", "https", "mailto")

	return &Cleaner{policy: policy}
}

// NewStrictCleaner creates a cleaner that strips ALL HTML
func NewStrictCleaner() *Cleaner {
	return &Cleaner{policy: bluemonday.StrictPolicy()}
}

// Clean sanitizes HTML content
func (c *Cleaner) Clean(html string) string {
	return c.policy.Sanitize(html)
}

// CleanToText removes all HTML and returns plain text
func (c *Cleaner) CleanToText(html string) string {
	strict := bluemonday.StrictPolicy()
	text := strict.Sanitize(html)

	// Clean up whitespace
	text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	text = strings.TrimSpace(text)

	return text
}

// CleanMap sanitizes all string values in a map
func (c *Cleaner) CleanMap(data map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range data {
		switch val := v.(type) {
		case string:
			result[k] = c.Clean(val)
		case map[string]any:
			result[k] = c.CleanMap(val)
		default:
			result[k] = v
		}
	}
	return result
}
