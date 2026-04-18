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

type diskEntry struct {
	Value     json.RawMessage `json:"value"`
	ExpiresAt time.Time       `json:"expires_at"`
}

type Disk struct {
	dir string
}

func NewDisk(dir string) (*Disk, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("disk cache: %w", err)
	}
	return &Disk{dir: dir}, nil
}

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

func (d *Disk) Delete(key string) {
	os.Remove(d.keyPath(key))
}

func (d *Disk) keyPath(key string) string {
	h := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(h[:])[:32] + ".json"
	return filepath.Join(d.dir, name)
}
