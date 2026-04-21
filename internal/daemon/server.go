package daemon

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

const CommandTimeout = 30 * time.Second

// Server exposes the daemon's HTTP endpoints.
type Server struct {
	hub *Hub
}

func NewServer(hub *Hub) *Server {
	return &Server{hub: hub}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/poll", s.handlePoll)
	mux.HandleFunc("/response", s.handleResponse)
	mux.HandleFunc("/command", s.handleCommand)
	mux.HandleFunc("/health", s.handleHealth)
	return corsMiddleware(mux)
}

// GET /poll — extension calls this in a tight loop; blocks until a command
// is ready or 20s elapses, then returns immediately so the extension starts
// the next poll.  Each in-flight fetch keeps the MV3 service worker alive.
func (s *Server) handlePoll(w http.ResponseWriter, r *http.Request) {
	cmd, ok := s.hub.Poll(r.Context())
	if !ok {
		writeJSON(w, map[string]string{"type": "idle"})
		return
	}
	writeJSON(w, cmd)
}

// POST /response — extension posts the result of a processed command here.
func (s *Server) handleResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var resp pollResponse
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.hub.Respond(resp)
	writeJSON(w, map[string]bool{"ok": true})
}

// POST /command — CLI calls this; blocks until the extension responds.
func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]string{"error": "bad request: " + err.Error()})
		return
	}

	ctx := r.Context()
	data, err := s.hub.Send(ctx, req.Type, req.Payload)
	if err != nil {
		log.Debug().Err(err).Str("type", req.Type).Msg("daemon: command failed")
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]json.RawMessage{"data": data})
}

// GET /health — liveness check; also indicates whether extension is polling.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{
		"ok":        true,
		"connected": s.hub.Connected(),
	})
}

// corsMiddleware adds headers required for Chrome's Private Network Access checks.
// Extensions fetching http://127.0.0.1 trigger a PNA preflight; without these
// headers the browser silently blocks the request.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
