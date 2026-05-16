package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/mohammed/letsreview/internal/mcp"
	"github.com/mohammed/letsreview/internal/server"
)

const defaultAddr = "127.0.0.1:55492"

type config struct {
	addr     string
	help     bool
	mcp      bool
	repoPath string
	stop     bool
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

	if cfg.help {
		printUsage(stdout)
		return nil
	}

	if cfg.stop {
		return stopServer(stdout)
	}

	if cfg.mcp {
		srv := mcp.NewMCPServer(cfg.addr)
		srv.Run(ctx, os.Stdin, stdout)
		return nil
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
		listener.Close()
		return fmt.Errorf("start letsreview: %w", err)
	}

	pidPath := pidFilePath()
	if err := writePIDFile(pidPath, os.Getpid(), cfg.addr); err != nil {
		listener.Close()
		return fmt.Errorf("write pid file: %w", err)
	}

	project := projectResponse{ID: projectID(absRepo), RepoPath: absRepo, Repo: filepath.Base(absRepo)}
	printProject(stdout, listener.Addr().String(), project)

	err = app.ServeWithShutdown(ctx, listener)
	os.Remove(pidPath)
	return err
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
	url := fmt.Sprintf("http://%s?project=%s", addr, project.ID)
	fmt.Fprintf(stdout, "letsreview is running at %s\n", url)
	fmt.Fprintf(stdout, "reviewing %s\n", project.RepoPath)
	openBrowser(url)
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

func projectID(absRepo string) string {
	sum := md5.Sum([]byte(absRepo))
	return hex.EncodeToString(sum[:])
}

func parseConfig(args []string) (config, error) {
	flags := flag.NewFlagSet("letsreview", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	addr := flags.String("addr", defaultAddr, "address to listen on")
	help := flags.Bool("help", false, "show help")
	mcpMode := flags.Bool("mcp", false, "run as MCP server over stdio")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return config{help: true}, nil
		}
		return config{}, err
	}

	stop := false
	repoPath := "."
	if flags.NArg() > 0 {
		if flags.Arg(0) == "stop" {
			stop = true
		} else {
			repoPath = flags.Arg(0)
		}
	}

	return config{addr: *addr, help: *help, mcp: *mcpMode, repoPath: repoPath, stop: stop}, nil
}

func printUsage(stdout io.Writer) {
	fmt.Fprint(stdout, `letsreview - local Git diff review UI

Usage:
  letsreview [flags] [repo]
  letsreview stop
  letsreview --mcp [flags]

Commands:
  stop    stop the running letsreview server

Flags:
  -addr string
        address to listen on (default "127.0.0.1:55492")
  -help
        show help
  -mcp
        run as MCP server over stdio

Examples:
  letsreview .
  letsreview ~/Projects/email-client
  letsreview -addr 127.0.0.1:6000 .
  letsreview stop
  letsreview --mcp
`)
}

var pidFilePath = defaultPIDFilePath

func defaultPIDFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "letsreview", "server.pid")
}

func writePIDFile(path string, pid int, addr string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content := fmt.Sprintf("%d\n%s\n", pid, addr)
	return os.WriteFile(path, []byte(content), 0644)
}

type pidEntry struct {
	PID  int
	Addr string
}

func readPIDFile(path string) (pidEntry, error) {
	if path == "" {
		return pidEntry{}, errors.New("no pid file path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return pidEntry{}, err
	}
	lines := splitLines(string(data))
	if len(lines) < 1 {
		return pidEntry{}, errors.New("empty pid file")
	}
	pid, err := strconv.Atoi(lines[0])
	if err != nil {
		return pidEntry{}, fmt.Errorf("parse pid: %w", err)
	}
	addr := defaultAddr
	if len(lines) >= 2 && lines[1] != "" {
		addr = lines[1]
	}
	return pidEntry{PID: pid, Addr: addr}, nil
}

func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func stopServer(stdout io.Writer) error {
	path := pidFilePath()
	if path == "" {
		return errors.New("cannot determine pid file path")
	}
	entry, err := readPIDFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(stdout, "no letsreview server is running")
			return nil
		}
		return fmt.Errorf("read pid file: %w", err)
	}

	if !isProcessAlive(entry.PID) {
		os.Remove(path)
		fmt.Fprintln(stdout, "stale pid file removed (server not running)")
		return nil
	}

	proc, err := os.FindProcess(entry.PID)
	if err != nil {
		return fmt.Errorf("find process %d: %w", entry.PID, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to %d: %w", entry.PID, err)
	}

	if err := waitExit(proc, 5*time.Second); err != nil {
		fmt.Fprintf(stdout, "server (pid %d) did not exit cleanly, killing\n", entry.PID)
		proc.Signal(syscall.SIGKILL)
	}

	os.Remove(path)
	fmt.Fprintf(stdout, "server (pid %d) stopped\n", entry.PID)
	return nil
}

func waitExit(proc *os.Process, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		_, err := proc.Wait()
		done <- err
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return errors.New("timeout")
	}
}

func splitLines(s string) []string {
	var lines []string
	for _, l := range splitBy(s, '\n') {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func splitBy(s string, sep byte) []string {
	var parts []string
	for {
		i := indexOf(s, sep)
		if i < 0 {
			parts = append(parts, s)
			return parts
		}
		parts = append(parts, s[:i])
		s = s[i+1:]
	}
}

func indexOf(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
