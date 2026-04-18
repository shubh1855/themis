package scraper

import (
	"strings"
)

type SelectorResult struct {
	Selector string   `json:"selector"`
	Matches  []string `json:"matches"`
}

func ExtractBySelector(html string, selectors []string) []SelectorResult {
	var results []SelectorResult

	for _, sel := range selectors {
		var matches []string

		switch {
		case strings.HasPrefix(sel, "."):
			className := sel[1:]
			matches = extractByClass(html, className)
		case strings.HasPrefix(sel, "#"):
			id := sel[1:]
			matches = extractByID(html, id)
		default:
			matches = extractByTag(html, sel)
		}

		results = append(results, SelectorResult{
			Selector: sel,
			Matches:  matches,
		})
	}

	return results
}

func extractByTag(html, tag string) []string {
	var matches []string
	lower := strings.ToLower(html)
	openTag := "<" + strings.ToLower(tag)
	closeTag := "</" + strings.ToLower(tag) + ">"

	offset := 0
	for {
		idx := strings.Index(lower[offset:], openTag)
		if idx < 0 {
			break
		}
		start := offset + idx
		gt := strings.Index(lower[start:], ">")
		if gt < 0 {
			break
		}
		contentStart := start + gt + 1
		end := strings.Index(lower[contentStart:], closeTag)
		if end < 0 {
			offset = contentStart
			continue
		}
		text := strings.TrimSpace(StripAllTags(html[contentStart : contentStart+end]))
		if text != "" {
			matches = append(matches, text)
		}
		offset = contentStart + end + len(closeTag)
	}
	return matches
}

func extractByClass(html, className string) []string {
	var matches []string
	searchStr := "class=\"" + className + "\""
	lower := strings.ToLower(html)
	lowerSearch := strings.ToLower(searchStr)

	offset := 0
	for {
		idx := strings.Index(lower[offset:], lowerSearch)
		if idx < 0 {
			break
		}
		pos := offset + idx
		tagStart := strings.LastIndex(lower[:pos], "<")
		if tagStart < 0 {
			offset = pos + len(lowerSearch)
			continue
		}
		gt := strings.Index(lower[pos:], ">")
		if gt < 0 {
			break
		}
		contentStart := pos + gt + 1
		tagName := extractTagName(lower[tagStart:])
		closeTag := "</" + tagName + ">"
		end := strings.Index(lower[contentStart:], closeTag)
		if end < 0 {
			offset = contentStart
			continue
		}
		text := strings.TrimSpace(StripAllTags(html[contentStart : contentStart+end]))
		if text != "" {
			matches = append(matches, text)
		}
		offset = contentStart + end + len(closeTag)
	}
	return matches
}

func extractByID(html, id string) []string {
	searchStr := "id=\"" + id + "\""
	lower := strings.ToLower(html)
	lowerSearch := strings.ToLower(searchStr)

	idx := strings.Index(lower, lowerSearch)
	if idx < 0 {
		return nil
	}
	gt := strings.Index(lower[idx:], ">")
	if gt < 0 {
		return nil
	}
	contentStart := idx + gt + 1
	tagStart := strings.LastIndex(lower[:idx], "<")
	if tagStart < 0 {
		return nil
	}
	tagName := extractTagName(lower[tagStart:])
	closeTag := "</" + tagName + ">"
	end := strings.Index(lower[contentStart:], closeTag)
	if end < 0 {
		return nil
	}
	text := strings.TrimSpace(StripAllTags(html[contentStart : contentStart+end]))
	if text != "" {
		return []string{text}
	}
	return nil
}

func extractTagName(s string) string {
	if len(s) < 2 {
		return ""
	}
	s = s[1:]
	end := strings.IndexAny(s, " \t\n\r/>")
	if end < 0 {
		return s
	}
	return s[:end]
}
