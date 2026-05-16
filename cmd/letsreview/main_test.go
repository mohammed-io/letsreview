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

func init() {
	openBrowser = func(string) {}
}

func TestRootCommandNoArgsShowsHelp(t *testing.T) {
	rootCmd := newRootCmd(context.Background())
	rootCmd.SetArgs([]string{})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no args provided")
	}
}

func TestRootCommandWithExplicitPath(t *testing.T) {
	rootCmd := newRootCmd(context.Background())
	rootCmd.SetArgs([]string{"/nonexistent/path/that/does/not/exist"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestRootCommandAcceptsAddrFlag(t *testing.T) {
	rootCmd := newRootCmd(context.Background())
	rootCmd.SetArgs([]string{"--addr", "127.0.0.1:6000", "/nonexistent/path"})
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestMCPCommandExists(t *testing.T) {
	rootCmd := newRootCmd(context.Background())
	cmds := rootCmd.Commands()
	found := false
	for _, cmd := range cmds {
		if cmd.Use == "mcp" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected mcp subcommand")
	}
}

func TestStopCommandExists(t *testing.T) {
	rootCmd := newRootCmd(context.Background())
	cmds := rootCmd.Commands()
	found := false
	for _, cmd := range cmds {
		if cmd.Use == "stop" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected stop subcommand")
	}
}

func TestStopWithNoServerReportsNotRunning(t *testing.T) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
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
	if err := appendPIDFile(path, 12345, "127.0.0.1:55492"); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	entries, err := readAllPIDEntries(path)
	if err != nil {
		t.Fatalf("read pid: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].PID != 12345 {
		t.Fatalf("expected pid 12345, got %d", entries[0].PID)
	}
	if entries[0].Addr != "127.0.0.1:55492" {
		t.Fatalf("expected addr, got %q", entries[0].Addr)
	}
}

func TestAppendPIDFileMultipleEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.pid")
	if err := appendPIDFile(path, 111, "127.0.0.1:55492"); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := appendPIDFile(path, 222, "127.0.0.1:55493"); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	entries, err := readAllPIDEntries(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].PID != 111 || entries[1].PID != 222 {
		t.Fatalf("expected pids 111,222 got %d,%d", entries[0].PID, entries[1].PID)
	}
}

func TestRemovePIDEntryDeletesFileWhenEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.pid")
	appendPIDFile(path, 111, "127.0.0.1:55492")
	removePIDEntry(path, 111)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expected pid file deleted after removing last entry")
	}
}

func TestRemovePIDEntryKeepsOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.pid")
	appendPIDFile(path, 111, "127.0.0.1:55492")
	appendPIDFile(path, 222, "127.0.0.1:55493")
	removePIDEntry(path, 111)
	entries, err := readAllPIDEntries(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 1 || entries[0].PID != 222 {
		t.Fatalf("expected 1 entry with pid 222, got %v", entries)
	}
}

func TestWritePIDFileCreatesDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "server.pid")
	if err := appendPIDFile(path, 999, "127.0.0.1:9000"); err != nil {
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
	if err := appendPIDFile(path, 42, ""); err != nil {
		t.Fatalf("write: %v", err)
	}
	entries, err := readAllPIDEntries(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if entries[0].Addr != defaultAddr {
		t.Fatalf("expected default addr, got %q", entries[0].Addr)
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

	if err := appendPIDFile(path, 99999999, defaultAddr); err != nil {
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
