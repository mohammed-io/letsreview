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
	"path/filepath"
	"sync"
	"time"

	"github.com/mohammed/letsreview/internal/gitdiff"
	"github.com/mohammed/letsreview/internal/server"
)

const protocolVersion = "2024-11-05"

type MCPServer struct {
	store      *server.Store
	httpServer *server.Server
	addr       string
	mu         sync.Mutex
	started    bool
}

func NewMCPServer(addr string) *MCPServer {
	return &MCPServer{
		store: server.NewStore(),
		addr:  addr,
	}
}

func (m *MCPServer) Run(ctx context.Context, stdin io.Reader, stdout io.Writer) {
	decoder := json.NewDecoder(stdin)
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	var writeMu sync.Mutex
	var wg sync.WaitGroup

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
			response := m.handleMessage(ctx, request)
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

func (m *MCPServer) handleMessage(ctx context.Context, raw json.RawMessage) *jsonRPCResponse {
	var req jsonRPCRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return &jsonRPCResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "Parse error"}}
	}

	if req.JSONRPC != "2.0" {
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32600, Message: "Invalid Request"}}
	}

	if len(req.ID) == 0 {
		return nil
	}

	switch req.Method {
	case "initialize":
		return m.handleInitialize(req)
	case "notifications/initialized", "notifications/cancelled":
		return nil
	case "ping":
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: struct{}{}}
	case "tools/list":
		return m.handleToolsList(req)
	case "tools/call":
		return m.handleToolsCall(ctx, req)
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
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": "letsreview", "version": "0.1.0"},
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
					"description": "Request a human code review for a repository. Starts a review session and returns a URL for the reviewer to open in their browser. Call wait_for_review_event to block until the reviewer asks for explanation or submits review comments.",
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
					"name":        "wait_for_review_event",
					"description": "Long-poll until the reviewer asks for explanation or submits the review. Returns explanation_requested, review_submitted, or timeout. Pass afterSeq from the previous event to wait for the next one.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"sessionId":      map[string]any{"type": "string", "description": "Session ID from request_code_review"},
							"afterSeq":       map[string]any{"type": "integer", "description": "Last event sequence already handled"},
							"timeoutSeconds": map[string]any{"type": "integer", "description": "How long to wait before returning timeout"},
						},
						"required": []string{"sessionId"},
					},
				},
				{
					"name":        "check_review_status",
					"description": "Check if a code review has been submitted. Returns 'pending' or 'submitted'.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"sessionId": map[string]any{"type": "string", "description": "Session ID from request_code_review"},
						},
						"required": []string{"sessionId"},
					},
				},
				{
					"name":        "get_review_result",
					"description": "Get the submitted review result with all comments. Call after check_review_status returns 'submitted'.",
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
					"name":        "get_explanation_requests",
					"description": "Get pending explanation requests from the reviewer. The reviewer selects lines and asks for explanation; the agent should respond with submit_explanation.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"sessionId": map[string]any{"type": "string", "description": "Session ID"},
						},
						"required": []string{"sessionId"},
					},
				},
				{
					"name":        "wait_for_explanation_request",
					"description": "Long-poll until the reviewer asks for an explanation. Returns one explanation request or timeout. Pass afterSeq from the previous response to wait for the next request.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"sessionId":      map[string]any{"type": "string", "description": "Session ID"},
							"afterSeq":       map[string]any{"type": "integer", "description": "Last event sequence already handled"},
							"timeoutSeconds": map[string]any{"type": "integer", "description": "How long to wait before returning timeout"},
						},
						"required": []string{"sessionId"},
					},
				},
				{
					"name":        "submit_explanation",
					"description": "Submit an explanation for specific lines of code. The explanation will appear inline in the reviewer's browser.",
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
	case "wait_for_review_event":
		result, err = m.toolWaitForReviewEvent(ctx, params.Arguments)
	case "check_review_status":
		result, err = m.toolCheckReviewStatus(params.Arguments)
	case "get_review_result":
		result, err = m.toolGetReviewResult(params.Arguments)
	case "cancel_review":
		result, err = m.toolCancelReview(params.Arguments)
	case "get_explanation_requests":
		result, err = m.toolGetExplanationRequests(params.Arguments)
	case "wait_for_explanation_request":
		result, err = m.toolWaitForExplanationRequest(ctx, params.Arguments)
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
	return map[string]any{
		"sessionId": session.ID,
		"url":       url,
		"projectID": project.ID,
		"files":     len(session.Files),
		"summary":   session.Summary,
	}, nil
}

func (m *MCPServer) toolCheckReviewStatus(raw json.RawMessage) (any, error) {
	var args struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	_, ok := m.store.GetSubmittedReview(args.SessionID)
	status := "pending"
	if ok {
		status = "submitted"
	}
	return map[string]string{"status": status, "sessionId": args.SessionID}, nil
}

func (m *MCPServer) toolWaitForReviewEvent(ctx context.Context, raw json.RawMessage) (any, error) {
	var args struct {
		SessionID      string `json:"sessionId"`
		AfterSeq       int64  `json:"afterSeq"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
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

	timeout := normalizedTimeout(args.TimeoutSeconds)
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	event, ok := m.store.WaitForReviewEvent(waitCtx, args.SessionID, args.AfterSeq)
	if !ok {
		return map[string]any{"status": "timeout", "sessionId": args.SessionID, "afterSeq": args.AfterSeq}, nil
	}
	return map[string]any{"status": "event", "event": event}, nil
}

func (m *MCPServer) toolGetReviewResult(raw json.RawMessage) (any, error) {
	var args struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	review, ok := m.store.GetSubmittedReview(args.SessionID)
	if !ok {
		return nil, fmt.Errorf("review not yet submitted for session %s", args.SessionID)
	}
	return review, nil
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

func (m *MCPServer) toolGetExplanationRequests(raw json.RawMessage) (any, error) {
	var args struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	requests := m.store.GetExplanationRequests(args.SessionID)
	if requests == nil {
		requests = []server.ExplanationRequest{}
	}
	return map[string]any{"sessionId": args.SessionID, "requests": requests}, nil
}

func (m *MCPServer) toolWaitForExplanationRequest(ctx context.Context, raw json.RawMessage) (any, error) {
	var args struct {
		SessionID      string `json:"sessionId"`
		AfterSeq       int64  `json:"afterSeq"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
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

	timeout := normalizedTimeout(args.TimeoutSeconds)
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	afterSeq := args.AfterSeq
	for {
		event, ok := m.store.WaitForReviewEvent(waitCtx, args.SessionID, afterSeq)
		if !ok {
			return map[string]any{"status": "timeout", "sessionId": args.SessionID, "afterSeq": afterSeq}, nil
		}
		afterSeq = event.Seq
		if event.Type != "explanation_requested" || event.ExplanationRequest == nil {
			continue
		}
		return map[string]any{
			"status":             "explanation_requested",
			"seq":                event.Seq,
			"sessionId":          args.SessionID,
			"projectID":          event.ProjectID,
			"explanationRequest": event.ExplanationRequest,
		}, nil
	}
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
	return map[string]any{"status": "submitted", "sessionId": args.SessionID, "explanationId": explanation.ID}, nil
}

func normalizedTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return time.Hour
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
