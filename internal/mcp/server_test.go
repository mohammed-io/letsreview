package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func init() {
	openBrowser = func(string) {}
}

func handleMessageTest(srv *MCPServer, ctx context.Context, raw json.RawMessage) *jsonRPCResponse {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	var mu sync.Mutex
	return srv.handleMessage(ctx, raw, encoder, &mu)
}

func TestMCPInitialize(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	}))

	if resp.Error != nil {
		t.Fatalf("initialize error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if result["protocolVersion"] != protocolVersion {
		t.Fatalf("expected protocol version %s, got %v", protocolVersion, result["protocolVersion"])
	}
}

func TestMCPInitializeAdvertisesSubscriptions(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      100,
		"method":  "initialize",
		"params":  map[string]any{},
	}))

	result := resp.Result.(map[string]any)
	caps := result["capabilities"].(map[string]any)
	if _, ok := caps["subscriptions"]; !ok {
		t.Fatal("expected subscriptions capability")
	}
}

func TestMCPToolsListNoPollingTools(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}))

	result := resp.Result.(map[string]any)
	tools := result["tools"].([]map[string]any)
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool["name"].(string)
	}
	expected := []string{"request_code_review", "get_pending_events", "get_review_result", "cancel_review", "submit_explanation"}
	for _, name := range expected {
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected tool %q in %v", name, names)
		}
	}
	forbidden := []string{"wait_for_review_event", "wait_for_explanation_request", "get_explanation_requests", "subscribe_review_events", "unsubscribe_review_events"}
	for _, name := range forbidden {
		for _, n := range names {
			if n == name {
				t.Fatalf("forbidden tool %q still present", name)
			}
		}
	}
}

func TestMCPToolsDescriptionsMentionGetPendingEvents(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}))

	result := resp.Result.(map[string]any)
	tools := result["tools"].([]map[string]any)
	for _, tool := range tools {
		name := tool["name"].(string)
		desc := tool["description"].(string)
		switch name {
		case "request_code_review":
			if !strings.Contains(desc, "get_pending_events") {
				t.Fatalf("request_code_review must mention get_pending_events")
			}
		case "get_pending_events":
			if !strings.Contains(desc, "never blocks") {
				t.Fatalf("get_pending_events must say it never blocks")
			}
		}
	}
}

func TestMCPPing(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "ping",
	}))
	if resp.Error != nil {
		t.Fatalf("ping error: %s", resp.Error.Message)
	}
}

func TestMCPGetReviewResultNotSubmitted(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_review_result",
			"arguments": map[string]string{"sessionId": "nonexistent"},
		},
	}))
	text := toolResultText(t, resp)
	if !strings.Contains(text, "Error:") {
		t.Fatalf("expected error, got %s", text)
	}
}

func TestMCPUnknownTool(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "nonexistent_tool",
			"arguments": map[string]string{},
		},
	}))
	if resp.Error == nil {
		t.Fatal("expected error")
	}
}

func TestMCPRunReadsFromStdin(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	var output strings.Builder
	done := make(chan struct{})
	go func() {
		srv.Run(ctx, strings.NewReader(input), &output)
		close(done)
	}()
	<-done
	if !strings.Contains(output.String(), `"protocolVersion"`) {
		t.Fatalf("expected initialize response, got %s", output.String())
	}
}

func TestMCPRequestCodeReviewReturnsLastEventSeq(t *testing.T) {
	repo := makeMCPRepo(t)
	srv := NewMCPServer("127.0.0.1:0")
	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      50,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "request_code_review",
			"arguments": map[string]string{"repoPath": repo, "mode": "working"},
		},
	}))
	result := toolResultJSON(t, resp)
	if _, ok := result["lastEventSeq"]; !ok {
		t.Fatal("expected lastEventSeq in request_code_review response")
	}
}

func TestMCPGetPendingEventsReturnsEmpty(t *testing.T) {
	repo := makeMCPRepo(t)
	srv := NewMCPServer("127.0.0.1:0")
	start := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      60,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "request_code_review",
			"arguments": map[string]string{"repoPath": repo, "mode": "working"},
		},
	}))
	sessionID := toolResultJSON(t, start)["sessionId"].(string)
	lastSeq := toolResultJSON(t, start)["lastEventSeq"].(float64)

	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      61,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_pending_events",
			"arguments": map[string]any{
				"sessionId": sessionID,
				"afterSeq":  int64(lastSeq),
			},
		},
	}))

	result := toolResultJSON(t, resp)
	if result["count"].(float64) != 0 {
		t.Fatalf("expected 0 events, got %v", result["count"])
	}
}

