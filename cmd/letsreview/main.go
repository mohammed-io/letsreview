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
	"strings"
	"syscall"
	"time"

	"github.com/mohammed-io/letsreview/internal/mcp"
	"github.com/mohammed-io/letsreview/internal/server"
)

const defaultAddr = "127.0.0.1:55492"

type config struct {
	addr     string
	help     bool
	mcp      bool
	noOpen   bool
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
		printProject(stdout, cfg.addr, project, cfg.noOpen)
		return heartbeatLoop(ctx, cfg.addr, project.ID)
	}

	app, err := server.New(absRepo)
	if err != nil {
		listener.Close()
		return fmt.Errorf("start letsreview: %w", err)
	}

	pidPath := pidFilePath()
	if err := appendPIDFile(pidPath, os.Getpid(), cfg.addr); err != nil {
		listener.Close()
		return fmt.Errorf("write pid file: %w", err)
	}

	project := projectResponse{ID: projectID(absRepo), RepoPath: absRepo, Repo: filepath.Base(absRepo)}
	printProject(stdout, listener.Addr().String(), project, cfg.noOpen)

	err = app.ServeWithShutdown(ctx, listener)
	removePIDEntry(pidPath, os.Getpid())
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

func printProject(stdout io.Writer, addr string, project projectResponse, noOpen bool) {
	url := fmt.Sprintf("http://%s?project=%s", addr, project.ID)
	fmt.Fprintf(stdout, "letsreview is running at %s\n", url)
	fmt.Fprintf(stdout, "reviewing %s\n", project.RepoPath)
	if !noOpen {
		openBrowser(url)
	}
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
	noOpen := flags.Bool("no-open", false, "don't open browser automatically")
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

	return config{addr: *addr, help: *help, mcp: *mcpMode, noOpen: *noOpen, repoPath: repoPath, stop: stop}, nil
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
  -no-open
        don't open browser automatically

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

func appendPIDFile(path string, pid int, addr string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	entry := fmt.Sprintf("%d %s\n", pid, addr)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}

type pidEntry struct {
	PID  int
	Addr string
}

func readAllPIDEntries(path string) ([]pidEntry, error) {
	if path == "" {
		return nil, errors.New("no pid file path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []pidEntry
	for _, line := range splitLines(string(data)) {
		parts := splitBy(line, ' ')
		if len(parts) < 1 {
			continue
		}
		pid, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		addr := defaultAddr
		if len(parts) >= 2 && parts[1] != "" {
			addr = parts[1]
		}
		entries = append(entries, pidEntry{PID: pid, Addr: addr})
	}
	if len(entries) == 0 {
		return nil, errors.New("empty pid file")
	}
	return entries, nil
}

func removePIDEntry(path string, pid int) {
	if path == "" {
		return
	}
	entries, err := readAllPIDEntries(path)
	if err != nil {
		return
	}
	var remaining []string
	for _, e := range entries {
		if e.PID != pid {
			remaining = append(remaining, fmt.Sprintf("%d %s", e.PID, e.Addr))
		}
	}
	if len(remaining) == 0 {
		os.Remove(path)
		return
	}
	os.WriteFile(path, []byte(strings.Join(remaining, "\n")+"\n"), 0644)
}

func readPIDFile(path string) (pidEntry, error) {
	entries, err := readAllPIDEntries(path)
	if err != nil {
		return pidEntry{}, err
	}
	return entries[0], nil
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
	entries, err := readAllPIDEntries(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(stdout, "no letsreview server is running")
			return nil
		}
		return fmt.Errorf("read pid file: %w", err)
	}

	var alive []pidEntry
	for _, entry := range entries {
		if !isProcessAlive(entry.PID) {
			fmt.Fprintf(stdout, "stale pid %d skipped (not running)\n", entry.PID)
			continue
		}
		alive = append(alive, entry)
	}

	if len(alive) == 0 {
		os.Remove(path)
		fmt.Fprintln(stdout, "stale pid file removed (no live servers)")
		return nil
	}

	for _, entry := range alive {
		proc, err := os.FindProcess(entry.PID)
		if err != nil {
			fmt.Fprintf(stdout, "find process %d: %v\n", entry.PID, err)
			continue
		}
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			fmt.Fprintf(stdout, "SIGTERM to %d: %v\n", entry.PID, err)
			continue
		}
		if err := waitExit(proc, 5*time.Second); err != nil {
			fmt.Fprintf(stdout, "server (pid %d) did not exit cleanly, killing\n", entry.PID)
			proc.Signal(syscall.SIGKILL)
		}
		fmt.Fprintf(stdout, "server (pid %d) stopped\n", entry.PID)
	}

	os.Remove(path)
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
