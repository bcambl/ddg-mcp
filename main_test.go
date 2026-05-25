package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func newTestSearchClient(srv *httptest.Server) *SearchClient {
	client := newSearchClient()
	client.baseURL = srv.URL
	client.httpClient = srv.Client()
	client.maxRetries = 0
	client.retryDelay = 0
	return client
}

func setupMCPTest(t *testing.T) (*httptest.Server, *mcp.ClientSession) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGHTML))
	}))
	t.Cleanup(srv.Close)

	server := newServer(newTestSearchClient(srv))

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { serverSession.Close() })

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { clientSession.Close() })

	return srv, clientSession
}

func TestMCPServerToolRegistration(t *testing.T) {
	_, clientSession := setupMCPTest(t)

	ctx := context.Background()
	tools, err := clientSession.ListTools(ctx, nil)
	require.NoError(t, err)
	require.Len(t, tools.Tools, 1)

	tool := tools.Tools[0]
	require.Equal(t, "web_search", tool.Name)
	require.NotEmpty(t, tool.Description)
}

func TestMCPServerWebSearchToolCall(t *testing.T) {
	_, clientSession := setupMCPTest(t)

	ctx := context.Background()
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "web_search",
		Arguments: map[string]any{
			"query": "golang programming",
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)

	var parsed map[string]any
	err = json.Unmarshal([]byte(textContent.Text), &parsed)
	require.NoError(t, err)

	require.Equal(t, "golang programming", parsed["query"])

	resultCount, ok := parsed["result_count"].(float64)
	require.True(t, ok)
	require.Equal(t, float64(3), resultCount)

	results, ok := parsed["results"].([]any)
	require.True(t, ok)
	require.Len(t, results, 3)

	firstResult, ok := results[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Go Programming Language", firstResult["title"])
	require.Equal(t, "https://example.com/golang", firstResult["url"])
}

func TestMCPServerWebSearchEmptyQuery(t *testing.T) {
	_, clientSession := setupMCPTest(t)

	ctx := context.Background()
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "web_search",
		Arguments: map[string]any{
			"query": "",
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)

	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, textContent.Text, "query must not be empty")
}

func TestMCPServerWebSearchServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		tWrite(t, w, []byte("server error"))
	}))
	defer srv.Close()

	server := newServer(newTestSearchClient(srv))

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "web_search",
		Arguments: map[string]any{
			"query": "test",
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)

	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, textContent.Text, "HTTP 500")
}

func TestMCPServerWebSearchRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		tWrite(t, w, []byte("rate limited"))
	}))
	defer srv.Close()

	server := newServer(newTestSearchClient(srv))

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	defer serverSession.Close()

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "web_search",
		Arguments: map[string]any{
			"query": "test",
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)

	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, textContent.Text, "rate limited")
}

func TestMCPServerWebSearchQueryTooLong(t *testing.T) {
	_, clientSession := setupMCPTest(t)

	longQuery := strings.Repeat("a", maxQueryLength+1)

	ctx := context.Background()
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "web_search",
		Arguments: map[string]any{
			"query": longQuery,
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)

	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, textContent.Text, "maximum length")
}
