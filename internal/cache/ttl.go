package cache

import "time"

const DefaultTTL = 5 * time.Minute

const SearchTTL = 10 * time.Minute

const RegistryTTL = 15 * time.Minute

const PageTTL = 10 * time.Minute

const CleanupInterval = 1 * time.Minute
