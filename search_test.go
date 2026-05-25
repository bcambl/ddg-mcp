package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const mockDDGHTML = `<!DOCTYPE html>
<html>
<body>
<div class="result results_links results_links_deep web-result">
  <div class="links_main links_deep result__body">
    <h2 class="result__title">
      <a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fgolang&rut=abc123">Go Programming Language</a>
    </h2>
    <a class="result__snippet" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fgolang&rut=abc123">Go is an open source programming language that makes it easy to build <b>simple</b>, reliable, and efficient software.</a>
    <div class="result__extras__url">
      <a class="result__url" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fgolang">https://example.com/golang</a>
    </div>
  </div>
</div>
<div class="result results_links results_links_deep web-result">
  <div class="links_main links_deep result__body">
    <h2 class="result__title">
      <a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org%2Fdoc%2Ftutorial&rut=def456">Go Tutorial - golang.org</a>
    </h2>
    <a class="result__snippet" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org%2Fdoc%2Ftutorial&rut=def456">A tutorial introducing the basics of Go programming. Learn how to write <b>packages</b> and programs.</a>
  </div>
</div>
<div class="result results_links results_links_deep web-result">
  <div class="links_main links_deep result__body">
    <h2 class="result__title">
      <a rel="nofollow" class="result__a" href="https://plain-url.example.com/page">Plain URL Result</a>
    </h2>
    <a class="result__snippet" href="https://plain-url.example.com/page">This result has a plain URL without DDG redirect.</a>
  </div>
</div>
</body>
</html>`

const mockDDGEmptyHTML = `<!DOCTYPE html>
<html>
<body>
<div class="no-results">No results found for your search.</div>
</body>
</html>`

