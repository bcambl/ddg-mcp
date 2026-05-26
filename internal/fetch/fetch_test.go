package fetch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcambl/ddg-mcp/internal/config"
	"github.com/bcambl/ddg-mcp/internal/utils"
)

func tWrite(t *testing.T, w http.ResponseWriter, data []byte) {
	t.Helper()
	_, err := w.Write(data)
	require.NoError(t, err)
}

const mockHTMLPage = `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<nav>Navigation</nav>
<header>Header</header>
<main>
<h1>Hello World</h1>
<p>This is a test page with some content.</p>
<script>alert('hello');</script>
<style>body { color: red; }</style>
<ul><li>Item 1</li><li>Item 2</li></ul>
</main>
<footer>Footer</footer>
</body>
</html>`

const mockPlainText = "This is plain text content without any HTML."

func newTestClient(srv *httptest.Server) *Client {
	cfg := config.DefaultTestConfig()
	client := NewClient(ClientOptions{
		HTTPClient:  srv.Client(),
		UserAgent:   cfg.FetchUserAgent,
		MaxBodySize: cfg.MaxBodySize,
		CheckSSRF:   false,
		Timeout:     cfg.FetchTimeout,
	})
	return client
}

func newMockFetchServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchBasicHTML(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tWrite(t, w, []byte(mockHTMLPage))
	})
	client := newTestClient(srv)

	result, err := client.Fetch(context.Background(), srv.URL, 0)
	require.NoError(t, err)
	require.Equal(t, "Test Page", result.Title)
	require.Contains(t, result.Content, "Hello World")
	require.Contains(t, result.Content, "This is a test page")
	require.Contains(t, result.ContentType, "text/html")
	require.False(t, result.Truncated)
}

func TestFetchStripsScriptsAndStyles(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tWrite(t, w, []byte(mockHTMLPage))
	})
	client := newTestClient(srv)

	result, err := client.Fetch(context.Background(), srv.URL, 0)
	require.NoError(t, err)
	require.NotContains(t, result.Content, "alert")
	require.NotContains(t, result.Content, "color: red")
}

func TestFetchStripsNavFooterHeader(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tWrite(t, w, []byte(mockHTMLPage))
	})
	client := newTestClient(srv)

	result, err := client.Fetch(context.Background(), srv.URL, 0)
	require.NoError(t, err)
	require.NotContains(t, result.Content, "Navigation")
	require.NotContains(t, result.Content, "Header")
	require.NotContains(t, result.Content, "Footer")
}

func TestFetchExtractsTitle(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tWrite(t, w, []byte(`<html><head><title>My Title</title></head><body>Content</body></html>`))
	})
	client := newTestClient(srv)

	result, err := client.Fetch(context.Background(), srv.URL, 0)
	require.NoError(t, err)
	require.Equal(t, "My Title", result.Title)
}

func TestFetchTruncation(t *testing.T) {
	longContent := strings.Repeat("word ", 10000)
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tWrite(t, w, []byte(`<html><head><title>Long</title></head><body>`+longContent+`</body></html>`))
	})
	client := newTestClient(srv)

	result, err := client.Fetch(context.Background(), srv.URL, 100)
	require.NoError(t, err)
	require.True(t, result.Truncated)
	require.LessOrEqual(t, result.ContentLength, 100)
}

func TestFetchContentTypeFiltering(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		tWrite(t, w, []byte(`{"error": "not html"}`))
	})
	client := newTestClient(srv)

	_, err := client.Fetch(context.Background(), srv.URL, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported content type")
}

func TestFetchPlainText(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		tWrite(t, w, []byte(mockPlainText))
	})
	client := newTestClient(srv)

	result, err := client.Fetch(context.Background(), srv.URL, 0)
	require.NoError(t, err)
	require.Contains(t, result.Content, "This is plain text content")
	require.Equal(t, "", result.Title)
}

func TestFetchHTMLError404(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		tWrite(t, w, []byte("Not Found"))
	})
	client := newTestClient(srv)

	_, err := client.Fetch(context.Background(), srv.URL, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 404")
}

