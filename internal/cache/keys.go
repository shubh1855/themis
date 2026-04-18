package cache

func Key(prefix, id string) string {
	return prefix + ":" + id
}

func SearchKey(provider, query string, limit int) string {
	return HashKey("search", provider, query, string(rune(limit)))
}

func RegistryKey(registry, name string) string {
	return HashKey("registry", registry, name)
}

func PageKey(url string) string {
	return HashKey("page", url)
}
