package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
