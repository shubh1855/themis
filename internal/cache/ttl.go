package cache

import "time"

// DefaultTTL is the default expiration duration for cached entries.
const DefaultTTL = 5 * time.Minute

// SearchTTL is the TTL for search results.
const SearchTTL = 10 * time.Minute

// RegistryTTL is the TTL for package registry lookups.
const RegistryTTL = 15 * time.Minute

// PageTTL is the TTL for fetched page content.
const PageTTL = 10 * time.Minute

// CleanupInterval is how often the background cleanup goroutine runs.
const CleanupInterval = 1 * time.Minute
