// Package messaging implements the Chrome Native Messaging protocol.
// Messages are length-prefixed JSON: a 4-byte little-endian uint32 followed
// by the JSON body. Maximum message size is 1 MB (Chrome limit).
package messaging

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

const maxMessageSize = 1024 * 1024 // 1 MB Chrome limit

// Host communicates with a browser extension via Native Messaging over
// a pair of ReadWriter (typically stdin/stdout of the native host process,
// but here inverted: the CLI writes to the host's stdin and reads from its
// stdout via a named pipe or process pipe).
//
// In our architecture the CLI binary IS the native host – so when acting
// as the initiating side we open the host process and write to its stdin.
// For now (Phase 1 MVP) we expose a simpler SendAndReceive that spawns
// nothing and just frames messages to stdout for integration testing, and
// a Send/Receive pair that can be used with an exec.Cmd's pipes.
type Host struct {
	w   io.Writer
	r   io.Reader
	timeout time.Duration
}

// New creates a Host that writes to w and reads from r.
func New(w io.Writer, r io.Reader, timeout time.Duration) *Host {
	return &Host{w: w, r: r, timeout: timeout}
}

// NewStdio creates a Host connected to the current process's stdin/stdout.
// Used when the ishtrak binary itself is launched as a native messaging host
// by the browser.
func NewStdio(timeout time.Duration) *Host {
	return New(os.Stdout, os.Stdin, timeout)
}

// Send encodes msg as a length-prefixed JSON message and writes it.
func (h *Host) Send(msg NativeMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	if len(body) > maxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", len(body), maxMessageSize)
	}

	length := uint32(len(body))
	if err := binary.Write(h.w, binary.LittleEndian, length); err != nil {
		return fmt.Errorf("write length prefix: %w", err)
	}
	if _, err := h.w.Write(body); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	return nil
}

// Receive reads one length-prefixed JSON message and decodes it into resp.
func (h *Host) Receive(resp *NativeResponse) error {
	var length uint32
	if err := binary.Read(h.r, binary.LittleEndian, &length); err != nil {
		return fmt.Errorf("read length prefix: %w", err)
	}
	if length > maxMessageSize {
		return fmt.Errorf("incoming message too large: %d bytes", length)
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(h.r, body); err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if err := json.Unmarshal(body, resp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

// ReceiveRequest reads one length-prefixed JSON message into msg.
// Used when ishtrak runs as the native host and must read requests from the extension.
func (h *Host) ReceiveRequest(msg *NativeMessage) error {
	var length uint32
	if err := binary.Read(h.r, binary.LittleEndian, &length); err != nil {
		return fmt.Errorf("read length prefix: %w", err)
	}
	if length > maxMessageSize {
		return fmt.Errorf("incoming message too large: %d bytes", length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(h.r, body); err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if err := json.Unmarshal(body, msg); err != nil {
		return fmt.Errorf("unmarshal request: %w", err)
	}
	return nil
}

// SendRaw encodes any value as a length-prefixed JSON message.
// Used when ishtrak runs as the native host and must send typed responses
// that don't fit the NativeMessage envelope (e.g. PendingTasksResponse).
func (h *Host) SendRaw(v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if len(body) > maxMessageSize {
		return fmt.Errorf("response too large: %d bytes (max %d)", len(body), maxMessageSize)
	}
	length := uint32(len(body))
	if err := binary.Write(h.w, binary.LittleEndian, length); err != nil {
		return fmt.Errorf("write length prefix: %w", err)
	}
	if _, err := h.w.Write(body); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	return nil
}

// SendAndReceive sends a message and waits for a response, respecting h.timeout.
// Returns an error if the timeout elapses before a response arrives.
func (h *Host) SendAndReceive(msg NativeMessage) (*NativeResponse, error) {
	if err := h.Send(msg); err != nil {
		return nil, err
	}

	type result struct {
		resp *NativeResponse
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		var r NativeResponse
		err := h.Receive(&r)
		ch <- result{&r, err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		return res.resp, nil
	case <-time.After(h.timeout):
		return nil, fmt.Errorf("timed out after %s waiting for extension response", h.timeout)
	}
}
