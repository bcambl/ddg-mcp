package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/bcambl/ddg-mcp/internal/config"
	"github.com/bcambl/ddg-mcp/internal/fetch"
	"github.com/bcambl/ddg-mcp/internal/inputs"
	"github.com/bcambl/ddg-mcp/internal/search"
)

func tWrite(t *testing.T, w http.ResponseWriter, data []byte) {
	t.Helper()
	_, err := w.Write(data)
	require.NoError(t, err)
}

const mockDDGLiteHTML = `<table border="0">
<tr><td valign="top">1.&nbsp;</td><td><a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fgolang&amp;rut=abc123">Go Programming Language</a></td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td class="result-snippet">Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.</td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td><span class="link-text">example.com/golang</span></td></tr>
<tr><td>&nbsp;</td><td>&nbsp;</td></tr>
<tr><td valign="top">2.&nbsp;</td><td><a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org%2Fdoc%2Ftutorial&amp;rut=def456">Go Tutorial - golang.org</a></td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td class="result-snippet">A tutorial introducing the basics of Go programming. Learn how to write packages and programs.</td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td><span class="link-text">golang.org/doc/tutorial</span></td></tr>
<tr><td>&nbsp;</td><td>&nbsp;</td></tr>
<tr><td valign="top">3.&nbsp;</td><td><a class="result-link" href="https://plain-url.example.com/page">Plain URL Result</a></td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td class="result-snippet">This result has a plain URL without DDG redirect.</td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td><span class="link-text">plain-url.example.com/page</span></td></tr>
<tr><td>&nbsp;</td><td>&nbsp;</td></tr>
</table>`

func newTestSearchClient(srv *httptest.Server) *search.Client {
	return search.NewClient(search.ClientOptions{
		HTTPClient:  srv.Client(),
		UserAgent:   config.DefaultUserAgent,
		BaseURL:     srv.URL,
		MaxRetries:  0,
		RetryDelay:  0,
		MaxBodySize: config.DefaultMaxBodySize,
	})
}

func newTestFetchClient(srv *httptest.Server) *fetch.Client {
	cfg := config.DefaultTestConfig()
	return fetch.NewClient(fetch.ClientOptions{
		HTTPClient:  srv.Client(),
		UserAgent:   cfg.FetchUserAgent,
		MaxBodySize: cfg.MaxBodySize,
		CheckSSRF:   false,
		Timeout:     cfg.FetchTimeout,
	})
}

func setupMCPTest(t *testing.T) (*httptest.Server, *mcp.ClientSession) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	t.Cleanup(srv.Close)

	searchClient := newTestSearchClient(srv)
	fetchClient := newTestFetchClient(srv)
	server := New(searchClient, fetchClient, "test", "")

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
	require.Len(t, tools.Tools, 2)

	toolNames := make(map[string]bool)
	for _, tool := range tools.Tools {
		toolNames[tool.Name] = true
	}
	require.True(t, toolNames["web_search"])
	require.True(t, toolNames["web_fetch"])
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

	searchClient := newTestSearchClient(srv)
	fetchClient := newTestFetchClient(srv)
	s := New(searchClient, fetchClient, "test", "")

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := s.Connect(ctx, serverTransport, nil)
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

	searchClient := newTestSearchClient(srv)
	fetchClient := newTestFetchClient(srv)
	s := New(searchClient, fetchClient, "test", "")

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := s.Connect(ctx, serverTransport, nil)
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

	longQuery := strings.Repeat("a", inputs.MaxQueryLength+1)

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

func TestMCPServerWebFetchToolCall(t *testing.T) {
	fetchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tWrite(t, w, []byte(`<html><head><title>Test Page</title></head><body><p>Hello World</p></body></html>`))
	}))
	t.Cleanup(fetchSrv.Close)

	searchSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	t.Cleanup(searchSrv.Close)

	searchClient := newTestSearchClient(searchSrv)
	fetchClient := newTestFetchClient(fetchSrv)
	s := New(searchClient, fetchClient, "test", "")

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := s.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { serverSession.Close() })

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { clientSession.Close() })

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "web_fetch",
		Arguments: map[string]any{
			"url": fetchSrv.URL,
		},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)

	var parsed map[string]any
	err = json.Unmarshal([]byte(textContent.Text), &parsed)
	require.NoError(t, err)
	require.Equal(t, fetchSrv.URL, parsed["url"])
	require.Equal(t, "Test Page", parsed["title"])
}

func TestSuccessResultFallback(t *testing.T) {
	r := successResult(make(chan int))
	require.Len(t, r.Content, 1)
	textContent, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.NotEmpty(t, textContent.Text)
}

func TestErrorResultHelper(t *testing.T) {
	res := errorResult(inputs.ErrEmptyQuery)
	require.NotNil(t, res)
	require.True(t, res.IsError)
	require.Len(t, res.Content, 1)

	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected *mcp.TextContent, got %T", res.Content[0])
	require.Contains(t, tc.Text, inputs.ErrEmptyQuery.Error())
	require.True(t, len(tc.Text) >= len("Error: "), "should be prefixed with 'Error: '")
}

func TestSuccessResultHelper(t *testing.T) {
	payload := map[string]any{"foo": "bar", "n": 42}
	res := successResult(payload)
	require.NotNil(t, res)
	require.False(t, res.IsError)
	require.Len(t, res.Content, 1)

	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected *mcp.TextContent, got %T", res.Content[0])
	require.Contains(t, tc.Text, `"foo": "bar"`)
	require.Contains(t, tc.Text, `"n": 42`)
}
