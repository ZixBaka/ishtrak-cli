// Package queue manages pending task payloads written by the git hook and
// drained by the native messaging host when the browser extension polls.
package queue

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/zixbaka/ishtrak/internal/config"
)

var (
	mu      sync.Mutex
	dirOnce sync.Once
)

// DefaultPath returns ~/.config/ishtrak/pending.jsonl
func DefaultPath() string {
	return filepath.Join(config.ConfigDir(), "pending.jsonl")
}

func ensureDir() {
	dirOnce.Do(func() { os.MkdirAll(filepath.Dir(DefaultPath()), 0o700) }) //nolint:errcheck
}

// Enqueue appends item as a JSON line to the queue file.
func Enqueue(item interface{}) error {
	mu.Lock()
	defer mu.Unlock()

	ensureDir()
	f, err := os.OpenFile(DefaultPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(item)
}

// DrainAll reads all queued items as raw JSON messages and atomically clears
// the queue. Returns nil slice (not an error) when the queue is empty.
func DrainAll() ([]json.RawMessage, error) {
	mu.Lock()
	defer mu.Unlock()

	path := DefaultPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Truncate before decoding so a crash between these two operations doesn't replay items.
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return nil, err
	}

	var items []json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			continue // skip malformed lines
		}
		items = append(items, raw)
	}
	return items, nil
}
