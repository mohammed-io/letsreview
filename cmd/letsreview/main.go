package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mohammed/letsreview/internal/server"
)

const defaultAddr = "127.0.0.1:55492"

type config struct {
	addr     string
	openUI   bool
	repoPath string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}

	absRepo, err := filepath.Abs(cfg.repoPath)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}

	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		project, joinErr := registerProject(ctx, cfg.addr, absRepo)
		if joinErr != nil {
			return fmt.Errorf("listen on %s: %w; join existing server: %v", cfg.addr, err, joinErr)
		}
		printProject(stdout, cfg.addr, project)
		return heartbeatLoop(ctx, cfg.addr, project.ID)
	}

	app, err := server.New(absRepo)
	if err != nil {
		return fmt.Errorf("start letsreview: %w", err)
	}

	project := projectResponse{ID: projectID(absRepo), RepoPath: absRepo, Repo: filepath.Base(absRepo)}
	printProject(stdout, listener.Addr().String(), project)
	if cfg.openUI {
		fmt.Fprintln(stdout, "open the URL above in your browser")
	}

	if err := app.Serve(listener); err != nil {
		return fmt.Errorf("serve letsreview: %w", err)
	}
	return nil
}

type projectResponse struct {
	ID       string `json:"id"`
	RepoPath string `json:"repoPath"`
	Repo     string `json:"repo"`
}

func registerProject(ctx context.Context, addr string, repoPath string) (projectResponse, error) {
	payload, err := json.Marshal(map[string]string{"repoPath": repoPath})
	if err != nil {
		return projectResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://%s/api/projects", addr), bytes.NewReader(payload))
	if err != nil {
		return projectResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return projectResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return projectResponse{}, fmt.Errorf("POST /api/projects returned %d: %s", resp.StatusCode, body)
	}
	var project projectResponse
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return projectResponse{}, err
	}
	return project, nil
}

func heartbeatLoop(ctx context.Context, addr string, projectID string) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := heartbeat(ctx, addr, projectID); err != nil {
				return err
			}
		}
	}
}

func heartbeat(ctx context.Context, addr string, projectID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://%s/api/projects/%s/heartbeat", addr, projectID), nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST heartbeat returned %d: %s", resp.StatusCode, body)
	}
	return nil
}

func printProject(stdout io.Writer, addr string, project projectResponse) {
	fmt.Fprintf(stdout, "letsreview is running at http://%s?project=%s\n", addr, project.ID)
	fmt.Fprintf(stdout, "reviewing %s\n", project.RepoPath)
}

func projectID(absRepo string) string {
	sum := md5.Sum([]byte(absRepo))
	return hex.EncodeToString(sum[:])
}

func parseConfig(args []string) (config, error) {
	flags := flag.NewFlagSet("letsreview", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	addr := flags.String("addr", defaultAddr, "address to listen on")
	openUI := flags.Bool("open", false, "print browser URL only; opening browsers is left to the caller")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}

	repoPath := "."
	if flags.NArg() > 0 {
		repoPath = flags.Arg(0)
	}

	return config{addr: *addr, openUI: *openUI, repoPath: repoPath}, nil
}
