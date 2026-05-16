package server

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebUIIncludesReviewCockpit(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / returned %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Review cockpit", "Feedback", "Keyboard shortcuts", "panel-add-comment", "review-comment-list"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected web UI to include %q", want)
		}
	}
}

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
	if _, ok := payload["repoPath"]; ok {
		t.Fatalf("expected compact payload without repo path, got %#v", payload)
	}
	if _, ok := payload["session"]; ok {
		t.Fatalf("expected compact payload without session diff data, got %#v", payload)
	}
	if _, ok := payload["directive"]; ok {
		t.Fatalf("expected compact payload without directive, got %#v", payload)
	}
	comments, ok := payload["comments"].([]any)
	if !ok {
		t.Fatalf("expected comments array in payload, got %#v", payload["comments"])
	}
	if len(comments) != 1 {
		t.Fatalf("expected one exported comment, got %#v", comments)
	}
	comment, ok := comments[0].(map[string]any)
	if !ok {
		t.Fatalf("expected exported comment object, got %#v", comments[0])
	}
	if comment["filePath"] != "main.go" || comment["body"] != "Rename this for clarity." {
		t.Fatalf("expected comment body and file path only, got %#v", comment)
	}
	if comment["startLine"] != float64(1) || comment["endLine"] != float64(2) {
		t.Fatalf("expected comment line range, got %#v", comment)
	}
	if comment["id"] == nil || comment["id"] == "" {
		t.Fatalf("expected exported comment to include id for agent resolve, got %#v", comment)
	}
	if comment["resolved"] != false {
		t.Fatalf("expected exported comment to include resolved=false, got %v", comment["resolved"])
	}
	if _, ok := comment["sessionId"]; ok {
		t.Fatalf("expected exported comment without session id, got %#v", comment)
	}
}

func TestDeleteFeedbackRemovesCommentFromAgentPayload(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	session := postJSON[Session](t, app.Handler(), "/api/sessions", map[string]string{"mode": "working"})
	feedback := postJSON[Feedback](t, app.Handler(), "/api/sessions/"+session.ID+"/feedback", map[string]any{
		"filePath":  "main.go",
		"startLine": 3,
		"endLine":   3,
		"body":      "Remove this comment.",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+session.ID+"/feedback/"+feedback.ID, nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE feedback returned %d: %s", rec.Code, rec.Body.String())
	}

	payload := getJSON[map[string]any](t, app.Handler(), "/api/sessions/"+session.ID+"/agent-payload")
	comments, ok := payload["comments"].([]any)
	if !ok {
		t.Fatalf("expected comments array in payload, got %#v", payload["comments"])
	}
	if len(comments) != 0 {
		t.Fatalf("expected deleted comment to be removed, got %#v", comments)
	}
}

func TestDeleteSessionRemovesFeedbackAndSession(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	session := postJSON[Session](t, app.Handler(), "/api/sessions", map[string]string{"mode": "working"})
	postJSON[Feedback](t, app.Handler(), "/api/sessions/"+session.ID+"/feedback", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   1,
		"body":      "Clear me.",
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+session.ID, nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE session returned %d: %s", rec.Code, rec.Body.String())
	}

	sessions := getJSON[[]Session](t, app.Handler(), "/api/sessions")
	if len(sessions) != 0 {
		t.Fatalf("expected deleted session to be removed, got %#v", sessions)
	}
}

func TestSubmitReviewStoresComments(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	session := postJSON[Session](t, app.Handler(), "/api/sessions", map[string]string{"mode": "working"})
	postJSON[Feedback](t, app.Handler(), "/api/sessions/"+session.ID+"/feedback", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   3,
		"body":      "Fix this.",
	})
	postJSON[Feedback](t, app.Handler(), "/api/sessions/"+session.ID+"/feedback", map[string]any{
		"filePath":  "other.go",
		"startLine": 10,
		"endLine":   10,
		"body":      "Also this.",
	})

	submitReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+session.ID+"/submit-review", nil)
	submitRec := httptest.NewRecorder()
	app.Handler().ServeHTTP(submitRec, submitReq)
	if submitRec.Code != http.StatusOK {
		t.Fatalf("POST submit-review returned %d: %s", submitRec.Code, submitRec.Body.String())
	}

	var submitResult map[string]any
	if err := json.NewDecoder(submitRec.Body).Decode(&submitResult); err != nil {
		t.Fatalf("decode submit result: %v", err)
	}
	if submitResult["status"] != "submitted" {
		t.Fatalf("expected submitted, got %v", submitResult["status"])
	}
	if submitResult["commentCount"] != float64(2) {
		t.Fatalf("expected 2 comments, got %v", submitResult["commentCount"])
	}

	events := app.Store().GetEventsAfter(session.ID, 0)
	if len(events) == 0 {
		t.Fatal("expected at least 1 event after submit")
	}
	found := false
	for _, e := range events {
		if e.Type == "review_submitted" && e.Review != nil {
			found = true
			if len(e.Review.Comments) != 2 {
				t.Fatalf("expected 2 review comments, got %d", len(e.Review.Comments))
			}
			if e.Review.Comments[0].Body != "Fix this." {
				t.Fatalf("expected first comment body, got %q", e.Review.Comments[0].Body)
			}
		}
	}
	if !found {
		t.Fatal("expected review_submitted event with review")
	}
}

