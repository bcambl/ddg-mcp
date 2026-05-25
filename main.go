package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverName = "ddg-mcp"

var version = "dev"

func newServer(searchClient *SearchClient, fetchClient *FetchClient) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: version,
	}, &mcp.ServerOptions{
		Instructions: "DuckDuckGo Web Search - Search the web using DuckDuckGo Lite endpoint. " +
			"Returns search results with titles, URLs, snippets, and domain info. " +
			"Supports pagination via offset/vqd tokens, region bias (kl), safe search (kp), and time range (df). " +
			"Also provides web_fetch to retrieve and extract readable content from URLs. " +
			"Be mindful of rate limiting; avoid rapid successive queries.",
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "web_search",
		Description: "Search the web using DuckDuckGo. Returns search results with titles, URLs, and snippets. Supports pagination, region, safe search, and time range filters.",
	}, makeWebSearchHandler(searchClient))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "web_fetch",
		Description: "Fetch and extract readable content from a URL. Returns the page title and text content with scripts, styles, and navigation elements removed.",
	}, makeWebFetchHandler(fetchClient))

	return server
}

func makeWebSearchHandler(client *SearchClient) mcp.ToolHandlerFor[webSearchInput, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input webSearchInput) (*mcp.CallToolResult, any, error) {
		if err := input.validate(); err != nil {
			return errorResult(err), nil, nil
		}

		opts := SearchOptions{
			Region:     input.Region,
			SafeSearch: input.SafeSearch,
			TimeRange:  input.TimeRange,
		}

		var resp *SearchResponse
		var err error

		if input.Offset > 0 {
			resp, err = client.SearchWithOffset(ctx, input.Query, input.Offset, input.Vqd, opts)
		} else {
			resp, err = client.Search(ctx, input.Query, opts)
		}

		if err != nil {
			return errorResult(err), nil, nil
		}

		r := successResult(resp)
		return r, nil, nil
	}
}

func makeWebFetchHandler(client *FetchClient) mcp.ToolHandlerFor[webFetchInput, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input webFetchInput) (*mcp.CallToolResult, any, error) {
		if err := input.validate(); err != nil {
			return errorResult(err), nil, nil
		}

		result, err := client.Fetch(ctx, input.URL, input.MaxLength)
		if err != nil {
			return errorResult(err), nil, nil
		}

		r := successResult(result)
		return r, nil, nil
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	searchClient := newSearchClient()
	fetchClient := newFetchClient()
	server := newServer(searchClient, fetchClient)

	slog.Info("starting ddg-mcp server", "version", version)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		slog.Error("MCP server failed", "error", err)
		os.Exit(1)
	}
}
