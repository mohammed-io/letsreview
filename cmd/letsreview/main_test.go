package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfigDefaultsToFixedPortAndCurrentRepo(t *testing.T) {
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if cfg.addr != "127.0.0.1:55492" {
		t.Fatalf("expected fixed default address, got %q", cfg.addr)
	}
	if cfg.repoPath != "." {
		t.Fatalf("expected current directory repo by default, got %q", cfg.repoPath)
	}
}

func TestParseConfigAcceptsRepoPathAndAddressOverride(t *testing.T) {
	cfg, err := parseConfig([]string{"-addr", "127.0.0.1:6000", "-open", "/tmp/repo"})
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if cfg.addr != "127.0.0.1:6000" {
		t.Fatalf("expected address override, got %q", cfg.addr)
	}
	if !cfg.openUI {
		t.Fatalf("expected open flag to be enabled")
	}
	if cfg.repoPath != "/tmp/repo" {
		t.Fatalf("expected repo path override, got %q", cfg.repoPath)
	}
}

func TestParseConfigAcceptsMCPMode(t *testing.T) {
	cfg, err := parseConfig([]string{"--mcp", "-addr", "127.0.0.1:6000"})
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if !cfg.mcp {
		t.Fatal("expected MCP mode")
	}
	if cfg.addr != "127.0.0.1:6000" {
		t.Fatalf("expected address override, got %q", cfg.addr)
	}
}

func TestParseConfigRecognizesStopCommand(t *testing.T) {
	cfg, err := parseConfig([]string{"stop"})
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if !cfg.stop {
		t.Fatal("expected stop flag")
	}
}

func TestRunHelpPrintsUsageWithoutError(t *testing.T) {
	var stdout bytes.Buffer
	if err := run(context.Background(), []string{"--help"}, &stdout); err != nil {
		t.Fatalf("run help: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "Usage:") || !strings.Contains(output, "letsreview --mcp") {
		t.Fatalf("expected usage output, got %q", output)
	}
}

func TestRunStopWithNoServerReportsNotRunning(t *testing.T) {
	var stdout bytes.Buffer
	pidPath := filepath.Join(t.TempDir(), "server.pid")
	origPidFilePath := pidFilePath
	pidFilePath = func() string { return pidPath }
	defer func() { pidFilePath = origPidFilePath }()

	if err := stopServer(&stdout); err != nil {
		t.Fatalf("stop with no server: %v", err)
	}
	if !strings.Contains(stdout.String(), "no letsreview server") {
		t.Fatalf("expected not-running message, got %q", stdout.String())
	}
}

func TestProjectIDUsesMD5OfAbsolutePath(t *testing.T) {
	got := projectID("/tmp/example")
	want := "89a35363ec8de7131a16c2ed7419999a"
	if got != want {
		t.Fatalf("expected md5 project id %q, got %q", want, got)
	}
}

func TestRegisterProjectAndHeartbeatUseExistingServerAPI(t *testing.T) {
	var registeredPath string
	var heartbeatPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects":
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode register request: %v", err)
			}
			registeredPath = req["repoPath"]
			_ = json.NewEncoder(w).Encode(projectResponse{ID: "abc123", RepoPath: registeredPath, Repo: "repo"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/abc123/heartbeat":
			heartbeatPath = r.URL.Path
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	project, err := registerProject(context.Background(), addr, "/tmp/repo")
	if err != nil {
		t.Fatalf("register project: %v", err)
	}
	if project.ID != "abc123" || registeredPath != "/tmp/repo" {
		t.Fatalf("expected registered project, got %#v path %q", project, registeredPath)
	}
	if err := heartbeat(context.Background(), addr, project.ID); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if heartbeatPath != "/api/projects/abc123/heartbeat" {
		t.Fatalf("expected heartbeat path, got %q", heartbeatPath)
	}
}

func TestWriteAndReadPIDFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.pid")
	if err := writePIDFile(path, 12345, "127.0.0.1:55492"); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	entry, err := readPIDFile(path)
	if err != nil {
		t.Fatalf("read pid: %v", err)
	}
	if entry.PID != 12345 {
		t.Fatalf("expected pid 12345, got %d", entry.PID)
	}
	if entry.Addr != "127.0.0.1:55492" {
		t.Fatalf("expected addr, got %q", entry.Addr)
	}
}

func TestWritePIDFileCreatesDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "server.pid")
	if err := writePIDFile(path, 999, "127.0.0.1:9000"); err != nil {
		t.Fatalf("write pid with mkdir: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("pid file not created: %v", err)
	}
}

func TestReadPIDFileMissingReturnsError(t *testing.T) {
	_, err := readPIDFile(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Fatal("expected error for missing pid file")
	}
}

func TestReadPIDFileDefaultsAddr(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.pid")
	if err := writePIDFile(path, 42, ""); err != nil {
		t.Fatalf("write: %v", err)
	}
	entry, err := readPIDFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if entry.Addr != defaultAddr {
		t.Fatalf("expected default addr, got %q", entry.Addr)
	}
}

func TestIsProcessAliveForCurrentProcess(t *testing.T) {
	if !isProcessAlive(os.Getpid()) {
		t.Fatal("current process should be alive")
	}
}

func TestIsProcessAliveForFakePID(t *testing.T) {
	if isProcessAlive(99999999) {
		t.Fatal("fake pid should not be alive")
	}
}

func TestStopServerCleansStalePID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.pid")
	origPidFilePath := pidFilePath
	pidFilePath = func() string { return path }
	defer func() { pidFilePath = origPidFilePath }()

	if err := writePIDFile(path, 99999999, defaultAddr); err != nil {
		t.Fatalf("write: %v", err)
	}
	var stdout bytes.Buffer
	if err := stopServer(&stdout); err != nil {
		t.Fatalf("stop stale: %v", err)
	}
	if !strings.Contains(stdout.String(), "stale") {
		t.Fatalf("expected stale message, got %q", stdout.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("stale pid file should be removed")
	}
}
