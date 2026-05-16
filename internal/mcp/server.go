package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/mohammed-io/letsreview/internal/gitdiff"
	"github.com/mohammed-io/letsreview/internal/server"
"github.com/mohammed-io/letsreview"
)

const protocolVersion = "2024-11-05"

type subscription struct {
	id         string
	filters    subscriptionFilters
	eventCh    chan server.ReviewEvent
	cancelCh   chan struct{}
	sessionIDs map[string]bool
}

type subscriptionFilters struct {
	reviewEvents          bool
	explanationRequests   bool
	allSessions           bool
	sessionIDs            map[string]bool
}

type MCPServer struct {
	store        *server.Store
	httpServer   *server.Server
	addr         string
	mu           sync.Mutex
	started      bool
	subMu        sync.RWMutex
	subscriptions map[string]*subscription
	writeNotify  func(json.RawMessage)
}

func NewMCPServer(addr string) *MCPServer {
	m := &MCPServer{
		store:         server.NewStore(),
		addr:          addr,
		subscriptions: map[string]*subscription{},
	}
	m.store.OnEventPublished = m.publishToSubscriptions
	return m
}

func (m *MCPServer) Run(ctx context.Context, stdin io.Reader, stdout io.Writer) {
	decoder := json.NewDecoder(stdin)
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	var writeMu sync.Mutex
	var wg sync.WaitGroup

	m.writeNotify = func(msg json.RawMessage) {
		writeMu.Lock()
		defer writeMu.Unlock()
		if err := encoder.Encode(msg); err != nil {
			log.Printf("mcp: notification encode error: %v", err)
		}
	}

	defer m.removeAllSubscriptions()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		default:
		}

		var request json.RawMessage
		if err := decoder.Decode(&request); err != nil {
			if ctx.Err() != nil {
				wg.Wait()
				return
			}
			log.Printf("mcp: decode error: %v", err)
			wg.Wait()
			return
		}

		wg.Add(1)
		go func(request json.RawMessage) {
			defer wg.Done()
			response := m.handleMessage(ctx, request, encoder, &writeMu)
			if response == nil {
				return
			}
			writeMu.Lock()
			defer writeMu.Unlock()
			if err := encoder.Encode(response); err != nil {
				log.Printf("mcp: encode error: %v", err)
			}
		}(request)
	}
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (m *MCPServer) handleMessage(ctx context.Context, raw json.RawMessage, encoder *json.Encoder, writeMu *sync.Mutex) *jsonRPCResponse {
	var req jsonRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return &jsonRPCResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "Parse error"}}
	}

	if req.JSONRPC != "2.0" {
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32600, Message: "Invalid Request"}}
	}

	if len(req.ID) == 0 {
		if req.Method == "notifications/cancelled" {
			m.handleNotificationCancelled(req)
		}
		return nil
	}

	switch req.Method {
	case "initialize":
		return m.handleInitialize(req)
	case "notifications/initialized":
		return nil
	case "ping":
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: struct{}{}}
	case "tools/list":
		return m.handleToolsList(req)
	case "tools/call":
		return m.handleToolsCall(ctx, req)
	case "subscriptions/listen":
		return m.handleSubscriptionsListen(ctx, req, encoder, writeMu)
	default:
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}}
	}
}

func (m *MCPServer) handleInitialize(req jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools":         map[string]any{},
				"subscriptions": map[string]any{},
			},
			"serverInfo": map[string]string{"name": "letsreview", "version": letsreview.Version},
		},
	}
}

