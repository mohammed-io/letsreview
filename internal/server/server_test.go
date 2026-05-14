package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionFeedbackAndAgentPayload(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	session := postJSON[Session](t, app.Handler(), "/api/sessions", map[string]string{"mode": "working"})
	if session.ID == "" {
		t.Fatal("expected session id")
	}
	if session.Stats["files"] != 1 || session.Stats["additions"] == 0 {
		t.Fatalf("expected changed file stats, got %#v", session.Stats)
	}

	feedback := postJSON[Feedback](t, app.Handler(), "/api/sessions/"+session.ID+"/feedback", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   2,
		"body":      "Rename this for clarity.",
	})
	if feedback.Body != "Rename this for clarity." {
		t.Fatalf("expected feedback body, got %q", feedback.Body)
	}

	payload := getJSON[map[string]any](t, app.Handler(), "/api/sessions/"+session.ID+"/agent-payload")
	if payload["repoPath"] != repo {
		t.Fatalf("expected repo path in payload, got %#v", payload["repoPath"])
	}
	if !strings.Contains(payload["directive"].(string), "Apply requested changes") {
		t.Fatalf("expected agent directive, got %#v", payload["directive"])
	}
}

func TestCreateSessionRejectsPlainDirectoryWithoutInitializingGit(t *testing.T) {
	dir := t.TempDir()
	app, err := New(dir)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sessions", bytes.NewReader([]byte(`{"mode":"working"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for non-git directory, got %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Fatalf("expected no .git directory to be created, stat error: %v", err)
	}
}

func makeRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "Test User")
	run(t, dir, "git", "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc value() int { return 1 }\n"), 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}
	run(t, dir, "git", "add", "main.go")
	run(t, dir, "git", "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc value() int { return 2 }\nfunc added() bool { return true }\n"), 0o644); err != nil {
		t.Fatalf("write changed file: %v", err)
	}
	return dir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
}

func postJSON[T any](t *testing.T, handler http.Handler, path string, body any) T {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code > 299 {
		t.Fatalf("POST %s returned %d: %s", path, rec.Code, rec.Body.String())
	}
	var value T
	if err := json.NewDecoder(rec.Body).Decode(&value); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return value
}

func getJSON[T any](t *testing.T, handler http.Handler, path string) T {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code > 299 {
		t.Fatalf("GET %s returned %d: %s", path, rec.Code, rec.Body.String())
	}
	var value T
	if err := json.NewDecoder(rec.Body).Decode(&value); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return value
}