func TestMCPGetPendingEventsReturnsReviewSubmitted(t *testing.T) {
	repo := makeMCPRepo(t)
	srv := NewMCPServer("127.0.0.1:0")
	start := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      70,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "request_code_review",
			"arguments": map[string]string{"repoPath": repo, "mode": "working"},
		},
	}))
	sessionID := toolResultJSON(t, start)["sessionId"].(string)
	lastSeq := int64(toolResultJSON(t, start)["lastEventSeq"].(float64))

	postMCPJSON(t, srv, "/api/sessions/"+sessionID+"/feedback", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   2,
		"body":      "Fix this",
	})
	postMCP(t, srv, "/api/sessions/"+sessionID+"/submit-review")

	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      71,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_pending_events",
			"arguments": map[string]any{
				"sessionId": sessionID,
				"afterSeq":  lastSeq,
			},
		},
	}))

	result := toolResultJSON(t, resp)
	events := result["events"].([]any)
	if len(events) == 0 {
		t.Fatal("expected at least 1 event")
	}
	first := events[0].(map[string]any)
	if first["type"] != "review_submitted" {
		t.Fatalf("expected review_submitted, got %v", first["type"])
	}
	newSeq := int64(result["lastSeq"].(float64))
	if newSeq <= lastSeq {
		t.Fatalf("expected lastSeq > %d, got %d", lastSeq, newSeq)
	}
}

func TestMCPGetPendingEventsReturnsExplanationRequested(t *testing.T) {
	repo := makeMCPRepo(t)
	srv := NewMCPServer("127.0.0.1:0")
	start := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      80,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "request_code_review",
			"arguments": map[string]string{"repoPath": repo, "mode": "working"},
		},
	}))
	sessionID := toolResultJSON(t, start)["sessionId"].(string)
	lastSeq := int64(toolResultJSON(t, start)["lastEventSeq"].(float64))

	postMCPJSON(t, srv, "/api/sessions/"+sessionID+"/explain", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   3,
	})

	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      81,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_pending_events",
			"arguments": map[string]any{
				"sessionId": sessionID,
				"afterSeq":  lastSeq,
			},
		},
	}))

	result := toolResultJSON(t, resp)
	events := result["events"].([]any)
	if len(events) == 0 {
		t.Fatal("expected at least 1 event")
	}
	first := events[0].(map[string]any)
	if first["type"] != "explanation_requested" {
		t.Fatalf("expected explanation_requested, got %v", first["type"])
	}
}

func TestMCPGetPendingEventsReturnsExplanationSubmitted(t *testing.T) {
	repo := makeMCPRepo(t)
	srv := NewMCPServer("127.0.0.1:0")
	start := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      90,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "request_code_review",
			"arguments": map[string]string{"repoPath": repo, "mode": "working"},
		},
	}))
	sessionID := toolResultJSON(t, start)["sessionId"].(string)
	lastSeq := int64(toolResultJSON(t, start)["lastEventSeq"].(float64))

	postMCPJSON(t, srv, "/api/sessions/"+sessionID+"/explain", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   3,
	})

	explainResp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      91,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "submit_explanation",
			"arguments": map[string]any{
				"sessionId":   sessionID,
				"filePath":    "main.go",
				"startLine":   1,
				"endLine":     3,
				"explanation": "This function returns a value.",
			},
		},
	}))
	if toolResultJSON(t, explainResp)["status"] != "submitted" {
		t.Fatal("expected explanation submitted")
	}

	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      92,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_pending_events",
			"arguments": map[string]any{
				"sessionId": sessionID,
				"afterSeq":  lastSeq,
			},
		},
	}))

	result := toolResultJSON(t, resp)
	events := result["events"].([]any)
	types := make(map[string]bool)
	for _, e := range events {
		et := e.(map[string]any)["type"].(string)
		types[et] = true
	}
	if !types["explanation_requested"] {
		t.Fatal("expected explanation_requested event")
	}
	if !types["explanation_submitted"] {
		t.Fatal("expected explanation_submitted event")
	}
}

