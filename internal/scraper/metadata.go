package scraper

import (
	"strings"
)

// Metadata holds extracted page metadata.
type Metadata struct {
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
	Links       []string          `json:"links,omitempty"`
	Headings    []string          `json:"headings,omitempty"`
}

// ExtractMetadata extracts title, meta tags, links, and headings from HTML.
func ExtractMetadata(html string) *Metadata {
	m := &Metadata{
		Meta: make(map[string]string),
	}

	// Extract title
	m.Title = extractSimpleTag(html, "title")

	// Extract meta tags
	extractMetaTags(html, m)

	// Extract links
	m.Links = extractLinks(html)

	// Extract headings
	m.Headings = extractHeadings(html)

	return m
}

func extractSimpleTag(html, tag string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<"+tag)
	if start < 0 {
		return ""
	}
	gt := strings.Index(lower[start:], ">")
	if gt < 0 {
		return ""
	}
	contentStart := start + gt + 1
	end := strings.Index(lower[contentStart:], "</"+tag+">")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(html[contentStart : contentStart+end])
}

func extractMetaTags(html string, m *Metadata) {
	lower := strings.ToLower(html)
	offset := 0
	for {
		idx := strings.Index(lower[offset:], "<meta ")
		if idx < 0 {
			break
		}
		pos := offset + idx
		end := strings.Index(lower[pos:], ">")
		if end < 0 {
			break
		}
		tag := html[pos : pos+end+1]
		lowerTag := strings.ToLower(tag)

		name := extractAttr(lowerTag, "name")
		if name == "" {
			name = extractAttr(lowerTag, "property")
		}
		content := extractAttr(tag, "content")

		if name != "" && content != "" {
			m.Meta[name] = content
			if strings.EqualFold(name, "description") || strings.EqualFold(name, "og:description") {
				m.Description = content
			}
		}

		offset = pos + end + 1
	}
}

func extractLinks(html string) []string {
	var links []string
	lower := strings.ToLower(html)
	offset := 0
	for {
		idx := strings.Index(lower[offset:], "href=\"")
		if idx < 0 {
			break
		}
		pos := offset + idx + 6
		end := strings.Index(html[pos:], "\"")
		if end < 0 {
			break
		}
		href := html[pos : pos+end]
		if strings.HasPrefix(href, "http") {
			links = append(links, href)
		}
		offset = pos + end + 1
	}
	return dedup(links)
}

func extractHeadings(html string) []string {
	var headings []string
	for _, tag := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		for _, h := range extractByTag(html, tag) {
			headings = append(headings, h)
		}
	}
	return headings
}

func extractAttr(tag, attr string) string {
	search := attr + "=\""
	idx := strings.Index(strings.ToLower(tag), search)
	if idx < 0 {
		return ""
	}
	start := idx + len(search)
	end := strings.Index(tag[start:], "\"")
	if end < 0 {
		return ""
	}
	return tag[start : start+end]
}

func dedup(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
