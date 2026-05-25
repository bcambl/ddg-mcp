package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
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

const mockDDGLiteHTMLWithPagination = `<table border="0">
<tr><td valign="top">1.&nbsp;</td><td><a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&amp;rut=abc">Result 1</a></td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td class="result-snippet">Snippet 1</td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td><span class="link-text">example.com</span></td></tr>
<tr><td>&nbsp;</td><td>&nbsp;</td></tr>
</table>
<form class="next_form" action="/lite/" method="post">
<input name="q" value="test">
<input name="s" value="20">
<input name="dc" value="11">
<input name="vqd" value="4-3185-abc123">
<input name="kl" value="wt-wt">
<input name="v" value="l">
<input name="o" value="json">
<input name="api" value="d.js">
</form>`

const mockDDGLiteHTMLWithSponsored = `<table border="0">
<tr><td valign="top">1.&nbsp;</td><td><a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fsponsor.example.com&amp;rut=sp1">Sponsored Result (Sponsored link - more info)</a></td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td class="result-snippet">This is a sponsored result.</td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td><span class="link-text">sponsor.example.com</span></td></tr>
<tr><td>&nbsp;</td><td>&nbsp;</td></tr>
<tr><td valign="top">2.&nbsp;</td><td><a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Forganic.example.com&amp;rut=org1">Organic Result</a></td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td class="result-snippet">This is an organic result.</td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td><span class="link-text">organic.example.com</span></td></tr>
<tr><td>&nbsp;</td><td>&nbsp;</td></tr>
</table>`

const mockDDGLiteHTMLWithZeroClick = `<table border="0">
<tr><td>Zero-click info: <a href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fen.wikipedia.org%2Fwiki%2FGo_(programming_language)&amp;rut=zc1">Go (programming language)</a></td></tr>
<tr><td>A statically typed, compiled programming language designed at Google. <a href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fen.wikipedia.org%2Fwiki%2FGo_(programming_language)&amp;rut=zc2">More at Wikipedia</a></td></tr>
<tr><td valign="top">1.&nbsp;</td><td><a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&amp;rut=abc">Example Result</a></td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td class="result-snippet">Example snippet.</td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td><span class="link-text">example.com</span></td></tr>
<tr><td>&nbsp;</td><td>&nbsp;</td></tr>
</table>`

const mockDDGEmptyHTML = `<!DOCTYPE html>
<html>
<body>
<div class="no-results">No results found for your search.</div>
</body>
</html>`

const mockDDGLiteHTMLBroken = `<table border="0">
<tr><td valign="top">1.&nbsp;</td><td><a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&amp;rut=abc">Broken Result</a></td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td class="result-snippet">This should be parsed but something went wrong.</td></tr>
</table>`

