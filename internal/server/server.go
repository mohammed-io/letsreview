package server

import (
	"crypto/md5"
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
	"strconv"
	"sync"
	"time"

	"github.com/mohammed/letsreview/internal/gitdiff"
)

//go:embed web/*
var webFS embed.FS

type Server struct {
	defaultProjectID string
	mux              *http.ServeMux
	store            *Store
	static           fs.FS
}

type Store struct {
	mu       sync.RWMutex
	projects map[string]*Project
}

type Project struct {
	ID        string                `json:"id"`
	RepoPath  string                `json:"repoPath"`
	Repo      string                `json:"repo"`
	CreatedAt time.Time             `json:"createdAt"`
	LastPing  time.Time             `json:"lastPing"`
	Sessions  map[string]Session    `json:"-"`
	Feedback  map[string][]Feedback `json:"-"`
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

type AgentComment struct {
	FilePath  string    `json:"filePath"`
	StartLine int       `json:"startLine"`
	EndLine   int       `json:"endLine"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

func New(repoPath string) (*Server, error) {
	static, err := webStaticFS()
	if err != nil {
		return nil, err
	}

	project, err := newProject(repoPath)
	if err != nil {
		return nil, err
	}

	store := &Store{
		projects: map[string]*Project{project.ID: &project},
	}
	server := &Server{defaultProjectID: project.ID, mux: http.NewServeMux(), store: store, static: static}
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
	s.mux.HandleFunc("POST /api/projects", s.registerProject)
	s.mux.HandleFunc("POST /api/projects/{projectID}/heartbeat", s.heartbeatProject)
	s.mux.HandleFunc("GET /api/projects/{projectID}/live", s.projectLiveDiff)
	s.mux.HandleFunc("GET /api/projects/{projectID}/sessions", s.projectListSessions)
	s.mux.HandleFunc("POST /api/projects/{projectID}/sessions", s.projectCreateSession)
	s.mux.HandleFunc("DELETE /api/projects/{projectID}/sessions/{id}", s.projectDeleteSession)
	s.mux.HandleFunc("GET /api/projects/{projectID}/sessions/{id}", s.projectGetSession)
	s.mux.HandleFunc("POST /api/projects/{projectID}/sessions/{id}/explain", s.projectExplain)
	s.mux.HandleFunc("POST /api/projects/{projectID}/sessions/{id}/feedback", s.projectAddFeedback)
	s.mux.HandleFunc("DELETE /api/projects/{projectID}/sessions/{id}/feedback/{feedbackID}", s.projectDeleteFeedback)
	s.mux.HandleFunc("GET /api/projects/{projectID}/sessions/{id}/agent-payload", s.projectAgentPayload)
	s.mux.HandleFunc("GET /api/live", s.liveDiff)
	s.mux.HandleFunc("GET /api/sessions", s.listSessions)
	s.mux.HandleFunc("POST /api/sessions", s.createSession)
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.deleteSession)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.getSession)
	s.mux.HandleFunc("POST /api/sessions/{id}/explain", s.explain)
	s.mux.HandleFunc("POST /api/sessions/{id}/feedback", s.addFeedback)
	s.mux.HandleFunc("DELETE /api/sessions/{id}/feedback/{feedbackID}", s.deleteFeedback)
	s.mux.HandleFunc("GET /api/sessions/{id}/agent-payload", s.agentPayload)

	s.mux.Handle("/", http.FileServer(http.FS(s.static)))
}

func webStaticFS() (fs.FS, error) {
	root := os.Getenv("WEB_UI_ROOT")
	if root != "" {
		return os.DirFS(root), nil
	}
	return fs.Sub(webFS, "web")
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	project, _ := s.project(s.defaultProjectID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "repoPath": project.RepoPath, "projectID": project.ID})
}

func (s *Server) registerProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RepoPath string `json:"repoPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	project, err := newProject(req.RepoPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	s.store.mu.Lock()
	existing, ok := s.store.projects[project.ID]
	if ok {
		existing.LastPing = time.Now().UTC()
		project = *existing
	} else {
		s.store.projects[project.ID] = &project
	}
	s.store.mu.Unlock()

	writeJSON(w, http.StatusOK, project)
}

func (s *Server) heartbeatProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	s.store.mu.Lock()
	project, ok := s.store.projects[projectID]
	if ok {
		project.LastPing = time.Now().UTC()
	}
	s.store.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("project session not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) liveDiff(w http.ResponseWriter, r *http.Request) {
	s.liveDiffFor(w, r, s.defaultProjectID)
}

func (s *Server) projectLiveDiff(w http.ResponseWriter, r *http.Request) {
	s.liveDiffFor(w, r, r.PathValue("projectID"))
}

func (s *Server) liveDiffFor(w http.ResponseWriter, r *http.Request, projectID string) {
	project, ok := s.project(projectID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("project session not found"))
		return
	}
	req := gitdiff.Request{
		RepoPath:     project.RepoPath,
		Mode:         gitdiff.ModeWorking,
		ContextLines: contextLinesFromQuery(r),
	}

	files, err := gitdiff.Load(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result := map[string]any{
		"title":   "Working tree vs HEAD",
		"files":   files,
		"summary": gitdiff.Summary(files),
		"stats":   stats(files),
		"meta":    map[string]string{"repo": project.Repo, "projectID": project.ID},
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	s.listSessionsFor(w, r, s.defaultProjectID)
}

func (s *Server) projectListSessions(w http.ResponseWriter, r *http.Request) {
	s.listSessionsFor(w, r, r.PathValue("projectID"))
}

func (s *Server) listSessionsFor(w http.ResponseWriter, r *http.Request, projectID string) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	project, ok := s.store.projects[projectID]
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("project session not found"))
		return
	}

	sessions := make([]Session, 0, len(project.Sessions))
	for _, session := range project.Sessions {
		session.Feedback = project.Feedback[session.ID]
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	s.createSessionFor(w, r, s.defaultProjectID)
}

