package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const pollTimeout = 20 * time.Second
const connectedGrace = 25 * time.Second // slightly longer than pollTimeout

// envelope is the wire format for commands sent to the extension.
type envelope struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// pollResponse is what the extension POSTs back after handling a command.
type pollResponse struct {
	ID    string          `json:"id"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// Hub manages the long-poll connection between the daemon and the browser extension.
// The extension calls GET /poll (blocks up to 20s), picks up a command, processes
// it, then POSTs the result to /response — keeping its service worker alive the
// whole time via the in-flight fetch.
type Hub struct {
	mu           sync.Mutex
	waiters      map[string]chan pollResponse
	queue        chan *envelope
	lastPollTime int64 // unix nano, updated atomically on each incoming poll
}

func NewHub() *Hub {
	return &Hub{
		waiters: make(map[string]chan pollResponse),
		queue:   make(chan *envelope, 64),
	}
}

// Connected reports whether the extension polled recently enough to be considered live.
func (h *Hub) Connected() bool {
	last := atomic.LoadInt64(&h.lastPollTime)
	return last > 0 && time.Since(time.Unix(0, last)) < connectedGrace
}

// Send queues a command for the extension and waits for the response.
func (h *Hub) Send(ctx context.Context, msgType string, payload interface{}) (json.RawMessage, error) {
	if !h.Connected() {
		return nil, fmt.Errorf("extension not connected — open Chrome with the Ishtrak extension enabled")
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	id := uuid.New().String()
	cmd := &envelope{ID: id, Type: msgType, Payload: raw}

	ch := make(chan pollResponse, 1)
	h.mu.Lock()
	h.waiters[id] = ch
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.waiters, id)
		h.mu.Unlock()
	}()

	select {
	case h.queue <- cmd:
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out queuing command")
	}

	select {
	case resp := <-ch:
		if resp.Error != "" {
			return nil, fmt.Errorf("%s", resp.Error)
		}
		return resp.Data, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out waiting for extension response")
	}
}

// Poll is called by the extension's GET /poll request.
// It blocks until a command is available (or pollTimeout elapses) and returns it.
func (h *Hub) Poll(ctx context.Context) (*envelope, bool) {
	atomic.StoreInt64(&h.lastPollTime, time.Now().UnixNano())

	deadline, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()

	select {
	case cmd := <-h.queue:
		return cmd, true
	case <-deadline.Done():
		return nil, false // idle — extension should poll again immediately
	}
}

// Respond is called by the extension's POST /response request.
func (h *Hub) Respond(resp pollResponse) {
	h.mu.Lock()
	ch, ok := h.waiters[resp.ID]
	if ok {
		delete(h.waiters, resp.ID)
	}
	h.mu.Unlock()

	if ok {
		ch <- resp
	}
}