func (m *MCPServer) handleToolsList(req jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tools": []map[string]any{
				{
					"name":        "request_code_review",
					"description": "Start a human code review session. Opens a browser UI for the reviewer and returns a session ID with the last event sequence (lastEventSeq). IMPORTANT: After this, periodically call get_pending_events with the sessionId and lastEventSeq to check for new events. The reviewer may ask for explanations (explanation_requested) or submit their review (review_submitted). You can call get_pending_events between responding to the user — it never blocks.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"repoPath": map[string]any{"type": "string", "description": "Absolute path to the git repository to review"},
							"mode":     map[string]any{"type": "string", "enum": []string{"working", "staged", "refs"}, "description": "Diff mode: working tree vs HEAD, staged changes, or two refs"},
							"baseRef":  map[string]any{"type": "string", "description": "Base ref for refs mode"},
							"headRef":  map[string]any{"type": "string", "description": "Head ref for refs mode"},
						},
						"required": []string{"repoPath"},
					},
				},
				{
					"name":        "get_pending_events",
					"description": "Check for new review events since the last call. Returns immediately — never blocks. Call this periodically between user interactions to detect: explanation_requested (reviewer asks about code), explanation_submitted (agent explanation saved), or review_submitted (reviewer finished). Pass afterSeq from the last event you processed (initially use lastEventSeq from request_code_review). On each call, update your afterSeq to the latest seq from the returned events.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"sessionId": map[string]any{"type": "string", "description": "Session ID from request_code_review"},
							"afterSeq":  map[string]any{"type": "integer", "description": "Last event sequence number processed. Use lastEventSeq from request_code_review initially, then update from returned events."},
						},
						"required": []string{"sessionId", "afterSeq"},
					},
				},
				{
					"name":        "get_review_result",
					"description": "Get the latest submitted review result with all comments. Call after get_pending_events returns a review_submitted event. Returns the most recent review for the session. Multiple reviews can be submitted per session. Each comment includes an `id` field — after applying a comment's change, call `resolve_feedback` with that id to mark it resolved.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"sessionId": map[string]any{"type": "string", "description": "Session ID from request_code_review"},
						},
						"required": []string{"sessionId"},
					},
				},
				{
					"name":        "cancel_review",
					"description": "Cancel a pending review session.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"sessionId": map[string]any{"type": "string", "description": "Session ID to cancel"},
						},
						"required": []string{"sessionId"},
					},
				},
				{
					"name":        "resolve_feedback",
					"description": "Mark a feedback comment as resolved after the AI agent has applied the requested change. The comment remains visible in the web UI with a resolved indicator. Submit review only sends unresolved comments.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"sessionId":  map[string]any{"type": "string", "description": "Session ID"},
							"commentId":  map[string]any{"type": "string", "description": "The ID of the comment/feedback to resolve. Use the id field from the comment object."},
						},
						"required": []string{"sessionId", "commentId"},
					},
				},
				{
					"name":        "submit_explanation",
					"description": "Submit an explanation for specific lines of code. Call this when get_pending_events returns an explanation_requested event. The explanation will appear inline in the reviewer's browser.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"sessionId":   map[string]any{"type": "string", "description": "Session ID"},
							"filePath":    map[string]any{"type": "string", "description": "File path"},
							"startLine":   map[string]any{"type": "integer", "description": "Start line"},
							"endLine":     map[string]any{"type": "integer", "description": "End line"},
							"explanation": map[string]any{"type": "string", "description": "The explanation text"},
						},
						"required": []string{"sessionId", "filePath", "startLine", "endLine", "explanation"},
					},
				},
			},
		},
	}
}

func (m *MCPServer) handleToolsCall(ctx context.Context, req jsonRPCRequest) *jsonRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return rpcErr(req.ID, -32602, "Invalid params")
	}

	var result any
	var err error

	switch params.Name {
	case "request_code_review":
		result, err = m.toolRequestCodeReview(ctx, params.Arguments)
	case "get_pending_events":
		result, err = m.toolGetPendingEvents(params.Arguments)
	case "get_review_result":
		result, err = m.toolGetReviewResult(params.Arguments)
	case "cancel_review":
		result, err = m.toolCancelReview(params.Arguments)
	case "resolve_feedback":
		result, err = m.toolResolveFeedback(params.Arguments)
	case "submit_explanation":
		result, err = m.toolSubmitExplanation(params.Arguments)
	default:
		return rpcErr(req.ID, -32602, fmt.Sprintf("Unknown tool: %s", params.Name))
	}

	if err != nil {
		return toolError(req.ID, err.Error())
	}

	return toolResult(req.ID, result)
}