func newMockDDGServer(t *testing.T, responseHTML string, statusCode int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(statusCode)
		_, err := w.Write([]byte(responseHTML))
		require.NoError(t, err)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestClient(srv *httptest.Server) *SearchClient {
	cfg := defaultTestConfig()
	client := newSearchClient(cfg)
	client.baseURL = srv.URL
	client.httpClient = srv.Client()
	client.maxRetries = 0
	client.retryDelay = 0
	return client
}

func TestSearchBasic(t *testing.T) {
	srv := newMockDDGServer(t, mockDDGLiteHTML, http.StatusOK)
	client := newTestClient(srv)

	resp, err := client.Search(context.Background(), "golang", SearchOptions{})
	require.NoError(t, err)
	require.Len(t, resp.Results, 3)

	require.Equal(t, "Go Programming Language", resp.Results[0].Title)
	require.Equal(t, "https://example.com/golang", resp.Results[0].URL)
	require.Contains(t, resp.Results[0].Snippet, "Go is an open source programming language")
	require.Equal(t, "example.com/golang", resp.Results[0].Domain)

	require.Equal(t, "Go Tutorial - golang.org", resp.Results[1].Title)
	require.Equal(t, "https://golang.org/doc/tutorial", resp.Results[1].URL)
	require.Contains(t, resp.Results[1].Snippet, "basics of Go programming")

	require.Equal(t, "Plain URL Result", resp.Results[2].Title)
	require.Equal(t, "https://plain-url.example.com/page", resp.Results[2].URL)
}

func TestSearchURIDecoding(t *testing.T) {
	srv := newMockDDGServer(t, mockDDGLiteHTML, http.StatusOK)
	client := newTestClient(srv)

	resp, err := client.Search(context.Background(), "test", SearchOptions{})
	require.NoError(t, err)
	require.Len(t, resp.Results, 3)

	require.Equal(t, "https://example.com/golang", resp.Results[0].URL)
	require.Equal(t, "https://golang.org/doc/tutorial", resp.Results[1].URL)
	require.Equal(t, "https://plain-url.example.com/page", resp.Results[2].URL)
}

func TestSearchSnippetStripsBoldTags(t *testing.T) {
	liteHTMLWithBold := `<table border="0">
<tr><td valign="top">1.&nbsp;</td><td><a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com">Result with Bold</a></td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td class="result-snippet">Go is an open source programming language that makes it easy to build <b>simple</b>, reliable, and efficient software.</td></tr>
<tr><td>&nbsp;&nbsp;&nbsp;</td><td><span class="link-text">example.com</span></td></tr>
<tr><td>&nbsp;</td><td>&nbsp;</td></tr>
</table>`
	srv := newMockDDGServer(t, liteHTMLWithBold, http.StatusOK)
	client := newTestClient(srv)

	resp, err := client.Search(context.Background(), "test", SearchOptions{})
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	require.NotContains(t, resp.Results[0].Snippet, "<b>")
	require.NotContains(t, resp.Results[0].Snippet, "</b>")
}

func TestSearchEmptyResults(t *testing.T) {
	srv := newMockDDGServer(t, mockDDGEmptyHTML, http.StatusOK)
	client := newTestClient(srv)

	resp, err := client.Search(context.Background(), "obscurequery12345", SearchOptions{})
	require.NoError(t, err)
	require.Empty(t, resp.Results)
}

func TestSearch429RateLimit(t *testing.T) {
	srv := newMockDDGServer(t, "rate limited", http.StatusTooManyRequests)
	client := newTestClient(srv)

	_, err := client.Search(context.Background(), "test", SearchOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rate limited")
	require.Contains(t, err.Error(), "429")
}

func TestSearch500Error(t *testing.T) {
	srv := newMockDDGServer(t, "internal server error", http.StatusInternalServerError)
	client := newTestClient(srv)

	_, err := client.Search(context.Background(), "test", SearchOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 500")
}

func TestSearchRequestHasCorrectHeaders(t *testing.T) {
	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.Search(context.Background(), "test", SearchOptions{})
	require.NoError(t, err)

	require.Equal(t, ddgDefaultUA, capturedHeaders.Get("User-Agent"))
	require.Equal(t, "https://lite.duckduckgo.com/", capturedHeaders.Get("Referer"))
	require.NotEmpty(t, capturedHeaders.Get("Accept"))
	require.NotEmpty(t, capturedHeaders.Get("Accept-Language"))
}

func TestSearchQueryURLEncoded(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.Search(context.Background(), "go programming & testing", SearchOptions{})
	require.NoError(t, err)

	require.Equal(t, "go programming & testing", capturedQuery)
}

func TestSearchWithRegion(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.Search(context.Background(), "test", SearchOptions{Region: "us-en"})
	require.NoError(t, err)
	require.Contains(t, capturedURL, "kl=us-en")
}

func TestSearchWithSafeSearch(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.Search(context.Background(), "test", SearchOptions{SafeSearch: 1})
	require.NoError(t, err)
	require.Contains(t, capturedURL, "kp=1")
}

func TestSearchWithSafeSearchDefault(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.Search(context.Background(), "test", SearchOptions{SafeSearch: 0})
	require.NoError(t, err)
	require.NotContains(t, capturedURL, "kp=")
}

func TestSearchWithTimeRange(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.Search(context.Background(), "test", SearchOptions{TimeRange: "w"})
	require.NoError(t, err)
	require.Contains(t, capturedURL, "df=w")
}

func TestSearchPagination(t *testing.T) {
	srv := newMockDDGServer(t, mockDDGLiteHTMLWithPagination, http.StatusOK)
	client := newTestClient(srv)

	resp, err := client.Search(context.Background(), "test", SearchOptions{})
	require.NoError(t, err)
	require.True(t, resp.HasNextPage)
	require.Equal(t, 20, resp.NextOffset)
	require.Equal(t, "4-3185-abc123", resp.Vqd)
}

func TestSearchWithOffsetUsesPOST(t *testing.T) {
	var capturedMethod string
	var capturedContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.SearchWithOffset(context.Background(), "test", 10, "vqd-token-123", SearchOptions{})
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, capturedMethod)
	require.Equal(t, "application/x-www-form-urlencoded", capturedContentType)
}

func TestSearchWithOffsetFormFields(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes := make([]byte, 4096)
		n, _ := r.Body.Read(bodyBytes)
		capturedBody = string(bodyBytes[:n])
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	_, err := client.SearchWithOffset(context.Background(), "test query", 10, "vqd-token", SearchOptions{Region: "us-en", SafeSearch: -1, TimeRange: "d"})
	require.NoError(t, err)

	parsed, err := url.ParseQuery(capturedBody)
	require.NoError(t, err)
	require.Equal(t, "test query", parsed.Get("q"))
	require.Equal(t, "10", parsed.Get("s"))
	require.Equal(t, "11", parsed.Get("dc"))
	require.Equal(t, "vqd-token", parsed.Get("vqd"))
	require.Equal(t, "us-en", parsed.Get("kl"))
	require.Equal(t, "-1", parsed.Get("kp"))
	require.Equal(t, "d", parsed.Get("df"))
	require.Equal(t, "l", parsed.Get("v"))
}

func TestSearchContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := newMockDDGServer(t, mockDDGLiteHTML, http.StatusOK)
	client := newTestClient(srv)

	_, err := client.Search(ctx, "test", SearchOptions{})
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

func TestParseLiteResultsBasic(t *testing.T) {
	results, zeroClick, _ := parseLiteResults(mockDDGLiteHTML)
	require.Len(t, results, 3)
	require.Equal(t, "Go Programming Language", results[0].Title)
	require.Equal(t, "https://example.com/golang", results[0].URL)
	require.Equal(t, "example.com/golang", results[0].Domain)
	require.Nil(t, zeroClick)
}

func TestParseLiteResultsWithSponsored(t *testing.T) {
	results, _, _ := parseLiteResults(mockDDGLiteHTMLWithSponsored)
	require.Len(t, results, 2)
	require.True(t, results[0].Sponsored)
	require.Equal(t, "Sponsored Result", results[0].Title)
	require.False(t, results[1].Sponsored)
	require.Equal(t, "Organic Result", results[1].Title)
}

func TestParseLiteResultsWithZeroClick(t *testing.T) {
	results, zeroClick, _ := parseLiteResults(mockDDGLiteHTMLWithZeroClick)
	require.Len(t, results, 1)
	require.NotNil(t, zeroClick)
	require.Equal(t, "Go (programming language)", zeroClick.Title)
}

func TestParseLitePaginationInfo(t *testing.T) {
	hasNextPage, nextOffset, vqd := parsePaginationInfo(mockDDGLiteHTMLWithPagination)
	require.True(t, hasNextPage)
	require.Equal(t, 20, nextOffset)
	require.Equal(t, "4-3185-abc123", vqd)
}

func TestParseLitePaginationInfoNoPagination(t *testing.T) {
	hasNextPage, nextOffset, vqd := parsePaginationInfo(mockDDGLiteHTML)
	require.False(t, hasNextPage)
	require.Equal(t, 0, nextOffset)
	require.Equal(t, "", vqd)
}

func TestDetectBreakageWithResultMarkers(t *testing.T) {
	warning := detectBreakage(mockDDGLiteHTMLBroken, 0)
	require.NotEmpty(t, warning)
	require.Contains(t, warning, "parser may be broken")
}

func TestDetectBreakageWithNoMarkers(t *testing.T) {
	warning := detectBreakage(mockDDGEmptyHTML, 0)
	require.Empty(t, warning)
}

func TestDetectBreakageWithResultsPresent(t *testing.T) {
	warning := detectBreakage(mockDDGLiteHTML, 3)
	require.Empty(t, warning)
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
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	defer srv.Close()

	cfg := defaultTestConfig()
	client := newSearchClient(cfg)
	client.baseURL = srv.URL
	client.httpClient = srv.Client()
	client.maxRetries = 1
	client.retryDelay = 10 * time.Millisecond

	resp, err := client.Search(context.Background(), "test", SearchOptions{})
	require.NoError(t, err)
	require.Len(t, resp.Results, 3)
	require.Equal(t, 2, callCount)
}

func TestSearchRetryExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	cfg := defaultTestConfig()
	client := newSearchClient(cfg)
	client.baseURL = srv.URL
	client.httpClient = srv.Client()
	client.maxRetries = 1
	client.retryDelay = 10 * time.Millisecond

	_, err := client.Search(context.Background(), "test", SearchOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rate limited")
}

func TestSearchConcurrentSafety(t *testing.T) {
	srv := newMockDDGServer(t, mockDDGLiteHTML, http.StatusOK)
	client := newTestClient(srv)

	const goroutines = 10
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			_, err := client.Search(context.Background(), "test", SearchOptions{})
			errCh <- err
		}()
	}

	for i := 0; i < goroutines; i++ {
		require.NoError(t, <-errCh)
	}
}

