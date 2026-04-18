package models

type PackageInfo struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Homepage    string   `json:"homepage,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	License     string   `json:"license,omitempty"`
	Downloads   int64    `json:"downloads,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Registry    string   `json:"registry"`
}

type PackageSearchResult struct {
	Query    string        `json:"query"`
	Registry string        `json:"registry"`
	Packages []PackageInfo `json:"packages"`
	Total    int           `json:"total"`
}