func (m *MCPServer) handleSubscriptionsListen(ctx context.Context, req jsonRPCRequest, encoder *json.Encoder, writeMu *sync.Mutex) *jsonRPCResponse {
	var params struct {
		Notifications json.RawMessage `json:"notifications"`
	}
	if len(req.Params) > 0 {
		_ = json.Unmarshal(req.Params, &params)
	}

	var id string
	if err := json.Unmarshal(req.ID, &id); err != nil {
		id = string(req.ID)
	}
	if id == "" {
		return rpcErr(req.ID, -32602, "subscription request must have an ID")
	}

	filters := parseSubscriptionFilters(params.Notifications)
	sub := &subscription{
		id:         id,
		filters:    filters,
		eventCh:    make(chan server.ReviewEvent, 64),
		cancelCh:   make(chan struct{}),
		sessionIDs: filters.sessionIDs,
	}

	m.subMu.Lock()
	if existing, ok := m.subscriptions[id]; ok {
		close(existing.cancelCh)
		delete(m.subscriptions, id)
	}
	m.subscriptions[id] = sub
	m.subMu.Unlock()

	ackParams := map[string]any{
		"_meta": map[string]any{
			"io.modelcontextprotocol/subscriptionId": id,
		},
		"notifications": buildAckNotifications(filters),
	}
	ackMsg, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/subscriptions/acknowledged",
		"params":  ackParams,
	})

	writeMu.Lock()
	if err := encoder.Encode(json.RawMessage(ackMsg)); err != nil {
		writeMu.Unlock()
		m.removeSubscription(id)
		return rpcErr(req.ID, -32603, "failed to send acknowledgment")
	}
	writeMu.Unlock()

	go m.subscriptionLoop(ctx, sub, encoder, writeMu)

	return nil
}

func (m *MCPServer) handleNotificationCancelled(req jsonRPCRequest) {
	var params struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if len(req.Params) > 0 {
		_ = json.Unmarshal(req.Params, &params)
	}
	var id string
	if len(params.RequestID) > 0 {
		if err := json.Unmarshal(params.RequestID, &id); err != nil {
			id = string(params.RequestID)
		}
	}
	if id != "" {
		m.removeSubscription(id)
	}
}

func (m *MCPServer) subscriptionLoop(ctx context.Context, sub *subscription, encoder *json.Encoder, writeMu *sync.Mutex) {
	defer m.removeSubscriptionIfMatch(sub.id, sub)

	afterSeq := int64(0)
	if !sub.filters.allSessions {
		for sid := range sub.sessionIDs {
			events := m.store.GetEventsAfter(sid, 0)
			if len(events) > 0 {
				afterSeq = events[len(events)-1].Seq
			}
		}
	}
	_ = afterSeq

	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.cancelCh:
			return
		case event, ok := <-sub.eventCh:
			if !ok {
				return
			}
			if !m.matchSubscription(sub, event) {
				continue
			}
			notif := buildReviewNotification(sub.id, event)
			if notif == nil {
				continue
			}
			msg, _ := json.Marshal(notif)
			writeMu.Lock()
			if err := encoder.Encode(json.RawMessage(msg)); err != nil {
				writeMu.Unlock()
				return
			}
			writeMu.Unlock()
		}
	}
}

func (m *MCPServer) matchSubscription(sub *subscription, event server.ReviewEvent) bool {
	if !sub.filters.allSessions {
		if !sub.sessionIDs[event.SessionID] {
			return false
		}
	}

	switch event.Type {
	case "review_submitted":
		return sub.filters.reviewEvents
	case "explanation_requested":
		return sub.filters.explanationRequests
	case "explanation_submitted":
		return sub.filters.explanationRequests
	default:
		return sub.filters.reviewEvents
	}
}

func (m *MCPServer) removeSubscription(id string) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	if sub, ok := m.subscriptions[id]; ok {
		select {
		case <-sub.cancelCh:
		default:
			close(sub.cancelCh)
		}
		delete(m.subscriptions, id)
	}
}

func (m *MCPServer) removeSubscriptionIfMatch(id string, sub *subscription) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	if current, ok := m.subscriptions[id]; ok && current == sub {
		select {
		case <-sub.cancelCh:
		default:
			close(sub.cancelCh)
		}
		delete(m.subscriptions, id)
	}
}

func (m *MCPServer) removeAllSubscriptions() {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	for id, sub := range m.subscriptions {
		select {
		case <-sub.cancelCh:
		default:
			close(sub.cancelCh)
		}
		delete(m.subscriptions, id)
	}
}