func TestSearchNewRequestError(t *testing.T) {
	cfg := defaultTestConfig()
	client := newSearchClient(cfg)
	client.baseURL = "http://\x00invalid"

	_, err := client.Search(context.Background(), "test", SearchOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create search request")
}

func TestSuccessResultFallback(t *testing.T) {
	r := successResult(make(chan int))
	require.Len(t, r.Content, 1)
	textContent, ok := r.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	require.NotEmpty(t, textContent.Text)
}

func TestParseResultsWithMalformedHTML(t *testing.T) {
	malformedHTML := `<html><body>random broken <div>stuff</div></body></html>`
	results, _, _ := parseLiteResults(malformedHTML)
	require.Empty(t, results)
}

func TestParseLiteResultsWithIncompleteResult(t *testing.T) {
	html := `<table border="0">
<tr><td valign="top">1.&nbsp;</td><td><a class="result-link" href="http://example.com">Title Only</a></td></tr>
</table>`
	results, _, _ := parseLiteResults(html)
	require.Len(t, results, 1)
	require.Equal(t, "Title Only", results[0].Title)
}

func BenchmarkParseResults(b *testing.B) {
	htmlContent := mockDDGLiteHTML
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseLiteResults(htmlContent)
	}
}

func BenchmarkSearch(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(mockDDGLiteHTML)); err != nil {
			b.Fatal(err)
		}
	}))
	defer srv.Close()

	client := newTestClient(srv)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Search(context.Background(), "test query", SearchOptions{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestSearchAllOptionsTogether verifies that region, safe search, and time
// range can be combined on the same GET request without one option clobbering
// another. All three should appear in the encoded query string.
func TestSearchAllOptionsTogether(t *testing.T) {
	var capturedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	opts := SearchOptions{
		Region:     "de-de",
		SafeSearch: 1,
		TimeRange:  "m",
	}
	resp, err := client.Search(context.Background(), "golang", opts)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Results)

	parsed, err := url.Parse(capturedURL)
	require.NoError(t, err)
	q := parsed.Query()
	require.Equal(t, "golang", q.Get("q"))
	require.Equal(t, "de-de", q.Get("kl"))
	require.Equal(t, "1", q.Get("kp"))
	require.Equal(t, "m", q.Get("df"))
}

// TestSearchWithOffsetAllOptionsTogether verifies that the POST-based
// pagination path also forwards all option fields simultaneously.
func TestSearchWithOffsetAllOptionsTogether(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		tWrite(t, w, []byte(mockDDGLiteHTML))
	}))
	defer srv.Close()

	client := newTestClient(srv)
	opts := SearchOptions{
		Region:     "uk-en",
		SafeSearch: -2,
		TimeRange:  "y",
	}
	_, err := client.SearchWithOffset(context.Background(), "test", 20, "tok", opts)
	require.NoError(t, err)

	form, err := url.ParseQuery(string(capturedBody))
	require.NoError(t, err)
	require.Equal(t, "test", form.Get("q"))
	require.Equal(t, "20", form.Get("s"))
	require.Equal(t, "tok", form.Get("vqd"))
	require.Equal(t, "uk-en", form.Get("kl"))
	require.Equal(t, "-2", form.Get("kp"))
	require.Equal(t, "y", form.Get("df"))
}

