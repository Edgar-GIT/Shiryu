package server

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"shiryu/CLI_VERSION/src/go/utils"
)

type Server struct {
	sessions *SessionManager
	web      fs.FS
}

func New(web fs.FS) *Server {
	loadStats()
	return &Server{sessions: NewSessionManager(), web: web}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/metadata", s.handleMetadata)
	mux.HandleFunc("/api/download/start", s.handleStart)
	mux.HandleFunc("/api/download/control", s.handleControl)
	mux.HandleFunc("/api/download/recover", s.handleRecover)
	mux.HandleFunc("/api/download/events", s.handleSSE)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.Handle("/", http.FileServer(http.FS(s.web)))
	return mux
}

func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	meta, err := utils.FetchURLMetadata(req.URL)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"filename":      meta.Filename,
		"size":          meta.Size,
		"supportsRange": meta.SupportsRange,
	})
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		jsonError(w, "url required", http.StatusBadRequest)
		return
	}
	sess, err := s.sessions.Start(req)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, sess.Snapshot())
}

func (s *Server) handleControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.sessions.Control(req.Action); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	sess := s.sessions.Current()
	if sess != nil {
		writeJSON(w, sess.Snapshot())
	} else {
		writeJSON(w, map[string]string{"ok": "true"})
	}
}

func (s *Server) handleRecover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.sessions.Recover(req.Action); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	sess := s.sessions.Current()
	if sess != nil {
		writeJSON(w, sess.Snapshot())
	}
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			sess := s.sessions.Current()
			if sess == nil {
				writeSSE(w, Progress{State: StateIdle})
			} else {
				writeSSE(w, sess.Snapshot())
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, GetStats())
}

func writeSSE(w http.ResponseWriter, v any) {
	data, _ := json.Marshal(v)
	io.WriteString(w, "data: ")
	w.Write(data)
	io.WriteString(w, "\n\n")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