func TestProjectRegistrationUsesMD5PathIDAndReusesDuplicates(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	first := postJSON[Project](t, app.Handler(), "/api/projects", map[string]string{"repoPath": repo})
	second := postJSON[Project](t, app.Handler(), "/api/projects", map[string]string{"repoPath": repo})
	expected := md5Hex(repo)

	if first.ID != expected {
		t.Fatalf("expected md5 project id %q, got %q", expected, first.ID)
	}
	if second.ID != first.ID {
		t.Fatalf("expected duplicate repo registration to reuse project id, got %q and %q", first.ID, second.ID)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+first.ID+"/heartbeat", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("heartbeat returned %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectScopedSessionsCanUseMultipleRepos(t *testing.T) {
	repoA := makeRepo(t)
	repoB := makeRepo(t)
	app, err := New(repoA)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	projectB := postJSON[Project](t, app.Handler(), "/api/projects", map[string]string{"repoPath": repoB})
	sessionB := postJSON[Session](t, app.Handler(), "/api/projects/"+projectB.ID+"/sessions", map[string]string{"mode": "working"})
	if sessionB.Meta["repo"] != filepath.Base(repoB) {
		t.Fatalf("expected scoped session to use repo B, got %#v", sessionB.Meta)
	}

	liveB := getJSON[map[string]any](t, app.Handler(), "/api/projects/"+projectB.ID+"/live")
	metaB, ok := liveB["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected project B live metadata, got %#v", liveB["meta"])
	}
	if metaB["projectID"] != projectB.ID || metaB["repo"] != filepath.Base(repoB) {
		t.Fatalf("expected project B live response, got %#v", metaB)
	}

	legacyLive := getJSON[map[string]any](t, app.Handler(), "/api/live")
	legacyMeta, ok := legacyLive["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected legacy live metadata, got %#v", legacyLive["meta"])
	}
	if legacyMeta["repo"] != filepath.Base(repoA) {
		t.Fatalf("expected legacy route to use default repo A, got %#v", legacyMeta)
	}
}

func TestLiveDiffExposesRepoNameForUI(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	body := getJSON[map[string]any](t, app.Handler(), "/api/live")
	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected live diff metadata, got %#v", body["meta"])
	}
	if meta["repo"] != filepath.Base(repo) {
		t.Fatalf("expected repo label %q, got %#v", filepath.Base(repo), meta["repo"])
	}
	if body["summary"] == "" {
		t.Fatalf("expected live summary for UI, got %#v", body["summary"])
	}
}