func (m *MCPServer) publishToSubscriptions(event server.ReviewEvent) {
	m.subMu.RLock()
	defer m.subMu.RUnlock()
	for _, sub := range m.subscriptions {
		if m.matchSubscription(sub, event) {
			select {
			case sub.eventCh <- event:
			default:
				log.Printf("mcp: subscription %s event channel full, dropping event", sub.id)
			}
		}
	}
}

func parseSubscriptionFilters(raw json.RawMessage) subscriptionFilters {
	f := subscriptionFilters{
		allSessions: true,
		sessionIDs:  map[string]bool{},
	}
	if len(raw) == 0 {
		f.reviewEvents = true
		f.explanationRequests = true
		return f
	}

	var parsed struct {
		ReviewEvents        bool     `json:"reviewEvents"`
		ExplanationRequests bool     `json:"explanationRequests"`
		SessionIDs          []string `json:"sessionIds"`
	}
	_ = json.Unmarshal(raw, &parsed)

	f.reviewEvents = parsed.ReviewEvents || !parsed.ReviewEvents && !parsed.ExplanationRequests && len(parsed.SessionIDs) == 0
	f.explanationRequests = parsed.ExplanationRequests || !parsed.ReviewEvents && !parsed.ExplanationRequests && len(parsed.SessionIDs) == 0

	if len(parsed.SessionIDs) > 0 {
		f.allSessions = false
		for _, sid := range parsed.SessionIDs {
			f.sessionIDs[sid] = true
		}
	}

	return f
}

func buildAckNotifications(f subscriptionFilters) map[string]any {
	ack := map[string]any{}
	if f.reviewEvents {
		ack["reviewEvents"] = true
	}
	if f.explanationRequests {
		ack["explanationRequests"] = true
	}
	return ack
}

func buildReviewNotification(subID string, event server.ReviewEvent) map[string]any {
	method := ""
	params := map[string]any{
		"_meta": map[string]any{
			"io.modelcontextprotocol/subscriptionId": subID,
		},
		"sessionId": event.SessionID,
		"projectId": event.ProjectID,
		"timestamp": event.CreatedAt.Format(time.RFC3339),
		"seq":       event.Seq,
	}

	switch event.Type {
	case "review_submitted":
		method = "notifications/review/submitted"
		if event.Review != nil {
			params["commentCount"] = len(event.Review.Comments)
			params["review"] = event.Review
		}
	case "explanation_requested":
		method = "notifications/review/explanation_requested"
		if event.ExplanationRequest != nil {
			params["explanationRequest"] = event.ExplanationRequest
		}
	case "explanation_submitted":
		method = "notifications/review/explanation_submitted"
		if event.Explanation != nil {
			params["explanation"] = event.Explanation
		}
	default:
		method = "notifications/review/event"
		params["type"] = event.Type
	}

	return map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
}

func (m *MCPServer) ensureHTTPServer(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return nil
	}

	static, err := server.WebStaticFS()
	if err != nil {
		return fmt.Errorf("load static files: %w", err)
	}

	m.httpServer = server.NewWithStore(m.store, static)
	listener, err := net.Listen("tcp", m.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", m.addr, err)
	}
	m.addr = listener.Addr().String()

	go func() {
		if err := m.httpServer.Serve(listener); err != nil {
			log.Printf("mcp: http server error: %v", err)
		}
	}()

	m.started = true
	return nil
}

