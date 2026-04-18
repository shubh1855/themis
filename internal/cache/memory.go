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

type Memory struct {
	mu      sync.RWMutex
	items   map[string]entry
	defaultTTL time.Duration
	stopCh  chan struct{}
}

func NewMemory(defaultTTL, cleanupInterval time.Duration) *Memory {
	m := &Memory{
		items:      make(map[string]entry),
		defaultTTL: defaultTTL,
		stopCh:     make(chan struct{}),
	}
	go m.cleanup(cleanupInterval)
	return m
}

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

func (m *Memory) Set(key string, value interface{}) {
	m.SetWithTTL(key, value, m.defaultTTL)
}

func (m *Memory) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[key] = entry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

func (m *Memory) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
}

func (m *Memory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]entry)
}

func (m *Memory) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

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

func HashKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}