// TestWebSearchInputNegativeOffset confirms that negative offsets are
// rejected by validation. This was added to inputs.go to defend against
// callers passing nonsensical pagination values.
func TestWebSearchInputNegativeOffset(t *testing.T) {
	tests := []struct {
		name   string
		offset int
		want   bool // want error
	}{
		{"offset -1 rejected", -1, true},
		{"offset -100 rejected", -100, true},
		{"offset 0 allowed", 0, false},
		{"offset 10 with vqd allowed", 10, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := webSearchInput{
				Query:  "test",
				Offset: tc.offset,
			}
			if tc.offset > 0 {
				input.Vqd = "token" // required when offset > 0
			}
			err := input.validate()
			if tc.want {
				require.Error(t, err)
				require.Contains(t, err.Error(), "offset must be non-negative")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestWebSearchInputAllOptionsTogether confirms that the validator accepts a
// fully-populated input with every supported option set at once.
func TestWebSearchInputAllOptionsTogether(t *testing.T) {
	input := webSearchInput{
		Query:      "search me",
		Region:     "us-en",
		SafeSearch: 1,
		TimeRange:  "w",
		Offset:     20,
		Vqd:        "vqd-token-xyz",
	}
	err := input.validate()
	require.NoError(t, err)
	require.Equal(t, "search me", input.Query)
	require.Equal(t, "us-en", input.Region)
	require.Equal(t, 1, input.SafeSearch)
	require.Equal(t, "w", input.TimeRange)
	require.Equal(t, 20, input.Offset)
	require.Equal(t, "vqd-token-xyz", input.Vqd)
}

// TestWebSearchInputQueryWhitespaceTrimmed ensures the query is trimmed before
// validation (already covered indirectly elsewhere but explicit here for clarity).
func TestWebSearchInputQueryWhitespaceTrimmed(t *testing.T) {
	input := webSearchInput{Query: "  golang  "}
	err := input.validate()
	require.NoError(t, err)
	require.Equal(t, "golang", input.Query)
}

func TestWebSearchInputAllSafeSearchValues(t *testing.T) {
	for _, val := range []int{0, 1, -1, -2} {
		input := webSearchInput{Query: "test", SafeSearch: val}
		err := input.validate()
		require.NoError(t, err, "safe_search=%d should be valid", val)
	}
}

func TestWebSearchInputAllTimeRangeValues(t *testing.T) {
	for _, val := range []string{"", "d", "w", "m", "y"} {
		input := webSearchInput{Query: "test", TimeRange: val}
		err := input.validate()
		require.NoError(t, err, "time_range=%q should be valid", val)
	}
}