func TestLiveDiffAcceptsContextLineQuery(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/live?contextLines=20", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET live diff returned %d: %s", rec.Code, rec.Body.String())
	}
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func TestStaticUIKeepsCanvasDiffRenderer(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / returned %d: %s", rec.Code, rec.Body.String())
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	html := string(body)
	if !strings.Contains(html, `<canvas id="diff-canvas"`) {
		t.Fatalf("expected canvas diff renderer in static UI")
	}
	if !strings.Contains(html, `id="inline-review"`) {
		t.Fatalf("expected inline review form in static UI")
	}
	if strings.Contains(html, `class="assistant"`) {
		t.Fatalf("expected feedback to be inline, not in a side assistant panel")
	}
	if !strings.Contains(html, `class="review-panel"`) {
		t.Fatalf("expected review cockpit panel in static UI")
	}
	if !strings.Contains(html, `id="viewed-file"`) {
		t.Fatalf("expected viewed checkbox in static UI")
	}
	if strings.Contains(html, `id="show-file-comments"`) {
		t.Fatalf("expected file comments button to be removed from diff toolbar")
	}
	if !strings.Contains(html, `id="agent-payload-modal"`) {
		t.Fatalf("expected agent payload modal in static UI")
	}
	if !strings.Contains(html, `id="clear-session"`) {
		t.Fatalf("expected clear session button in static UI")
	}
	if strings.Contains(html, `id="context-lines"`) {
		t.Fatalf("expected no context selector in static UI")
	}
	if !strings.Contains(html, `/app.js`) || !strings.Contains(html, `/styles.css`) {
		t.Fatalf("expected app assets to be linked in static UI")
	}
}

func TestWebUIRootServesStaticFilesFromDisk(t *testing.T) {
	repo := makeRepo(t)
	webRoot := t.TempDir()
	t.Setenv("WEB_UI_ROOT", webRoot)
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("dev static root"), 0o644); err != nil {
		t.Fatalf("write dev index: %v", err)
	}

	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / returned %d: %s", rec.Code, rec.Body.String())
	}
	if strings.TrimSpace(rec.Body.String()) != "dev static root" {
		t.Fatalf("expected WEB_UI_ROOT asset, got %q", rec.Body.String())
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

func TestResolveFeedbackMarksCommentAndExcludesFromSubmit(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	session := postJSON[Session](t, app.Handler(), "/api/sessions", map[string]string{"mode": "working"})
	fb1 := postJSON[Feedback](t, app.Handler(), "/api/sessions/"+session.ID+"/feedback", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   3,
		"body":      "Fix this.",
	})
	fb2 := postJSON[Feedback](t, app.Handler(), "/api/sessions/"+session.ID+"/feedback", map[string]any{
		"filePath":  "other.go",
		"startLine": 10,
		"endLine":   10,
		"body":      "Keep this.",
	})

	_ = fb2

	req := httptest.NewRequest(http.MethodPatch, "/api/sessions/"+session.ID+"/feedback/"+fb1.ID+"/resolve", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH resolve returned %d: %s", rec.Code, rec.Body.String())
	}

	sessions := getJSON[[]Session](t, app.Handler(), "/api/sessions")
	if len(sessions) != 1 {
		t.Fatal("expected one session")
	}
	var found Feedback
	for _, fb := range sessions[0].Feedback {
		if fb.ID == fb1.ID {
			found = fb
		}
	}
	if !found.Resolved {
		t.Fatal("expected resolved=true after PATCH")
	}
	if found.ResolvedAt == nil {
		t.Fatal("expected resolvedAt to be set")
	}

	submitReq := httptest.NewRequest(http.MethodPost, "/api/sessions/"+session.ID+"/submit-review", nil)
	submitRec := httptest.NewRecorder()
	app.Handler().ServeHTTP(submitRec, submitReq)
	if submitRec.Code != http.StatusOK {
		t.Fatalf("submit returned %d: %s", submitRec.Code, submitRec.Body.String())
	}

	events := app.Store().GetEventsAfter(session.ID, 0)
	var review *SubmittedReview
	for _, e := range events {
		if e.Type == "review_submitted" && e.Review != nil {
			review = e.Review
		}
	}
	if review == nil {
		t.Fatal("expected review_submitted event")
	}
	if len(review.Comments) != 1 {
		t.Fatalf("expected 1 unresolved comment in review, got %d", len(review.Comments))
	}
	if review.Comments[0].Body != "Keep this." {
		t.Fatalf("expected unresolved comment body, got %q", review.Comments[0].Body)
	}
}

