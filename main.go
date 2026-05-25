package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverName = "ddg-mcp"

var version = "dev"

func newServer(client *SearchClient) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: version,
	}, &mcp.ServerOptions{
		Instructions: "DuckDuckGo Web Search - Search the web using DuckDuckGo HTML scraping. " +
			"Returns search results with titles, URLs, and snippets. " +
			"Results are limited to the first page (~10 results). " +
			"Be mindful of rate limiting; avoid rapid successive queries.",
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "web_search",
		Description: "Search the web using DuckDuckGo. Returns search results with titles, URLs, and snippets. Limited to ~10 results per query.",
	}, makeWebSearchHandler(client))

	return server
}

func makeWebSearchHandler(client *SearchClient) mcp.ToolHandlerFor[webSearchInput, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input webSearchInput) (*mcp.CallToolResult, any, error) {
		input.Query = strings.TrimSpace(input.Query)
		if err := input.validate(); err != nil {
			r, _ := errorResult(err)
			return r, nil, nil
		}

		results, err := client.Search(ctx, input.Query)
		if err != nil {
			r, _ := errorResult(err)
			return r, nil, nil
		}

		r, _ := successResult(map[string]any{
			"query":        input.Query,
			"result_count": len(results),
			"results":      results,
		})
		return r, nil, nil
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	client := newSearchClient()
	server := newServer(client)

	slog.Info("starting ddg-mcp server", "version", version)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		slog.Error("MCP server failed", "error", err)
		os.Exit(1)
	}
}