func (s *Server) projectCreateSession(w http.ResponseWriter, r *http.Request) {
	s.createSessionFor(w, r, r.PathValue("projectID"))
}

func (s *Server) createSessionFor(w http.ResponseWriter, r *http.Request, projectID string) {
	project, ok := s.project(projectID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("project session not found"))
		return
	}

	var req gitdiff.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.RepoPath = project.RepoPath

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
		Meta:      map[string]string{"repo": project.Repo, "projectID": project.ID},
	}

	s.store.mu.Lock()
	if project := s.store.projects[projectID]; project != nil {
		project.Sessions[session.ID] = session
	}
	s.store.mu.Unlock()

	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	s.getSessionFor(w, r, s.defaultProjectID)
}

func (s *Server) projectGetSession(w http.ResponseWriter, r *http.Request) {
	s.getSessionFor(w, r, r.PathValue("projectID"))
}

func (s *Server) getSessionFor(w http.ResponseWriter, r *http.Request, projectID string) {
	session, ok := s.session(projectID, r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("session not found"))
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	s.deleteSessionFor(w, r, s.defaultProjectID)
}

func (s *Server) projectDeleteSession(w http.ResponseWriter, r *http.Request) {
	s.deleteSessionFor(w, r, r.PathValue("projectID"))
}

func (s *Server) deleteSessionFor(w http.ResponseWriter, r *http.Request, projectID string) {
	sessionID := r.PathValue("id")
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	project, ok := s.store.projects[projectID]
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("project session not found"))
		return
	}
	if _, ok := project.Sessions[sessionID]; !ok {
		writeError(w, http.StatusNotFound, errors.New("session not found"))
		return
	}
	delete(project.Sessions, sessionID)
	delete(project.Feedback, sessionID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) explain(w http.ResponseWriter, r *http.Request) {
	s.explainFor(w, r, s.defaultProjectID)
}

func (s *Server) projectExplain(w http.ResponseWriter, r *http.Request) {
	s.explainFor(w, r, r.PathValue("projectID"))
}