func TestResolveNonexistentFeedbackReturns404(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	session := postJSON[Session](t, app.Handler(), "/api/sessions", map[string]string{"mode": "working"})
	req := httptest.NewRequest(http.MethodPatch, "/api/sessions/"+session.ID+"/feedback/fake123/resolve", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent feedback, got %d", rec.Code)
	}
}

func TestAgentPayloadIncludesResolvedStatus(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	session := postJSON[Session](t, app.Handler(), "/api/sessions", map[string]string{"mode": "working"})
	fb := postJSON[Feedback](t, app.Handler(), "/api/sessions/"+session.ID+"/feedback", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   1,
		"body":      "Will be resolved.",
	})

	resolveReq := httptest.NewRequest(http.MethodPatch, "/api/sessions/"+session.ID+"/feedback/"+fb.ID+"/resolve", nil)
	resolveRec := httptest.NewRecorder()
	app.Handler().ServeHTTP(resolveRec, resolveReq)

	payload := getJSON[map[string]any](t, app.Handler(), "/api/sessions/"+session.ID+"/agent-payload")
	comments := payload["comments"].([]any)
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	c := comments[0].(map[string]any)
	if c["resolved"] != true {
		t.Fatalf("expected resolved=true in agent payload, got %v", c["resolved"])
	}
}

func TestExplainCreatesRequestAndAgentResponds(t *testing.T) {
	repo := makeRepo(t)
	app, err := New(repo)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	session := postJSON[Session](t, app.Handler(), "/api/sessions", map[string]string{"mode": "working"})

	explainResult := postJSON[map[string]any](t, app.Handler(), "/api/sessions/"+session.ID+"/explain", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   3,
	})
	if explainResult["summary"] == "" {
		t.Fatal("expected local explanation summary")
	}
	reqData, ok := explainResult["request"].(map[string]any)
	if !ok || reqData["id"] == "" {
		t.Fatalf("expected explanation request with id, got %v", explainResult["request"])
	}

	requests := getJSON[[]map[string]any](t, app.Handler(), "/api/sessions/"+session.ID+"/explanation-requests")
	if len(requests) != 1 {
		t.Fatalf("expected 1 explanation request, got %d", len(requests))
	}
	if requests[0]["resolved"] != false {
		t.Fatal("expected request to be unresolved")
	}

	explanations := getJSON[[]map[string]any](t, app.Handler(), "/api/sessions/"+session.ID+"/explanations")
	if len(explanations) != 0 {
		t.Fatalf("expected 0 explanations before agent responds, got %d", len(explanations))
	}

	agentExplanation := postJSON[map[string]any](t, app.Handler(), "/api/sessions/"+session.ID+"/explanations", map[string]any{
		"filePath":  "main.go",
		"startLine": 1,
		"endLine":   3,
		"body":      "This function returns an integer value.",
	})
	if agentExplanation["body"] != "This function returns an integer value." {
		t.Fatalf("expected explanation body, got %v", agentExplanation["body"])
	}

	explanations = getJSON[[]map[string]any](t, app.Handler(), "/api/sessions/"+session.ID+"/explanations")
	if len(explanations) != 1 {
		t.Fatalf("expected 1 explanation after agent responds, got %d", len(explanations))
	}
	if explanations[0]["body"] != "This function returns an integer value." {
		t.Fatalf("expected agent explanation body, got %v", explanations[0]["body"])
	}

	requests = getJSON[[]map[string]any](t, app.Handler(), "/api/sessions/"+session.ID+"/explanation-requests")
	if requests[0]["resolved"] != true {
		t.Fatal("expected request to be resolved after explanation submitted")
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
