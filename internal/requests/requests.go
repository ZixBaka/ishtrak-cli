// Package requests implements file-based IPC between the CLI and the browser
// extension. The CLI writes a request file and polls for a response file;
// the extension (via the native host) reads requests and writes responses.
package requests

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zixbaka/ishtrak/internal/config"
	"github.com/zixbaka/ishtrak/internal/messaging"
)

var (
	mu           sync.Mutex
	reqDirOnce   sync.Once
	respDirOnce  sync.Once
)

func requestsDir() string  { return filepath.Join(config.ConfigDir(), "requests") }
func responsesDir() string { return filepath.Join(config.ConfigDir(), "responses") }

func ensureRequestsDir() {
	reqDirOnce.Do(func() { os.MkdirAll(requestsDir(), 0o700) }) //nolint:errcheck
}

func ensureResponsesDir() {
	respDirOnce.Do(func() { os.MkdirAll(responsesDir(), 0o700) }) //nolint:errcheck
}

// WriteRequest writes a command request to disk and returns its UUID.
func WriteRequest(msgType string, payload interface{}) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	id := uuid.New().String()
	req := messaging.CommandRequest{
		UUID:    id,
		Type:    msgType,
		Payload: json.RawMessage(raw),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	ensureRequestsDir()
	path := filepath.Join(requestsDir(), id+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write request file: %w", err)
	}
	return id, nil
}

// ReadAllRequests reads and atomically clears all pending request files.
func ReadAllRequests() ([]messaging.CommandRequest, error) {
	mu.Lock()
	defer mu.Unlock()

	dir := requestsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read requests dir: %w", err)
	}

	var requests []messaging.CommandRequest
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := os.Remove(path); err != nil {
			continue
		}
		var req messaging.CommandRequest
		if err := json.Unmarshal(data, &req); err != nil {
			continue
		}
		requests = append(requests, req)
	}
	return requests, nil
}

// WriteResponse writes a command response to the responses directory.
func WriteResponse(id string, data interface{}, errMsg string) error {
	ensureResponsesDir()

	resp := messaging.CommandResponse{UUID: id, Error: errMsg}
	if data != nil && errMsg == "" {
		raw, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal response data: %w", err)
		}
		resp.Data = json.RawMessage(raw)
	}

	out, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	path := filepath.Join(responsesDir(), id+".json")
	return os.WriteFile(path, out, 0o600)
}

// WaitResponse polls for a response file until it appears or timeout elapses.
func WaitResponse(id string, timeout time.Duration) (*messaging.CommandResponse, error) {
	path := filepath.Join(responsesDir(), id+".json")
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			_ = os.Remove(path)
			var resp messaging.CommandResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				return nil, fmt.Errorf("unmarshal response: %w", err)
			}
			return &resp, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("timed out after %s waiting for extension response (is the extension running?)", timeout)
}
