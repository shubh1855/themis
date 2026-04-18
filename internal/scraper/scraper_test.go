package scraper_test

import (
	"strings"
	"testing"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/scraper"
)

const testHTML = `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
	<meta name="description" content="A test page for scraping">
	<meta property="og:title" content="OG Test Page">
</head>
<body>
	<nav><a href="/home">Home</a></nav>
	<header><h1>Welcome</h1></header>
	<main>
		<article>
			<h2>Article Title</h2>
			<p>This is the first paragraph with some content.</p>
			<p>This is the second paragraph.</p>
			<script>alert('bad')</script>
			<style>.hidden { display: none; }</style>
		</article>
	</main>
	<footer>Copyright 2024</footer>
</body>
</html>`

func TestStripScriptsAndStyles(t *testing.T) {
	result := scraper.StripScriptsAndStyles(testHTML)

	if strings.Contains(result, "alert") {
		t.Error("script content should be removed")
	}
	if strings.Contains(result, "display: none") {
		t.Error("style content should be removed")
	}
	if !strings.Contains(result, "Article Title") {
		t.Error("regular content should be preserved")
	}
}

func TestHTMLToText(t *testing.T) {
	result := scraper.HTMLToText(testHTML)

	if strings.Contains(result, "<") || strings.Contains(result, ">") {
		t.Error("HTML tags should be stripped")
	}
	if !strings.Contains(result, "Article Title") {
		t.Error("text content should be preserved")
	}
}

func TestExtractMainText(t *testing.T) {
	result := scraper.ExtractMainText(testHTML)

	if strings.Contains(result, "alert") {
		t.Error("script content should not appear in main text")
	}
	if strings.Contains(result, "Copyright") {
		t.Error("footer content should be removed")
	}
	if !strings.Contains(result, "first paragraph") {
		t.Error("main content should be present")
	}
}

func TestExtractMetadata(t *testing.T) {
	meta := scraper.ExtractMetadata(testHTML)

	if meta.Title != "Test Page" {
		t.Errorf("expected title 'Test Page', got %q", meta.Title)
	}
	if meta.Description != "A test page for scraping" {
		t.Errorf("expected description, got %q", meta.Description)
	}
	if len(meta.Headings) == 0 {
		t.Error("expected at least one heading")
	}
}

func TestExtractBySelector_Tag(t *testing.T) {
	results := scraper.ExtractBySelector(testHTML, []string{"h2", "p"})

	foundH2 := false
	foundP := false
	for _, r := range results {
		if r.Selector == "h2" && len(r.Matches) > 0 {
			foundH2 = true
		}
		if r.Selector == "p" && len(r.Matches) > 0 {
			foundP = true
		}
	}

	if !foundH2 {
		t.Error("expected h2 matches")
	}
	if !foundP {
		t.Error("expected p matches")
	}
}

func TestExtractParagraphs(t *testing.T) {
	paragraphs := scraper.ExtractParagraphs(testHTML)

	if len(paragraphs) < 2 {
		t.Errorf("expected at least 2 paragraphs, got %d", len(paragraphs))
	}
}