func (m *MCPServer) toolRequestCodeReview(ctx context.Context, raw json.RawMessage) (any, error) {
	var args struct {
		RepoPath string `json:"repoPath"`
		Mode     string `json:"mode"`
		BaseRef  string `json:"baseRef"`
		HeadRef  string `json:"headRef"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	absRepo, err := filepath.Abs(args.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	project, err := server.NewProject(absRepo)
	if err != nil {
		return nil, fmt.Errorf("invalid repo: %w", err)
	}

	m.store.RegisterProject(project)

	if err := m.ensureHTTPServer(ctx); err != nil {
		return nil, err
	}

	mode := gitdiff.Mode(args.Mode)
	if mode == "" {
		mode = gitdiff.ModeWorking
	}
	req := gitdiff.Request{Mode: mode, BaseRef: args.BaseRef, HeadRef: args.HeadRef}

	session, err := m.httpServer.CreateSessionForProject(ctx, project.ID, req)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	url := fmt.Sprintf("http://%s?project=%s&session=%s", m.addr, project.ID, session.ID)
	openBrowser(url)
	lastSeq := m.store.LastEventSeq()
	return map[string]any{
		"sessionId":    session.ID,
		"url":          url,
		"projectID":    project.ID,
		"files":        len(session.Files),
		"summary":      session.Summary,
		"lastEventSeq": lastSeq,
	}, nil
}

func (m *MCPServer) toolGetPendingEvents(raw json.RawMessage) (any, error) {
	var args struct {
		SessionID string `json:"sessionId"`
		AfterSeq  int64  `json:"afterSeq"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if args.SessionID == "" {
		return nil, fmt.Errorf("sessionId is required")
	}
	if m.store.FindProjectForSession(args.SessionID) == "" {
		return nil, fmt.Errorf("session %s not found", args.SessionID)
	}

	events := m.store.GetEventsAfter(args.SessionID, args.AfterSeq)
	if events == nil {
		events = []server.ReviewEvent{}
	}

	lastSeq := args.AfterSeq
	for _, e := range events {
		if e.Seq > lastSeq {
			lastSeq = e.Seq
		}
	}

	return map[string]any{
		"sessionId":  args.SessionID,
		"events":     events,
		"count":      len(events),
		"lastSeq":    lastSeq,
	}, nil
}

func (m *MCPServer) toolGetReviewResult(raw json.RawMessage) (any, error) {
	var args struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	events := m.store.GetEventsAfter(args.SessionID, 0)
	var latest *server.SubmittedReview
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == "review_submitted" && events[i].Review != nil {
			latest = events[i].Review
			break
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("no review submitted yet for session %s", args.SessionID)
	}
	return latest, nil
}

func (m *MCPServer) toolCancelReview(raw json.RawMessage) (any, error) {
	var args struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	return map[string]string{"status": "cancelled", "sessionId": args.SessionID}, nil
}

func (m *MCPServer) toolResolveFeedback(raw json.RawMessage) (any, error) {
	var args struct {
		SessionID string `json:"sessionId"`
		CommentID string `json:"commentId"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	projectID := m.store.FindProjectForSession(args.SessionID)
	if projectID == "" {
		return nil, fmt.Errorf("session %s not found", args.SessionID)
	}

	if !m.store.ResolveFeedback(projectID, args.SessionID, args.CommentID) {
		return nil, fmt.Errorf("comment %s not found in session %s", args.CommentID, args.SessionID)
	}

	return map[string]string{"status": "resolved", "sessionId": args.SessionID, "commentId": args.CommentID}, nil
}

func (m *MCPServer) toolSubmitExplanation(raw json.RawMessage) (any, error) {
	var args struct {
		SessionID   string `json:"sessionId"`
		FilePath    string `json:"filePath"`
		StartLine   int    `json:"startLine"`
		EndLine     int    `json:"endLine"`
		Explanation string `json:"explanation"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	projectID := m.store.FindProjectForSession(args.SessionID)
	if projectID == "" {
		return nil, fmt.Errorf("session %s not found", args.SessionID)
	}

	explanation := server.Explanation{
		ID:        generateID(),
		FilePath:  args.FilePath,
		StartLine: args.StartLine,
		EndLine:   args.EndLine,
		Body:      args.Explanation,
		CreatedAt: time.Now().UTC(),
	}

	m.store.AddExplanation(projectID, args.SessionID, explanation)
	m.store.PublishReviewEvent(server.ReviewEvent{
		Type:        "explanation_submitted",
		ProjectID:   projectID,
		SessionID:   args.SessionID,
		Explanation: &explanation,
	})
	return map[string]any{"status": "submitted", "sessionId": args.SessionID, "explanationId": explanation.ID}, nil
}

func normalizedTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return 15 * time.Minute
	}
	if seconds > 86400 {
		seconds = 86400
	}
	return time.Duration(seconds) * time.Second
}

func generateID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(bytes[:])
}

func rpcErr(id json.RawMessage, code int, msg string) *jsonRPCResponse {
	return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

func toolResult(id json.RawMessage, data any) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": toJSON(data)},
			},
		},
	}
}

func toolError(id json.RawMessage, msg string) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": fmt.Sprintf("Error: %s", msg)},
			},
			"isError": true,
		},
	}
}

func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(b)
}

var openBrowser = func(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
