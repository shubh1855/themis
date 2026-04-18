package cache

// Key constructs a namespaced cache key from a prefix and identifier.
func Key(prefix, id string) string {
	return prefix + ":" + id
}

// SearchKey builds a cache key for a search query.
func SearchKey(provider, query string, limit int) string {
	return HashKey("search", provider, query, string(rune(limit)))
}

// RegistryKey builds a cache key for a registry lookup.
func RegistryKey(registry, name string) string {
	return HashKey("registry", registry, name)
}

// PageKey builds a cache key for a fetched page URL.
func PageKey(url string) string {
	return HashKey("page", url)
}