func TestFetchHTMLError500(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		tWrite(t, w, []byte("Internal Server Error"))
	})
	client := newTestClient(srv)

	_, err := client.Fetch(context.Background(), srv.URL, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 500")
}

func TestFetchInvalidURL(t *testing.T) {
	cfg := config.DefaultTestConfig()
	client := NewClient(ClientOptions{
		UserAgent:   cfg.FetchUserAgent,
		MaxBodySize: cfg.MaxBodySize,
		CheckSSRF:   cfg.SSRFProtection,
		Timeout:     cfg.FetchTimeout,
	})
	_, err := client.Fetch(context.Background(), "not-a-url", 0)
	require.Error(t, err)
}

func TestFetchContextCancellation(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tWrite(t, w, []byte(mockHTMLPage))
	})
	client := newTestClient(srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Fetch(ctx, srv.URL, 0)
	require.Error(t, err)
}

func TestCollapseWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "multiple spaces",
			input:    "hello   world",
			expected: "hello world",
		},
		{
			name:     "multiple newlines",
			input:    "hello\n\n\nworld",
			expected: "hello\nworld",
		},
		{
			name:     "mixed whitespace",
			input:    "hello \n  \n  world",
			expected: "hello \nworld",
		},
		{
			name:     "leading trailing whitespace",
			input:    "  hello world  ",
			expected: "hello world",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := collapseWhitespace(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestFetchCheckRedirectBlocksPrivateIP(t *testing.T) {
	cfg := config.DefaultTestConfig()
	client := NewClient(ClientOptions{
		UserAgent:   cfg.FetchUserAgent,
		MaxBodySize: cfg.MaxBodySize,
		CheckSSRF:   cfg.SSRFProtection,
		Timeout:     cfg.FetchTimeout,
	})
	require.True(t, client.checkSSRF, "SSRF check should be enabled by default")
	require.NotNil(t, client.httpClient.CheckRedirect, "CheckRedirect must be configured")

	privateURL, err := url.Parse("http://192.168.1.1/secret")
	require.NoError(t, err)
	req := &http.Request{URL: privateURL}

	err = client.httpClient.CheckRedirect(req, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "redirect to private IP blocked")
}

func TestFetchCheckRedirectBlocksLoopback(t *testing.T) {
	cfg := config.DefaultTestConfig()
	client := NewClient(ClientOptions{
		UserAgent:   cfg.FetchUserAgent,
		MaxBodySize: cfg.MaxBodySize,
		CheckSSRF:   cfg.SSRFProtection,
		Timeout:     cfg.FetchTimeout,
	})
	loopbackURL, err := url.Parse("http://127.0.0.1:9999/internal")
	require.NoError(t, err)
	req := &http.Request{URL: loopbackURL}

	err = client.httpClient.CheckRedirect(req, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "redirect to private IP blocked")
}

func TestFetchCheckRedirectBlocksLocalhost(t *testing.T) {
	cfg := config.DefaultTestConfig()
	client := NewClient(ClientOptions{
		UserAgent:   cfg.FetchUserAgent,
		MaxBodySize: cfg.MaxBodySize,
		CheckSSRF:   cfg.SSRFProtection,
		Timeout:     cfg.FetchTimeout,
	})
	hostURL, err := url.Parse("http://localhost/admin")
	require.NoError(t, err)
	req := &http.Request{URL: hostURL}

	err = client.httpClient.CheckRedirect(req, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "redirect to private IP blocked")
}

func TestFetchCheckRedirectStopsAfter10(t *testing.T) {
	cfg := config.DefaultTestConfig()
	client := NewClient(ClientOptions{
		UserAgent:   cfg.FetchUserAgent,
		MaxBodySize: cfg.MaxBodySize,
		CheckSSRF:   cfg.SSRFProtection,
		Timeout:     cfg.FetchTimeout,
	})
	pubURL, err := url.Parse("https://example.com/page")
	require.NoError(t, err)
	req := &http.Request{URL: pubURL}

	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = req
	}

	err = client.httpClient.CheckRedirect(req, via)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stopped after 10 redirects")
}

func TestFetchCheckRedirectAllowsPublicWhenSSRFDisabled(t *testing.T) {
	cfg := config.DefaultTestConfig()
	client := NewClient(ClientOptions{
		UserAgent:   cfg.FetchUserAgent,
		MaxBodySize: cfg.MaxBodySize,
		CheckSSRF:   false,
		Timeout:     cfg.FetchTimeout,
	})

	loopbackURL, err := url.Parse("http://127.0.0.1:8080/page")
	require.NoError(t, err)
	req := &http.Request{URL: loopbackURL}

	err = client.httpClient.CheckRedirect(req, nil)
	require.NoError(t, err, "should permit loopback redirect when checkSSRF=false")
}

func TestFetchRedirectIntegrationBlocksPrivateIP(t *testing.T) {
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://192.168.255.255/internal", http.StatusFound)
	}))
	t.Cleanup(redirector.Close)

	cfg := config.DefaultTestConfig()
	client := NewClient(ClientOptions{
		UserAgent:   cfg.FetchUserAgent,
		MaxBodySize: cfg.MaxBodySize,
		CheckSSRF:   cfg.SSRFProtection,
		Timeout:     cfg.FetchTimeout,
	})
	require.True(t, client.checkSSRF, "default client should have SSRF check enabled")

	httpReq, err := http.NewRequest(http.MethodGet, redirector.URL, nil)
	require.NoError(t, err)

	resp, err := client.httpClient.Do(httpReq)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	require.Contains(t, err.Error(), "redirect to private IP blocked")
}

