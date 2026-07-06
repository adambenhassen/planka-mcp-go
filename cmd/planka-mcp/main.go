// Command planka-mcp runs the Planka MCP server over stdio or SSE.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/adambenhassen/planka-mcp-go/internal/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

// run wires configuration to the server and starts the selected transport,
// returning any fatal error to main so deferred cleanup still runs.
func run(logger *slog.Logger) error {
	cfg := server.LoadConfig()
	srv := server.New(cfg)

	logConfiguration(logger, cfg, srv)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.Transport == "sse" {
		return srv.ServeSSE(ctx, cfg.Port)
	}

	logger.Info("Planka MCP server started (stdio mode)", "tools", len(srv.EnabledTools()))
	return srv.ServeStdio(ctx)
}

// logConfiguration logs the grouped tool configuration, mirroring the
// TypeScript server's startup banner as structured attributes.
func logConfiguration(logger *slog.Logger, cfg server.Config, srv *server.Server) {
	c := srv.Counts()
	logger.Info("tool configuration (grouped)",
		"coreTools", c.Core, "coreOperations", c.CoreOperations,
		"adminTools", c.Admin, "adminOperations", c.AdminOperations, "adminEnabled", cfg.EnableAdmin,
		"optionalTools", c.Optional, "optionalOperations", c.OptionalOperations, "optionalEnabled", cfg.EnableOptional,
		"totalTools", c.Total, "totalOperations", c.TotalOperations,
		"enabledTools", len(srv.EnabledTools()),
	)
}
