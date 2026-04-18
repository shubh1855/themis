package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// diskEntry wraps a cached value with its expiration time for on-disk storage.
type diskEntry struct {
	Value     json.RawMessage `json:"value"`
	ExpiresAt time.Time       `json:"expires_at"`
}

// Disk provides a simple filesystem-backed cache with TTL support.
type Disk struct {
	dir string
}

// NewDisk creates a new disk cache that stores entries in the given directory.
func NewDisk(dir string) (*Disk, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("disk cache: %w", err)
	}
	return &Disk{dir: dir}, nil
}

// Get retrieves a value from disk. Returns nil and false if missing or expired.
func (d *Disk) Get(key string) ([]byte, bool) {
	path := d.keyPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var e diskEntry
	if err := json.Unmarshal(data, &e); err != nil {
		os.Remove(path)
		return nil, false
	}

	if time.Now().After(e.ExpiresAt) {
		os.Remove(path)
		return nil, false
	}

	return e.Value, true
}

// Set stores a JSON-serializable value on disk with the given TTL.
func (d *Disk) Set(key string, value interface{}, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("disk cache marshal: %w", err)
	}

	e := diskEntry{
		Value:     raw,
		ExpiresAt: time.Now().Add(ttl),
	}

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("disk cache wrap: %w", err)
	}

	return os.WriteFile(d.keyPath(key), data, 0644)
}

// Delete removes an entry from disk.
func (d *Disk) Delete(key string) {
	os.Remove(d.keyPath(key))
}

func (d *Disk) keyPath(key string) string {
	h := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(h[:])[:32] + ".json"
	return filepath.Join(d.dir, name)
}
