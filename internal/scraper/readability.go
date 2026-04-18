package scraper

import (
	"strings"
)

func ExtractMainText(html string) string {
	cleaned := StripScriptsAndStyles(html)
	cleaned = removeBetweenTags(cleaned, "nav")
	cleaned = removeBetweenTags(cleaned, "header")
	cleaned = removeBetweenTags(cleaned, "footer")
	cleaned = removeBetweenTags(cleaned, "aside")

	main := extractTagContent(cleaned, "main")
	if main == "" {
		main = extractTagContent(cleaned, "article")
	}
	if main == "" {
		main = extractTagContent(cleaned, "body")
	}
	if main == "" {
		main = cleaned
	}

	return HTMLToText(main)
}

func ExtractParagraphs(html string) []string {
	cleaned := StripScriptsAndStyles(html)
	var paragraphs []string

	parts := strings.Split(strings.ToLower(cleaned), "<p")
	for i := 1; i < len(parts); i++ {
		after := parts[i]
		gtIdx := strings.Index(after, ">")
		if gtIdx < 0 {
			continue
		}
		content := after[gtIdx+1:]
		endIdx := strings.Index(strings.ToLower(content), "</p>")
		if endIdx >= 0 {
			content = content[:endIdx]
		}
		text := strings.TrimSpace(StripAllTags(content))
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
	}
	return paragraphs
}

func extractTagContent(html, tag string) string {
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
		return html[contentStart:]
	}
	return html[contentStart : contentStart+end]
}
