// Package cache provides a thread-safe, TTL-based in-memory cache.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type entry struct {
	value     interface{}
	expiresAt time.Time
}

// Memory is a concurrency-safe in-memory cache with TTL expiration.
type Memory struct {
	mu      sync.RWMutex
	items   map[string]entry
	defaultTTL time.Duration
	stopCh  chan struct{}
}

// NewMemory creates a new in-memory cache with the given default TTL.
// It starts a background goroutine that cleans expired entries every interval.
func NewMemory(defaultTTL, cleanupInterval time.Duration) *Memory {
	m := &Memory{
		items:      make(map[string]entry),
		defaultTTL: defaultTTL,
		stopCh:     make(chan struct{}),
	}
	go m.cleanup(cleanupInterval)
	return m
}

// Get retrieves a value by key. Returns nil and false if not found or expired.
func (m *Memory) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	e, ok := m.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.value, true
}

// Set stores a value with the default TTL.
func (m *Memory) Set(key string, value interface{}) {
	m.SetWithTTL(key, value, m.defaultTTL)
}

// SetWithTTL stores a value with a custom TTL.
func (m *Memory) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[key] = entry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes an entry by key.
func (m *Memory) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
}

// Clear removes all entries.
func (m *Memory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]entry)
}

// Len returns the number of entries (including expired ones not yet cleaned).
func (m *Memory) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// Stop shuts down the background cleanup goroutine.
func (m *Memory) Stop() {
	close(m.stopCh)
}

func (m *Memory) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.evictExpired()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Memory) evictExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, e := range m.items {
		if now.After(e.expiresAt) {
			delete(m.items, k)
		}
	}
}

// HashKey creates a deterministic cache key from a string using SHA-256.
func HashKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}
