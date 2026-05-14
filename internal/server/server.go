package server

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/mohammed/letsreview/internal/gitdiff"
)

//go:embed web/*
var webFS embed.FS

type Server struct {
	repoPath string
	mux      *http.ServeMux
	store    *Store
}

type Store struct {
	mu       sync.RWMutex
	sessions map[string]Session
	feedback map[string][]Feedback
}

type Session struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	CreatedAt time.Time         `json:"createdAt"`
	Request   gitdiff.Request   `json:"request"`
	Files     []gitdiff.File    `json:"files"`
	Summary   string            `json:"summary"`
	Stats     map[string]int    `json:"stats"`
	Feedback  []Feedback        `json:"feedback,omitempty"`
	Meta      map[string]string `json:"meta"`
}

type Feedback struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	FilePath  string    `json:"filePath"`
	StartLine int       `json:"startLine"`
	EndLine   int       `json:"endLine"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

func New(repoPath string) (*Server, error) {
	if _, err := os.Stat(repoPath); err != nil {
		return nil, err
	}

	store := &Store{
		sessions: map[string]Session{},
		feedback: map[string][]Feedback{},
	}
	server := &Server{repoPath: repoPath, mux: http.NewServeMux(), store: store}
	server.routes()
	return server, nil
}

func (s *Server) Serve(listener net.Listener) error {
	return http.Serve(listener, s.mux)
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("GET /api/sessions", s.listSessions)
	s.mux.HandleFunc("POST /api/sessions", s.createSession)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.getSession)
	s.mux.HandleFunc("POST /api/sessions/{id}/explain", s.explain)
	s.mux.HandleFunc("POST /api/sessions/{id}/feedback", s.addFeedback)
	s.mux.HandleFunc("GET /api/sessions/{id}/agent-payload", s.agentPayload)

	static, _ := fs.Sub(webFS, "web")
	s.mux.Handle("/", http.FileServer(http.FS(static)))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "repoPath": s.repoPath})
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	sessions := make([]Session, 0, len(s.store.sessions))
	for _, session := range s.store.sessions {
		session.Feedback = s.store.feedback[session.ID]
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var req gitdiff.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.RepoPath = s.repoPath

	files, err := gitdiff.Load(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	session := Session{
		ID:        newID(),
		Title:     titleFor(req),
		CreatedAt: time.Now().UTC(),
		Request:   req,
		Files:     files,
		Summary:   gitdiff.Summary(files),
		Stats:     stats(files),
		Meta:      map[string]string{"repo": filepath.Base(s.repoPath)},
	}

	s.store.mu.Lock()
	s.store.sessions[session.ID] = session
	s.store.mu.Unlock()

	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	session, ok := s.session(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("session not found"))
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) explain(w http.ResponseWriter, r *http.Request) {
	session, ok := s.session(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("session not found"))
		return
	}

	var req struct {
		FilePath  string `json:"filePath"`
		StartLine int    `json:"startLine"`
		EndLine   int    `json:"endLine"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"summary": gitdiff.ExplainSelection(session.Files, req.FilePath, req.StartLine, req.EndLine),
	})
}

func (s *Server) addFeedback(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if _, ok := s.session(sessionID); !ok {
		writeError(w, http.StatusNotFound, errors.New("session not found"))
		return
	}

	var req struct {
		FilePath  string `json:"filePath"`
		StartLine int    `json:"startLine"`
		EndLine   int    `json:"endLine"`
		Body      string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, errors.New("feedback body is required"))
		return
	}

	feedback := Feedback{
		ID:        newID(),
		SessionID: sessionID,
		FilePath:  req.FilePath,
		StartLine: req.StartLine,
		EndLine:   req.EndLine,
		Body:      req.Body,
		CreatedAt: time.Now().UTC(),
	}

	s.store.mu.Lock()
	s.store.feedback[sessionID] = append(s.store.feedback[sessionID], feedback)
	s.store.mu.Unlock()

	writeJSON(w, http.StatusCreated, feedback)
}

func (s *Server) agentPayload(w http.ResponseWriter, r *http.Request) {
	session, ok := s.session(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("session not found"))
		return
	}

	payload := map[string]any{
		"repoPath":  s.repoPath,
		"session":   session,
		"directive": "Apply requested changes from feedback. Preserve unrelated user changes. Add tests for observable behavior.",
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) session(id string) (Session, bool) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	session, ok := s.store.sessions[id]
	if !ok {
		return Session{}, false
	}
	session.Feedback = s.store.feedback[id]
	return session, true
}

func stats(files []gitdiff.File) map[string]int {
	result := map[string]int{"files": len(files), "additions": 0, "deletions": 0}
	for _, file := range files {
		result["additions"] += file.Additions
		result["deletions"] += file.Deletions
	}
	return result
}

func titleFor(req gitdiff.Request) string {
	switch req.Mode {
	case gitdiff.ModeStaged:
		return "Staged changes"
	case gitdiff.ModeRefs:
		return req.BaseRef + "..." + req.HeadRef
	default:
		return "Working tree vs HEAD"
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func newID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(bytes[:])
}
