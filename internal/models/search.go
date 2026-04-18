package models

// SearchResult represents a single web search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Rank    int    `json:"rank"`
	Source  string `json:"source,omitempty"`
}

// SearchResponse holds the complete result set from a search query.
type SearchResponse struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
}

// PageContent represents scraped web page content.
type PageContent struct {
	URL      string            `json:"url"`
	Title    string            `json:"title"`
	Text     string            `json:"text"`
	Links    []string          `json:"links,omitempty"`
	Headings []string          `json:"headings,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}
