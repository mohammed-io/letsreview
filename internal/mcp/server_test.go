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
	"testing"
	"time"
)

func init() {
	openBrowser = func(string) {}
}

func TestMCPInitialize(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
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

func TestMCPToolsList(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}))

	if resp.Error != nil {
		t.Fatalf("tools/list error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	tools, ok := result["tools"].([]map[string]any)
	if !ok {
		t.Fatalf("expected tools array, got %T", result["tools"])
	}
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool["name"].(string)
	}
	expected := []string{"request_code_review", "wait_for_review_event", "wait_for_explanation_request", "check_review_status", "get_review_result", "cancel_review"}
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
}

func TestMCPPing(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "ping",
	}))
	if resp.Error != nil {
		t.Fatalf("ping error: %s", resp.Error.Message)
	}
}

func TestMCPCheckReviewStatus(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")

	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "check_review_status",
			"arguments": map[string]string{"sessionId": "nonexistent"},
		},
	}))

	if resp.Error != nil {
		t.Fatalf("check_review_status error: %s", resp.Error.Message)
	}

	text := toolResultText(t, resp)
	if !strings.Contains(text, `"pending"`) {
		t.Fatalf("expected pending status, got %s", text)
	}
}

func TestMCPGetReviewResultNotSubmitted(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")

	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_review_result",
			"arguments": map[string]string{"sessionId": "nonexistent"},
		},
	}))

	if resp.Error != nil {
		t.Fatalf("get_review_result error: %s", resp.Error.Message)
	}

	text := toolResultText(t, resp)
	if !strings.Contains(text, "Error:") {
		t.Fatalf("expected error for unsubmitted review, got %s", text)
	}
}

func TestMCPUnknownTool(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")

	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "nonexistent_tool",
			"arguments": map[string]string{},
		},
	}))

	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestMCPMethodNotFound(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")

	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "nonexistent/method",
	}))

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("expected -32601, got %d", resp.Error.Code)
	}
}

func TestMCPNotificationReturnsNil(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")

	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}))
	if resp != nil {
		t.Fatal("expected nil for notification")
	}
}

