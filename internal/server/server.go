package server

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/mohammed-io/letsreview/internal/gitdiff"
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
	Projects map[string]*Project
	events           map[string][]ReviewEvent
	waiters          map[string][]eventWaiter
	nextEventSeq     int64
	OnEventPublished func(event ReviewEvent)
}

type Project struct {
	ID              string                          `json:"id"`
	RepoPath        string                          `json:"repoPath"`
	Repo            string                          `json:"repo"`
	CreatedAt       time.Time                       `json:"createdAt"`
	LastPing        time.Time                       `json:"lastPing"`
	Sessions        map[string]Session              `json:"-"`
	Feedback        map[string][]Feedback           `json:"-"`
	Explanations    map[string][]Explanation        `json:"-"`
	ExplanationReqs map[string][]ExplanationRequest `json:"-"`
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
	ID         string     `json:"id"`
	SessionID  string     `json:"sessionId"`
	FilePath   string     `json:"filePath"`
	StartLine  int        `json:"startLine"`
	EndLine    int        `json:"endLine"`
	Body       string     `json:"body"`
	CreatedAt  time.Time  `json:"createdAt"`
	Resolved   bool       `json:"resolved"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

type SubmittedReview struct {
	SessionID   string         `json:"sessionId"`
	Comments    []AgentComment `json:"comments"`
	SubmittedAt time.Time      `json:"submittedAt"`
	RepoPath    string         `json:"repoPath"`
	Files       []string       `json:"files"`
	Summary     string         `json:"summary"`
}

type AgentComment struct {
	ID         string     `json:"id"`
	FilePath   string     `json:"filePath"`
	StartLine  int        `json:"startLine"`
	EndLine    int        `json:"endLine"`
	Body       string     `json:"body"`
	CreatedAt  time.Time  `json:"createdAt"`
	Resolved   bool       `json:"resolved"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

type Explanation struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"filePath"`
	StartLine int       `json:"startLine"`
	EndLine   int       `json:"endLine"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type ExplanationRequest struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"filePath"`
	StartLine int       `json:"startLine"`
	EndLine   int       `json:"endLine"`
	CreatedAt time.Time `json:"createdAt"`
	Resolved  bool      `json:"resolved"`
}

type ReviewEvent struct {
	Seq                int64               `json:"seq"`
	Type               string              `json:"type"`
	ProjectID          string              `json:"projectID"`
	SessionID          string              `json:"sessionId"`
	CreatedAt          time.Time           `json:"createdAt"`
	Review             *SubmittedReview    `json:"review,omitempty"`
	ExplanationRequest *ExplanationRequest `json:"explanationRequest,omitempty"`
	Explanation        *Explanation        `json:"explanation,omitempty"`
}

type eventWaiter struct {
	afterSeq int64
	ch       chan ReviewEvent
}

func NewStore() *Store {
	return &Store{
		Projects: map[string]*Project{},
		events:   map[string][]ReviewEvent{},
		waiters:  map[string][]eventWaiter{},
	}
}

func (s *Store) PublishReviewEvent(event ReviewEvent) ReviewEvent {
	s.mu.Lock()

	s.nextEventSeq++
	event.Seq = s.nextEventSeq
	event.CreatedAt = time.Now().UTC()
	s.events[event.SessionID] = append(s.events[event.SessionID], event)

	waiters := s.waiters[event.SessionID]
	remaining := make([]eventWaiter, 0, len(waiters))
	for _, waiter := range waiters {
		if event.Seq > waiter.afterSeq {
			select {
			case waiter.ch <- event:
			default:
			}
			close(waiter.ch)
			continue
		}
		remaining = append(remaining, waiter)
	}
	if len(remaining) == 0 {
		delete(s.waiters, event.SessionID)
	} else {
		s.waiters[event.SessionID] = remaining
	}
	cb := s.OnEventPublished
	s.mu.Unlock()

	if cb != nil {
		cb(event)
	}
	return event
}

func (s *Store) GetEventsAfter(sessionID string, afterSeq int64) []ReviewEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []ReviewEvent
	for _, event := range s.events[sessionID] {
		if event.Seq > afterSeq {
			result = append(result, event)
		}
	}
	return result
}

func (s *Store) LastEventSeq() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextEventSeq
}

func (s *Store) WaitForReviewEvent(ctx context.Context, sessionID string, afterSeq int64) (ReviewEvent, bool) {
	s.mu.Lock()
	for _, event := range s.events[sessionID] {
		if event.Seq > afterSeq {
			s.mu.Unlock()
			return event, true
		}
	}

	waiter := eventWaiter{afterSeq: afterSeq, ch: make(chan ReviewEvent, 1)}
	s.waiters[sessionID] = append(s.waiters[sessionID], waiter)
	s.mu.Unlock()

	select {
	case event, ok := <-waiter.ch:
		return event, ok
	case <-ctx.Done():
		s.removeWaiter(sessionID, waiter.ch)
		return ReviewEvent{}, false
	}
}

func (s *Store) removeWaiter(sessionID string, ch chan ReviewEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	waiters := s.waiters[sessionID]
	next := make([]eventWaiter, 0, len(waiters))
	for _, waiter := range waiters {
		if waiter.ch != ch {
			next = append(next, waiter)
		}
	}
	if len(next) == 0 {
		delete(s.waiters, sessionID)
	} else {
		s.waiters[sessionID] = next
	}
}

func (s *Store) RegisterProject(project Project) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.Projects[project.ID]; ok {
		existing.LastPing = time.Now().UTC()
	} else {
		s.Projects[project.ID] = &project
	}
}

func (s *Store) AddExplanation(projectID string, sessionID string, explanation Explanation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if project := s.Projects[projectID]; project != nil {
		project.Explanations[sessionID] = append(project.Explanations[sessionID], explanation)
	}
}

func (s *Store) GetExplanations(projectID string, sessionID string) []Explanation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	project, ok := s.Projects[projectID]
	if !ok {
		return nil
	}
	return project.Explanations[sessionID]
}

func (s *Store) GetExplanationRequests(sessionID string) []ExplanationRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, project := range s.Projects {
		if reqs, ok := project.ExplanationReqs[sessionID]; ok {
			return reqs
		}
	}
	return nil
}

func (s *Store) FindProjectForSession(sessionID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for pid, project := range s.Projects {
		if _, ok := project.Sessions[sessionID]; ok {
			return pid
		}
	}
	return ""
}

func (s *Store) ResolveFeedback(projectID, sessionID, feedbackID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	project := s.Projects[projectID]
	if project == nil {
		return false
	}

	feedback := project.Feedback[sessionID]
	for i := range feedback {
		if feedback[i].ID == feedbackID {
			feedback[i].Resolved = true
			now := time.Now().UTC()
			feedback[i].ResolvedAt = &now
			project.Feedback[sessionID] = feedback
			return true
		}
	}
	return false
}

func New(repoPath string) (*Server, error) {
	static, err := webStaticFS()
	if err != nil {
		return nil, err
	}

	project, err := NewProject(repoPath)
	if err != nil {
		return nil, err
	}

	store := NewStore()
	store.Projects[project.ID] = &project
	server := &Server{defaultProjectID: project.ID, mux: http.NewServeMux(), store: store, static: static}
	server.routes()
	return server, nil
}

func NewWithStore(store *Store, static fs.FS) *Server {
	s := &Server{mux: http.NewServeMux(), store: store, static: static}
	if len(store.Projects) > 0 {
		for id := range store.Projects {
			s.defaultProjectID = id
			break
		}
	}
	s.routes()
	return s
}

func (s *Server) Store() *Store {
	return s.store
}

func (s *Server) Serve(listener net.Listener) error {
	srv := &http.Server{Handler: s.mux}
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()
	<-errCh
	return nil
}

func (s *Server) ServeWithShutdown(ctx context.Context, listener net.Listener) error {
	srv := &http.Server{Handler: s.mux}
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
		return <-errCh
	}
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
	s.mux.HandleFunc("PATCH /api/projects/{projectID}/sessions/{id}/feedback/{feedbackID}/resolve", s.projectResolveFeedback)
	s.mux.HandleFunc("GET /api/projects/{projectID}/sessions/{id}/agent-payload", s.projectAgentPayload)
	s.mux.HandleFunc("POST /api/projects/{projectID}/sessions/{id}/submit-review", s.projectSubmitReview)
	s.mux.HandleFunc("POST /api/projects/{projectID}/sessions/{id}/explanations", s.projectAddExplanation)
	s.mux.HandleFunc("GET /api/projects/{projectID}/sessions/{id}/explanations", s.projectListExplanations)
	s.mux.HandleFunc("GET /api/projects/{projectID}/sessions/{id}/explanation-requests", s.projectListExplanationRequests)

	s.mux.HandleFunc("GET /api/live", s.liveDiff)
	s.mux.HandleFunc("GET /api/sessions", s.listSessions)
	s.mux.HandleFunc("POST /api/sessions", s.createSession)
	s.mux.HandleFunc("DELETE /api/sessions/{id}", s.deleteSession)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.getSession)
	s.mux.HandleFunc("POST /api/sessions/{id}/explain", s.explain)
	s.mux.HandleFunc("POST /api/sessions/{id}/feedback", s.addFeedback)
	s.mux.HandleFunc("DELETE /api/sessions/{id}/feedback/{feedbackID}", s.deleteFeedback)
	s.mux.HandleFunc("PATCH /api/sessions/{id}/feedback/{feedbackID}/resolve", s.resolveFeedback)
	s.mux.HandleFunc("GET /api/sessions/{id}/agent-payload", s.agentPayload)
	s.mux.HandleFunc("POST /api/sessions/{id}/submit-review", s.submitReview)
	s.mux.HandleFunc("POST /api/sessions/{id}/explanations", s.addExplanation)
	s.mux.HandleFunc("GET /api/sessions/{id}/explanations", s.listExplanations)
	s.mux.HandleFunc("GET /api/sessions/{id}/explanation-requests", s.listExplanationRequests)

	s.mux.Handle("/", http.FileServer(http.FS(s.static)))
}

func WebStaticFS() (fs.FS, error) {
	return webStaticFS()
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
	project, err := NewProject(req.RepoPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	s.store.mu.Lock()
	existing, ok := s.store.Projects[project.ID]
	if ok {
		existing.LastPing = time.Now().UTC()
		project = *existing
	} else {
		s.store.Projects[project.ID] = &project
	}
	s.store.mu.Unlock()

	writeJSON(w, http.StatusOK, project)
}

func (s *Server) heartbeatProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectID")
	s.store.mu.Lock()
	project, ok := s.store.Projects[projectID]
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

	project, ok := s.store.Projects[projectID]
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
	if project := s.store.Projects[projectID]; project != nil {
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

	project, ok := s.store.Projects[projectID]
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
	sessionID := r.PathValue("id")
	_, ok := s.session(projectID, sessionID)
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

	explanationReq := ExplanationRequest{
		ID:        newID(),
		FilePath:  req.FilePath,
		StartLine: req.StartLine,
		EndLine:   req.EndLine,
		CreatedAt: time.Now().UTC(),
	}

	storedReq := explanationReq
	s.store.mu.Lock()
	if project := s.store.Projects[projectID]; project != nil {
		project.ExplanationReqs[sessionID] = append(project.ExplanationReqs[sessionID], explanationReq)
		storedReq = project.ExplanationReqs[sessionID][len(project.ExplanationReqs[sessionID])-1]
	}
	s.store.mu.Unlock()
	s.store.PublishReviewEvent(ReviewEvent{
		Type:               "explanation_requested",
		ProjectID:          projectID,
		SessionID:          sessionID,
		ExplanationRequest: &storedReq,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"summary": "Explanation requested — waiting for agent response.",
		"request": storedReq,
	})
}

func (s *Server) addFeedback(w http.ResponseWriter, r *http.Request) {
	s.addFeedbackFor(w, r, s.defaultProjectID)
}

func (s *Server) projectAddFeedback(w http.ResponseWriter, r *http.Request) {
	s.addFeedbackFor(w, r, r.PathValue("projectID"))
}

func (s *Server) resolveFeedback(w http.ResponseWriter, r *http.Request) {
	s.resolveFeedbackFor(w, r, s.defaultProjectID)
}

func (s *Server) projectResolveFeedback(w http.ResponseWriter, r *http.Request) {
	s.resolveFeedbackFor(w, r, r.PathValue("projectID"))
}

func (s *Server) resolveFeedbackFor(w http.ResponseWriter, r *http.Request, projectID string) {
	sessionID := r.PathValue("id")
	feedbackID := r.PathValue("feedbackID")

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	project := s.store.Projects[projectID]
	if project == nil {
		writeError(w, http.StatusNotFound, errors.New("project not found"))
		return
	}

	feedback := project.Feedback[sessionID]
	found := false
	for i := range feedback {
		if feedback[i].ID == feedbackID {
			feedback[i].Resolved = true
			now := time.Now().UTC()
			feedback[i].ResolvedAt = &now
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, errors.New("feedback not found"))
		return
	}
	project.Feedback[sessionID] = feedback
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
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
	if project := s.store.Projects[projectID]; project != nil {
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

	project := s.store.Projects[projectID]
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
			ID:         feedback.ID,
			FilePath:   feedback.FilePath,
			StartLine:  feedback.StartLine,
			EndLine:    feedback.EndLine,
			Body:       feedback.Body,
			CreatedAt:  feedback.CreatedAt,
			Resolved:   feedback.Resolved,
			ResolvedAt: feedback.ResolvedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string][]AgentComment{"comments": comments})
}

func (s *Server) submitReview(w http.ResponseWriter, r *http.Request) {
	s.submitReviewFor(w, r, s.defaultProjectID)
}

func (s *Server) projectSubmitReview(w http.ResponseWriter, r *http.Request) {
	s.submitReviewFor(w, r, r.PathValue("projectID"))
}

func (s *Server) submitReviewFor(w http.ResponseWriter, r *http.Request, projectID string) {
	session, ok := s.session(projectID, r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("session not found"))
		return
	}

	project, ok := s.project(projectID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("project not found"))
		return
	}

	comments := make([]AgentComment, 0, len(session.Feedback))
	for _, feedback := range session.Feedback {
		if feedback.Resolved {
			continue
		}
		comments = append(comments, AgentComment{
			ID:         feedback.ID,
			FilePath:   feedback.FilePath,
			StartLine:  feedback.StartLine,
			EndLine:    feedback.EndLine,
			Body:       feedback.Body,
			CreatedAt:  feedback.CreatedAt,
			Resolved:   feedback.Resolved,
			ResolvedAt: feedback.ResolvedAt,
		})
	}

	filePaths := make([]string, 0, len(session.Files))
	for _, f := range session.Files {
		filePaths = append(filePaths, f.Path)
	}

	review := SubmittedReview{
		SessionID:   session.ID,
		Comments:    comments,
		SubmittedAt: time.Now().UTC(),
		RepoPath:    project.RepoPath,
		Files:       filePaths,
		Summary:     session.Summary,
	}

	s.store.PublishReviewEvent(ReviewEvent{
		Type:      "review_submitted",
		ProjectID: projectID,
		SessionID: session.ID,
		Review:    &review,
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "submitted", "sessionId": session.ID, "commentCount": len(comments)})
}

func (s *Server) addExplanation(w http.ResponseWriter, r *http.Request) {
	s.addExplanationFor(w, r, s.defaultProjectID)
}

func (s *Server) projectAddExplanation(w http.ResponseWriter, r *http.Request) {
	s.addExplanationFor(w, r, r.PathValue("projectID"))
}

func (s *Server) addExplanationFor(w http.ResponseWriter, r *http.Request, projectID string) {
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
		writeError(w, http.StatusBadRequest, errors.New("explanation body is required"))
		return
	}

	explanation := Explanation{
		ID:        newID(),
		FilePath:  req.FilePath,
		StartLine: req.StartLine,
		EndLine:   req.EndLine,
		Body:      req.Body,
		CreatedAt: time.Now().UTC(),
	}

	s.store.mu.Lock()
	if project := s.store.Projects[projectID]; project != nil {
		project.Explanations[sessionID] = append(project.Explanations[sessionID], explanation)
		for i := range project.ExplanationReqs[sessionID] {
			r := &project.ExplanationReqs[sessionID][i]
			if r.FilePath == req.FilePath && r.StartLine == req.StartLine && r.EndLine == req.EndLine && !r.Resolved {
				r.Resolved = true
			}
		}
	}
	s.store.mu.Unlock()

	writeJSON(w, http.StatusCreated, explanation)
}

func (s *Server) listExplanations(w http.ResponseWriter, r *http.Request) {
	s.listExplanationsFor(w, r, s.defaultProjectID)
}

func (s *Server) projectListExplanations(w http.ResponseWriter, r *http.Request) {
	s.listExplanationsFor(w, r, r.PathValue("projectID"))
}

func (s *Server) listExplanationsFor(w http.ResponseWriter, r *http.Request, projectID string) {
	sessionID := r.PathValue("id")
	if _, ok := s.session(projectID, sessionID); !ok {
		writeError(w, http.StatusNotFound, errors.New("session not found"))
		return
	}

	s.store.mu.RLock()
	defer s.store.mu.RUnlock()
	project := s.store.Projects[projectID]
	explanations := project.Explanations[sessionID]
	if explanations == nil {
		explanations = []Explanation{}
	}
	writeJSON(w, http.StatusOK, explanations)
}

func (s *Server) listExplanationRequests(w http.ResponseWriter, r *http.Request) {
	s.listExplanationRequestsFor(w, r, s.defaultProjectID)
}

func (s *Server) projectListExplanationRequests(w http.ResponseWriter, r *http.Request) {
	s.listExplanationRequestsFor(w, r, r.PathValue("projectID"))
}

func (s *Server) listExplanationRequestsFor(w http.ResponseWriter, r *http.Request, projectID string) {
	sessionID := r.PathValue("id")
	if _, ok := s.session(projectID, sessionID); !ok {
		writeError(w, http.StatusNotFound, errors.New("session not found"))
		return
	}

	s.store.mu.RLock()
	defer s.store.mu.RUnlock()
	project := s.store.Projects[projectID]
	requests := project.ExplanationReqs[sessionID]
	if requests == nil {
		requests = []ExplanationRequest{}
	}
	writeJSON(w, http.StatusOK, requests)
}

func (s *Server) CreateSessionForProject(ctx context.Context, projectID string, req gitdiff.Request) (*Session, error) {
	s.store.mu.RLock()
	p, ok := s.store.Projects[projectID]
	s.store.mu.RUnlock()
	if !ok {
		return nil, errors.New("project not found")
	}
	req.RepoPath = p.RepoPath

	files, err := gitdiff.Load(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("load diff: %w", err)
	}

	session := Session{
		ID:        newID(),
		Title:     titleFor(req),
		CreatedAt: time.Now().UTC(),
		Request:   req,
		Files:     files,
		Summary:   gitdiff.Summary(files),
		Stats:     stats(files),
		Meta:      map[string]string{"repo": p.Repo, "projectID": p.ID},
	}

	s.store.mu.Lock()
	if proj := s.store.Projects[projectID]; proj != nil {
		proj.Sessions[session.ID] = session
	}
	s.store.mu.Unlock()

	return &session, nil
}

func (s *Server) project(id string) (Project, bool) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	project, ok := s.store.Projects[id]
	if !ok {
		return Project{}, false
	}
	return *project, true
}

func (s *Server) session(projectID string, id string) (Session, bool) {
	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	project, ok := s.store.Projects[projectID]
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

func NewProject(repoPath string) (Project, error) {
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return Project{}, err
	}
	if _, err := os.Stat(absRepo); err != nil {
		return Project{}, err
	}
	now := time.Now().UTC()
	return Project{
		ID:              projectID(absRepo),
		RepoPath:        absRepo,
		Repo:            filepath.Base(absRepo),
		CreatedAt:       now,
		LastPing:        now,
		Sessions:        map[string]Session{},
		Feedback:        map[string][]Feedback{},
		Explanations:    map[string][]Explanation{},
		ExplanationReqs: map[string][]ExplanationRequest{},
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
