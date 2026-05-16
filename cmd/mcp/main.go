package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/mohammed/letsreview/internal/mcp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := "127.0.0.1:55492"
	if env := os.Getenv("LETSREVIEW_ADDR"); env != "" {
		addr = env
	}

	srv := mcp.NewMCPServer(addr)
	srv.Run(ctx, os.Stdin, os.Stdout)
}