func TestMCPGetPendingEventsIncrementsSeq(t *testing.T) {
	repo := makeMCPRepo(t)
	srv := NewMCPServer("127.0.0.1:0")
	start := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      95,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "request_code_review",
			"arguments": map[string]string{"repoPath": repo, "mode": "working"},
		},
	}))
	sessionID := toolResultJSON(t, start)["sessionId"].(string)
	lastSeq := int64(toolResultJSON(t, start)["lastEventSeq"].(float64))

	postMCP(t, srv, "/api/sessions/"+sessionID+"/submit-review")

	resp1 := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      96,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_pending_events",
			"arguments": map[string]any{
				"sessionId": sessionID,
				"afterSeq":  lastSeq,
			},
		},
	}))
	result1 := toolResultJSON(t, resp1)
	if result1["count"].(float64) != 1 {
		t.Fatalf("expected 1 event, got %v", result1["count"])
	}
	newSeq := int64(result1["lastSeq"].(float64))

	resp2 := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      97,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "get_pending_events",
			"arguments": map[string]any{
				"sessionId": sessionID,
				"afterSeq":  newSeq,
			},
		},
	}))
	result2 := toolResultJSON(t, resp2)
	if result2["count"].(float64) != 0 {
		t.Fatalf("expected 0 events after catchup, got %v", result2["count"])
	}
}

func TestMCPSubmitExplanationFailsForUnknownSession(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	resp := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      11,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "submit_explanation",
			"arguments": map[string]any{
				"sessionId":   "nonexistent",
				"filePath":    "main.go",
				"startLine":   1,
				"endLine":     3,
				"explanation": "test",
			},
		},
	}))
	text := toolResultText(t, resp)
	if !strings.Contains(text, "Error:") {
		t.Fatalf("expected error, got %s", text)
	}
}

func TestMCPSubscriptionsListenStillWorks(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	repo := makeMCPRepo(t)
	start := handleMessageTest(srv, context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      50,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "request_code_review",
			"arguments": map[string]string{"repoPath": repo, "mode": "working"},
		},
	}))
	sessionID := toolResultJSON(t, start)["sessionId"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	var writeMu sync.Mutex

	resp := srv.handleMessage(ctx, mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      200,
		"method":  "subscriptions/listen",
		"params": map[string]any{
			"notifications": map[string]any{
				"reviewEvents": true,
				"sessionIds":   []string{sessionID},
			},
		},
	}), encoder, &writeMu)

	if resp != nil {
		t.Fatalf("subscriptions/listen should return nil, got %+v", resp)
	}
	if !strings.Contains(output.String(), "notifications/subscriptions/acknowledged") {
		t.Fatalf("expected ack, got %s", output.String())
	}
	srv.removeAllSubscriptions()
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func toolResultJSON(t *testing.T, resp *jsonRPCResponse) map[string]any {
	t.Helper()
	text := toolResultText(t, resp)
	var value map[string]any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatalf("decode tool JSON %q: %v", text, err)
	}
	return value
}

func toolResultText(t *testing.T, resp *jsonRPCResponse) string {
	t.Helper()
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected content array, got %T", result["content"])
	}
	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatalf("expected text string, got %T", content[0]["text"])
	}
	return text
}

func makeMCPRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runMCP(t, dir, "git", "init")
	runMCP(t, dir, "git", "config", "user.email", "test@example.com")
	runMCP(t, dir, "git", "config", "user.name", "Test User")
	runMCP(t, dir, "git", "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc value() int { return 1 }\n"), 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}
	runMCP(t, dir, "git", "add", "main.go")
	runMCP(t, dir, "git", "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc value() int { return 2 }\nfunc added() bool { return true }\n"), 0o644); err != nil {
		t.Fatalf("write changed file: %v", err)
	}
	return dir
}

func runMCP(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
}

func postMCP(t *testing.T, srv *MCPServer, path string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	rec := httptest.NewRecorder()
	srv.httpServer.Handler().ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code > 299 {
		t.Fatalf("POST %s returned %d: %s", path, rec.Code, rec.Body.String())
	}
}

func postMCPJSON(t *testing.T, srv *MCPServer, path string, body any) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.httpServer.Handler().ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code > 299 {
		t.Fatalf("POST %s returned %d: %s", path, rec.Code, rec.Body.String())
	}
}
