package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bcambl/ddg-mcp/internal/fetch"
	"github.com/bcambl/ddg-mcp/internal/inputs"
	"github.com/bcambl/ddg-mcp/internal/search"
)

const serverName = "ddg-mcp"

func New(searchClient *search.Client, fetchClient *fetch.Client, version string, defaultRegion string) *mcp.Server {
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
	}, makeWebSearchHandler(searchClient, defaultRegion))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "web_fetch",
		Description: "Fetch and extract readable content from a URL. Returns the page title and text content with scripts, styles, and navigation elements removed.",
	}, makeWebFetchHandler(fetchClient))

	return server
}

func makeWebSearchHandler(client *search.Client, defaultRegion string) mcp.ToolHandlerFor[inputs.Search, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input inputs.Search) (*mcp.CallToolResult, any, error) {
		if err := input.Validate(); err != nil {
			return errorResult(err), nil, nil
		}

		region := input.Region
		if region == "" {
			region = defaultRegion
		}

		opts := search.SearchOptions{
			Region:     region,
			SafeSearch: input.SafeSearch,
			TimeRange:  input.TimeRange,
		}

		var resp *search.SearchResponse
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

func makeWebFetchHandler(client *fetch.Client) mcp.ToolHandlerFor[inputs.Fetch, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, input inputs.Fetch) (*mcp.CallToolResult, any, error) {
		if err := input.Validate(); err != nil {
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

func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Error: %s", err.Error())},
		},
		IsError: true,
	}
}

func successResult(result any) *mcp.CallToolResult {
	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("%v", result)},
			},
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultBytes)},
		},
	}
}

func ParseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