func (s *Server) explainFor(w http.ResponseWriter, r *http.Request, projectID string) {
	session, ok := s.session(projectID, r.PathValue("id"))
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
	s.addFeedbackFor(w, r, s.defaultProjectID)
}

func (s *Server) projectAddFeedback(w http.ResponseWriter, r *http.Request) {
	s.addFeedbackFor(w, r, r.PathValue("projectID"))
}

func (s *Server) addFeedbackFor(w http.ResponseWriter, r *http.Request, projectID string) {
	sessionID := r.PathValue("id")
	if _, ok := s.session(projectID, sessionID); !ok {
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
	if project := s.store.projects[projectID]; project != nil {
		project.Feedback[sessionID] = append(project.Feedback[sessionID], feedback)
	}
	s.store.mu.Unlock()

	writeJSON(w, http.StatusCreated, feedback)
}

func (s *Server) deleteFeedback(w http.ResponseWriter, r *http.Request) {
	s.deleteFeedbackFor(w, r, s.defaultProjectID)
}

func (s *Server) projectDeleteFeedback(w http.ResponseWriter, r *http.Request) {
	s.deleteFeedbackFor(w, r, r.PathValue("projectID"))
}

func (s *Server) deleteFeedbackFor(w http.ResponseWriter, r *http.Request, projectID string) {
	sessionID := r.PathValue("id")
	if _, ok := s.session(projectID, sessionID); !ok {
		writeError(w, http.StatusNotFound, errors.New("session not found"))
		return
	}

	feedbackID := r.PathValue("feedbackID")
	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	project := s.store.projects[projectID]
	feedback := project.Feedback[sessionID]
	next := make([]Feedback, 0, len(feedback))
	found := false
	for _, item := range feedback {
		if item.ID == feedbackID {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		writeError(w, http.StatusNotFound, errors.New("feedback not found"))
		return
	}
	project.Feedback[sessionID] = next
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) agentPayload(w http.ResponseWriter, r *http.Request) {
	s.agentPayloadFor(w, r, s.defaultProjectID)
}

func (s *Server) projectAgentPayload(w http.ResponseWriter, r *http.Request) {
	s.agentPayloadFor(w, r, r.PathValue("projectID"))
}

func (s *Server) agentPayloadFor(w http.ResponseWriter, r *http.Request, projectID string) {
	session, ok := s.session(projectID, r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("session not found"))
		return
	}

	comments := make([]AgentComment, 0, len(session.Feedback))
	for _, feedback := range session.Feedback {
		comments = append(comments, AgentComment{
			FilePath:  feedback.FilePath,
			StartLine: feedback.StartLine,
			EndLine:   feedback.EndLine,
			Body:      feedback.Body,
			CreatedAt: feedback.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string][]AgentComment{"comments": comments})
}

func (s *Server) project(id string) (Project, bool) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	project, ok := s.store.projects[id]
	if !ok {
		return Project{}, false
	}
	return *project, true
}

func (s *Server) session(projectID string, id string) (Session, bool) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	project, ok := s.store.projects[projectID]
	if !ok {
		return Session{}, false
	}
	session, ok := project.Sessions[id]
	if !ok {
		return Session{}, false
	}
	session.Feedback = project.Feedback[id]
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

func newProject(repoPath string) (Project, error) {
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return Project{}, err
	}
	if _, err := os.Stat(absRepo); err != nil {
		return Project{}, err
	}
	now := time.Now().UTC()
	return Project{
		ID:        projectID(absRepo),
		RepoPath:  absRepo,
		Repo:      filepath.Base(absRepo),
		CreatedAt: now,
		LastPing:  now,
		Sessions:  map[string]Session{},
		Feedback:  map[string][]Feedback{},
	}, nil
}

func contextLinesFromQuery(r *http.Request) int {
	value := r.URL.Query().Get("contextLines")
	if value == "" {
		return 0
	}
	lines, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return lines
}

func projectID(absRepo string) string {
	sum := md5.Sum([]byte(absRepo))
	return hex.EncodeToString(sum[:])
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
