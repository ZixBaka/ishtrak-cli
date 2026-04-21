// Package queue manages pending task payloads written by the git hook and
// drained by the native messaging host when the browser extension polls.
package queue

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var mu sync.Mutex

// DefaultPath returns ~/.config/ishtrak/pending.jsonl
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ishtrak", "pending.jsonl")
}

// Enqueue appends item as a JSON line to the queue file.
func Enqueue(item interface{}) error {
	mu.Lock()
	defer mu.Unlock()

	path := DefaultPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
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

	// Clear queue before processing so a crash doesn't re-process
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