func TestFetchTruncateStringUTF8(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		maxLen  int
		want    string
		wantLen int
	}{
		{
			name:    "ascii within max returns unchanged",
			input:   "hello",
			maxLen:  10,
			want:    "hello",
			wantLen: 5,
		},
		{
			name:    "ascii at exact boundary",
			input:   "abcdefghij",
			maxLen:  10,
			want:    "abcdefghij",
			wantLen: 10,
		},
		{
			name:    "ascii truncated to half",
			input:   "abcdefghij",
			maxLen:  5,
			want:    "abcde",
			wantLen: 5,
		},
		{
			name:    "utf8 backs off mid-rune to nearest boundary",
			input:   "日本語",
			maxLen:  5,
			want:    "日",
			wantLen: 3,
		},
		{
			name:    "utf8 at exact rune boundary",
			input:   "日本語",
			maxLen:  6,
			want:    "日本",
			wantLen: 6,
		},
		{
			name:    "emoji rune backs off when truncating mid-rune",
			input:   "ab🌟cd",
			maxLen:  4,
			want:    "ab",
			wantLen: 2,
		},
		{
			name:    "emoji boundary preserved when maxLen lands at rune end",
			input:   "ab🌟cd",
			maxLen:  6,
			want:    "ab🌟",
			wantLen: 6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := utils.TruncateString(tc.input, tc.maxLen)
			require.Equal(t, tc.want, got, "input=%q maxLen=%d", tc.input, tc.maxLen)
			require.Equal(t, tc.wantLen, len(got))
		})
	}
}

func TestFetchTruncateStringInvariants(t *testing.T) {
	inputs := []string{
		"plain ascii string",
		"日本語テスト",
		"🚀🌍🌟emoji galore",
		"mixed: hello 日本 🌟 world",
		"",
	}
	for _, in := range inputs {
		t.Run(fmt.Sprintf("input=%q", in), func(t *testing.T) {
			for maxLen := 0; maxLen <= len(in)+5; maxLen++ {
				got := utils.TruncateString(in, maxLen)
				if len(in) > maxLen {
					require.LessOrEqual(t, len(got), maxLen,
						"input=%q maxLen=%d got=%q", in, maxLen, got)
				}
				for _, r := range got {
					require.NotEqual(t, '\uFFFD', r,
						"truncated string contains invalid UTF-8: %q (maxLen=%d)", got, maxLen)
				}
			}
		})
	}
}