func newMockDDGServer(t *testing.T, responseHTML string, statusCode int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write([]byte(responseHTML))
		require.NoError(t, err)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(srv *httptest.Server) *SearchClient {
	client := newSearchClient()
	client.baseURL = srv.URL
	client.httpClient = srv.Client()
	client.maxRetries = 0
	client.retryDelay = 0
	return client
}

func TestSearchBasic(t *testing.T) {
	srv := newMockDDGServer(t, mockDDGHTML, http.StatusOK)
	client := newTestClient(srv)

	results, err := client.Search(context.Background(), "golang")
	require.NoError(t, err)
	require.Len(t, results, 3)

	require.Equal(t, "Go Programming Language", results[0].Title)
	require.Equal(t, "https://example.com/golang", results[0].URL)
	require.Contains(t, results[0].Snippet, "Go is an open source programming language")

	require.Equal(t, "Go Tutorial - golang.org", results[1].Title)
	require.Equal(t, "https://golang.org/doc/tutorial", results[1].URL)
	require.Contains(t, results[1].Snippet, "basics of Go programming")

	require.Equal(t, "Plain URL Result", results[2].Title)
	require.Equal(t, "https://plain-url.example.com/page", results[2].URL)
}

func TestSearchURIDecoding(t *testing.T) {
	srv := newMockDDGServer(t, mockDDGHTML, http.StatusOK)
	client := newTestClient(srv)

	results, err := client.Search(context.Background(), "test")
	require.NoError(t, err)
	require.Len(t, results, 3)

	require.Equal(t, "https://example.com/golang", results[0].URL)
	require.Equal(t, "https://golang.org/doc/tutorial", results[1].URL)
	require.Equal(t, "https://plain-url.example.com/page", results[2].URL)
}

func TestSearchSnippetStripsBoldTags(t *testing.T) {
	srv := newMockDDGServer(t, mockDDGHTML, http.StatusOK)
	client := newTestClient(srv)

	results, err := client.Search(context.Background(), "test")
	require.NoError(t, err)
	require.Len(t, results, 3)

	require.NotContains(t, results[0].Snippet, "<b>")
	require.NotContains(t, results[0].Snippet, "</b>")
	require.NotContains(t, results[1].Snippet, "<b>")
	require.NotContains(t, results[1].Snippet, "</b>")
}

func TestSearchEmptyResults(t *testing.T) {
	srv := newMockDDGServer(t, mockDDGEmptyHTML, http.StatusOK)
	client := newTestClient(srv)

	results, err := client.Search(context.Background(), "obscurequery12345")
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestSearch429RateLimit(t *testing.T) {
	srv := newMockDDGServer(t, "rate limited", http.StatusTooManyRequests)
	client := newTestClient(srv)

	_, err := client.Search(context.Background(), "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "rate limited")
	require.Contains(t, err.Error(), "429")
}

func TestSearch500Error(t *testing.T) {
	srv := newMockDDGServer(t, "internal server error", http.StatusInternalServerError)
	client := newTestClient(srv)

	_, err := client.Search(context.Background(), "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 500")
}

func TestSearchRequestHasCorrectHeaders(t *testing.T) {
	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockDDGHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.Search(context.Background(), "test")
	require.NoError(t, err)

	require.Equal(t, ddgDefaultUA, capturedHeaders.Get("User-Agent"))
	require.Equal(t, "https://html.duckduckgo.com/", capturedHeaders.Get("Referer"))
	require.NotEmpty(t, capturedHeaders.Get("Accept"))
	require.NotEmpty(t, capturedHeaders.Get("Accept-Language"))
}

func TestSearchQueryURLEncoded(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockDDGHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.Search(context.Background(), "go programming & testing")
	require.NoError(t, err)

	require.Equal(t, "go programming & testing", capturedQuery)
	parsed, err := url.Parse(srv.URL + "?q=" + url.QueryEscape("go programming & testing"))
	require.NoError(t, err)
	require.Equal(t, capturedQuery, parsed.Query().Get("q"))
}

func TestSearchContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := newMockDDGServer(t, mockDDGHTML, http.StatusOK)
	client := newTestClient(srv)

	_, err := client.Search(ctx, "test")
	require.Error(t, err)
}

func TestExtractRealURL(t *testing.T) {
	tests := []struct {
		name     string
		href     string
		expected string
	}{
		{
			name:     "uddg param encoded",
			href:     "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&rut=abc",
			expected: "https://example.com",
		},
		{
			name:     "uddg param simple",
			href:     "//duckduckgo.com/l/?uddg=http%3A%2F%2Ftest.org%2Fpage&rut=xyz",
			expected: "http://test.org/page",
		},
		{
			name:     "plain https URL",
			href:     "https://example.com/direct",
			expected: "https://example.com/direct",
		},
		{
			name:     "protocol-relative URL",
			href:     "//cdn.example.com/resource",
			expected: "https://cdn.example.com/resource",
		},
		{
			name:     "empty href",
			href:     "",
			expected: "",
		},
		{
			name:     "uddg with special chars",
			href:     "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fsearch%3Fq%3Dtest%26page%3D1",
			expected: "https://example.com/search?q=test&page=1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractRealURL(tc.href)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestParseResultsWithMalformedHTML(t *testing.T) {
	malformedHTML := `<html><body>random broken <div>stuff</div></body></html>`
	results, err := parseResults(strings.NewReader(malformedHTML))
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestParseResultsWithIncompleteResultIgnored(t *testing.T) {
	html := `<html><body>
<div class="result results_links results_links_deep web-result">
  <a class="result__a" href="bad://[invalid">Title Only - No Snippet</a>
</div>
</body></html>`
	results, err := parseResults(strings.NewReader(html))
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "Title Only - No Snippet", results[0].Title)
}

func TestSearchRetryOn429ThenSuccess(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockDDGHTML))
	}))
	defer srv.Close()

	client := newSearchClient()
	client.baseURL = srv.URL
	client.httpClient = srv.Client()
	client.maxRetries = 1
	client.retryDelay = 10 * time.Millisecond

	results, err := client.Search(context.Background(), "test")
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.Equal(t, 2, callCount)
}

func TestSearchRetryExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := newSearchClient()
	client.baseURL = srv.URL
	client.httpClient = srv.Client()
	client.maxRetries = 1
	client.retryDelay = 10 * time.Millisecond

	_, err := client.Search(context.Background(), "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "rate limited")
}

func TestSearchConcurrentSafety(t *testing.T) {
	srv := newMockDDGServer(t, mockDDGHTML, http.StatusOK)
	client := newTestClient(srv)

	const goroutines = 10
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			_, err := client.Search(context.Background(), "test")
			errCh <- err
		}()
	}

	for i := 0; i < goroutines; i++ {
		require.NoError(t, <-errCh)
	}
}

func TestSearchNewRequestError(t *testing.T) {
	client := newSearchClient()
	client.baseURL = "http://\x00invalid"

	_, err := client.Search(context.Background(), "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create search request")
}

func TestSuccessResultFallback(t *testing.T) {
	r, err := successResult(make(chan int))
	require.NoError(t, err)
	require.Len(t, r.Content, 1)
	textContent, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.NotEmpty(t, textContent.Text)
}

func BenchmarkParseResults(b *testing.B) {
	reader := strings.NewReader(mockDDGHTML)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parseResults(reader)
		reader.Reset(mockDDGHTML)
	}
}

func BenchmarkSearch(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockDDGHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Search(context.Background(), "test query")
	}
}
