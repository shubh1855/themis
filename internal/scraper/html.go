package scraper

import (
	"strings"
)

// StripScriptsAndStyles removes <script> and <style> blocks from HTML.
func StripScriptsAndStyles(html string) string {
	html = removeBetweenTags(html, "script")
	html = removeBetweenTags(html, "style")
	html = removeBetweenTags(html, "noscript")
	return html
}

// StripAllTags removes all HTML tags, leaving only text content.
func StripAllTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			result.WriteRune(' ')
		case !inTag:
			result.WriteRune(r)
		}
	}
	return collapseWhitespace(result.String())
}

// HTMLToText converts HTML to readable text by stripping tags and normalizing whitespace.
func HTMLToText(html string) string {
	cleaned := StripScriptsAndStyles(html)
	// Convert block elements to newlines
	for _, tag := range []string{"</p>", "</div>", "</li>", "</h1>", "</h2>", "</h3>", "</h4>", "</h5>", "</h6>", "<br>", "<br/>", "<br />"} {
		cleaned = strings.ReplaceAll(cleaned, tag, "\n")
	}
	text := StripAllTags(cleaned)
	return strings.TrimSpace(text)
}

func removeBetweenTags(html, tag string) string {
	lower := strings.ToLower(html)
	openTag := "<" + tag
	closeTag := "</" + tag + ">"

	for {
		start := strings.Index(lower, openTag)
		if start < 0 {
			break
		}
		end := strings.Index(lower[start:], closeTag)
		if end < 0 {
			// Remove to end
			html = html[:start]
			lower = lower[:start]
			break
		}
		end = start + end + len(closeTag)
		html = html[:start] + html[end:]
		lower = lower[:start] + lower[end:]
	}
	return html
}

func collapseWhitespace(s string) string {
	// Collapse runs of spaces/tabs into single spaces
	var result strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				result.WriteRune(' ')
				prevSpace = true
			}
		} else {
			result.WriteRune(r)
			prevSpace = false
		}
	}
	return result.String()
}
