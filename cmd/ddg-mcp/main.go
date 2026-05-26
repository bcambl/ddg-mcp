package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bcambl/ddg-mcp/internal/config"
	"github.com/bcambl/ddg-mcp/internal/fetch"
	"github.com/bcambl/ddg-mcp/internal/search"
	"github.com/bcambl/ddg-mcp/internal/server"
)

var version = "dev"

func main() {
	cfg := config.Load()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: server.ParseLogLevel(cfg.LogLevel),
	})))

	searchClient := search.NewClient(search.ClientOptions{
		HTTPClient:  &http.Client{Timeout: cfg.SearchTimeout},
		UserAgent:   cfg.UserAgent,
		BaseURL:     cfg.SearchURL,
		MaxRetries:  cfg.MaxRetries,
		MaxBodySize: cfg.MaxBodySize,
	})

	fetchClient := fetch.NewClient(fetch.ClientOptions{
		UserAgent:   cfg.FetchUserAgent,
		MaxBodySize: cfg.MaxBodySize,
		CheckSSRF:   cfg.SSRFProtection,
		Timeout:     cfg.FetchTimeout,
	})

	srv := server.New(searchClient, fetchClient, version, cfg.DefaultRegion)

	slog.Info("starting ddg-mcp server", "version", version)

	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		slog.Error("MCP server failed", "error", err)
		os.Exit(1)
	}
}