func TestMCPRunReadsFromStdin(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
`
	reader := strings.NewReader(input)
	var output strings.Builder

	done := make(chan struct{})
	go func() {
		srv.Run(ctx, reader, &output)
		close(done)
	}()

	<-done

	if output.Len() == 0 {
		t.Fatal("expected output from Run")
	}
	if !strings.Contains(output.String(), `"protocolVersion"`) {
		t.Fatalf("expected initialize response, got %s", output.String())
	}
}

func TestMCPGetExplanationRequestsEmpty(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")

	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "get_explanation_requests",
			"arguments": map[string]string{"sessionId": "nonexistent"},
		},
	}))

	text := toolResultText(t, resp)
	if !strings.Contains(text, `"requests"`) {
		t.Fatalf("expected requests field, got %s", text)
	}
}

func TestMCPSubmitExplanationFailsForUnknownSession(t *testing.T) {
	srv := NewMCPServer("127.0.0.1:0")

	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
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
		t.Fatalf("expected error for unknown session, got %s", text)
	}
}

func TestMCPWaitForReviewEventReturnsSubmittedReview(t *testing.T) {
	repo := makeMCPRepo(t)
	srv := NewMCPServer("127.0.0.1:0")
	start := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      12,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "request_code_review",
			"arguments": map[string]string{"repoPath": repo, "mode": "working"},
		},
	}))
	sessionID := toolResultJSON(t, start)["sessionId"].(string)

	postMCPJSON(t, srv, "/api/sessions/"+sessionID+"/feedback", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   2,
		"body":      "Fix this before commit.",
	})

	done := make(chan *jsonRPCResponse, 1)
	go func() {
		done <- srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
			"jsonrpc": "2.0",
			"id":      13,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "wait_for_review_event",
				"arguments": map[string]any{
					"sessionId":      sessionID,
					"timeoutSeconds": 2,
				},
			},
		}))
	}()

	postMCP(t, srv, "/api/sessions/"+sessionID+"/submit-review")

	select {
	case resp := <-done:
		event := toolResultJSON(t, resp)["event"].(map[string]any)
		if event["type"] != "review_submitted" {
			t.Fatalf("expected review_submitted event, got %#v", event)
		}
		review := event["review"].(map[string]any)
		comments := review["comments"].([]any)
		if len(comments) != 1 {
			t.Fatalf("expected submitted comments, got %#v", comments)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for review event")
	}
}

func TestMCPWaitForReviewEventReturnsExplanationRequest(t *testing.T) {
	repo := makeMCPRepo(t)
	srv := NewMCPServer("127.0.0.1:0")
	start := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      14,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "request_code_review",
			"arguments": map[string]string{"repoPath": repo, "mode": "working"},
		},
	}))
	sessionID := toolResultJSON(t, start)["sessionId"].(string)

	postMCPJSON(t, srv, "/api/sessions/"+sessionID+"/explain", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   3,
	})

	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      15,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "wait_for_review_event",
			"arguments": map[string]any{
				"sessionId":      sessionID,
				"timeoutSeconds": 1,
			},
		},
	}))

	event := toolResultJSON(t, resp)["event"].(map[string]any)
	if event["type"] != "explanation_requested" {
		t.Fatalf("expected explanation_requested event, got %#v", event)
	}
	req := event["explanationRequest"].(map[string]any)
	if req["filePath"] != "main.go" {
		t.Fatalf("expected explanation request file, got %#v", req)
	}
}

func TestMCPWaitForExplanationRequestReturnsSelectedRange(t *testing.T) {
	repo := makeMCPRepo(t)
	srv := NewMCPServer("127.0.0.1:0")
	start := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      16,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "request_code_review",
			"arguments": map[string]string{"repoPath": repo, "mode": "working"},
		},
	}))
	sessionID := toolResultJSON(t, start)["sessionId"].(string)

	done := make(chan *jsonRPCResponse, 1)
	go func() {
		done <- srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
			"jsonrpc": "2.0",
			"id":      17,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "wait_for_explanation_request",
				"arguments": map[string]any{
					"sessionId":      sessionID,
					"timeoutSeconds": 2,
				},
			},
		}))
	}()

	postMCPJSON(t, srv, "/api/sessions/"+sessionID+"/explain", map[string]any{
		"filePath":  "main.go",
		"startLine": 2,
		"endLine":   4,
	})

	select {
	case resp := <-done:
		result := toolResultJSON(t, resp)
		if result["status"] != "explanation_requested" {
			t.Fatalf("expected explanation_requested status, got %#v", result)
		}
		req := result["explanationRequest"].(map[string]any)
		if req["filePath"] != "main.go" || req["startLine"] != float64(2) || req["endLine"] != float64(4) {
			t.Fatalf("expected selected range in request, got %#v", req)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for explanation request")
	}
}

func TestMCPWaitForExplanationRequestIgnoresReviewSubmitAndTimesOut(t *testing.T) {
	repo := makeMCPRepo(t)
	srv := NewMCPServer("127.0.0.1:0")
	start := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      18,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "request_code_review",
			"arguments": map[string]string{"repoPath": repo, "mode": "working"},
		},
	}))
	sessionID := toolResultJSON(t, start)["sessionId"].(string)
	postMCP(t, srv, "/api/sessions/"+sessionID+"/submit-review")

	resp := srv.handleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      19,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "wait_for_explanation_request",
			"arguments": map[string]any{
				"sessionId":      sessionID,
				"timeoutSeconds": 1,
			},
		},
	}))

	result := toolResultJSON(t, resp)
	if result["status"] != "timeout" {
		t.Fatalf("expected timeout after non-explanation event, got %#v", result)
	}
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
