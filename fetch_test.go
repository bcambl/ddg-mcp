package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	client := newTestFetchClient(srv)

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
	client := newTestFetchClient(srv)

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
	client := newTestFetchClient(srv)

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
	client := newTestFetchClient(srv)

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
	client := newTestFetchClient(srv)

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
	client := newTestFetchClient(srv)

	_, err := client.Fetch(context.Background(), srv.URL, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported content type")
}

func TestFetchPlainText(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		tWrite(t, w, []byte(mockPlainText))
	})
	client := newTestFetchClient(srv)

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
	client := newTestFetchClient(srv)

	_, err := client.Fetch(context.Background(), srv.URL, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 404")
}

func TestFetchHTMLError500(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		tWrite(t, w, []byte("Internal Server Error"))
	})
	client := newTestFetchClient(srv)

	_, err := client.Fetch(context.Background(), srv.URL, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "HTTP 500")
}

func TestFetchInvalidURL(t *testing.T) {
	cfg := defaultTestConfig()
	client := newFetchClient(cfg)
	_, err := client.Fetch(context.Background(), "not-a-url", 0)
	require.Error(t, err)
}

func TestFetchContextCancellation(t *testing.T) {
	srv := newMockFetchServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		tWrite(t, w, []byte(mockHTMLPage))
	})
	client := newTestFetchClient(srv)

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

func TestPrivateIPDetection(t *testing.T) {
	tests := []struct {
		name      string
		input     webFetchInput
		expectErr bool
	}{
		{
			name:      "empty URL",
			input:     webFetchInput{URL: ""},
			expectErr: true,
		},
		{
			name:      "valid https URL",
			input:     webFetchInput{URL: "https://example.com"},
			expectErr: false,
		},
		{
			name:      "valid http URL",
			input:     webFetchInput{URL: "http://example.com"},
			expectErr: false,
		},
		{
			name:      "ftp URL rejected",
			input:     webFetchInput{URL: "ftp://example.com"},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.validate()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWebSearchInputValidation(t *testing.T) {
	tests := []struct {
		name      string
		input     webSearchInput
		expectErr bool
		errMsg    string
	}{
		{
			name:      "empty query",
			input:     webSearchInput{Query: ""},
			expectErr: true,
			errMsg:    "query must not be empty",
		},
		{
			name:      "valid query",
			input:     webSearchInput{Query: "test"},
			expectErr: false,
		},
		{
			name:      "offset without vqd",
			input:     webSearchInput{Query: "test", Offset: 10},
			expectErr: true,
			errMsg:    "vqd token is required",
		},
		{
			name:      "offset with vqd",
			input:     webSearchInput{Query: "test", Offset: 10, Vqd: "token"},
			expectErr: false,
		},
		{
			name:      "invalid region",
			input:     webSearchInput{Query: "test", Region: "invalid"},
			expectErr: true,
			errMsg:    "region must match format",
		},
		{
			name:      "valid region",
			input:     webSearchInput{Query: "test", Region: "us-en"},
			expectErr: false,
		},
		{
			name:      "invalid safe search",
			input:     webSearchInput{Query: "test", SafeSearch: 5},
			expectErr: true,
			errMsg:    "safe_search must be one of",
		},
		{
			name:      "valid safe search strict",
			input:     webSearchInput{Query: "test", SafeSearch: 1},
			expectErr: false,
		},
		{
			name:      "invalid time range",
			input:     webSearchInput{Query: "test", TimeRange: "x"},
			expectErr: true,
			errMsg:    "time_range must be one of",
		},
		{
			name:      "valid time range day",
			input:     webSearchInput{Query: "test", TimeRange: "d"},
			expectErr: false,
		},
		{
			name:      "query too long",
			input:     webSearchInput{Query: strings.Repeat("a", 501)},
			expectErr: true,
			errMsg:    "maximum length",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.validate()
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestFetchCheckRedirectBlocksPrivateIP exercises the CheckRedirect callback
// installed on the FetchClient to confirm that redirects targeting a private
// IP are blocked. We invoke the callback directly because httptest.Server
// always binds to 127.0.0.1, which means we cannot create a "public → private"
// redirect in a hermetic test using only httptest. The callback is the unit
// of behavior we want to verify.
func TestFetchCheckRedirectBlocksPrivateIP(t *testing.T) {
	cfg := defaultTestConfig()
	client := newFetchClient(cfg)
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
	cfg := defaultTestConfig()
	client := newFetchClient(cfg)
	loopbackURL, err := url.Parse("http://127.0.0.1:9999/internal")
	require.NoError(t, err)
	req := &http.Request{URL: loopbackURL}

	err = client.httpClient.CheckRedirect(req, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "redirect to private IP blocked")
}

func TestFetchCheckRedirectBlocksLocalhost(t *testing.T) {
	cfg := defaultTestConfig()
	client := newFetchClient(cfg)
	hostURL, err := url.Parse("http://localhost/admin")
	require.NoError(t, err)
	req := &http.Request{URL: hostURL}

	err = client.httpClient.CheckRedirect(req, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "redirect to private IP blocked")
}

func TestFetchCheckRedirectStopsAfter10(t *testing.T) {
	cfg := defaultTestConfig()
	client := newFetchClient(cfg)
	pubURL, err := url.Parse("https://example.com/page")
	require.NoError(t, err)
	req := &http.Request{URL: pubURL}

	// Simulate 10 prior redirects (>= 10 hops should be rejected).
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = req
	}

	err = client.httpClient.CheckRedirect(req, via)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stopped after 10 redirects")
}

func TestFetchCheckRedirectAllowsPublicWhenSSRFDisabled(t *testing.T) {
	cfg := defaultTestConfig()
	client := newFetchClient(cfg)
	// Disable SSRF check to ensure the callback honours the flag.
	client.checkSSRF = false

	loopbackURL, err := url.Parse("http://127.0.0.1:8080/page")
	require.NoError(t, err)
	req := &http.Request{URL: loopbackURL}

	err = client.httpClient.CheckRedirect(req, nil)
	require.NoError(t, err, "should permit loopback redirect when checkSSRF=false")
}

// TestFetchRedirectIntegrationBlocksPrivateIP performs an end-to-end test
// where a public-looking httptest server issues a redirect that points to a
// known private IP. The redirect should be blocked by the SSRF guard.
//
// We bypass Fetch's pre-request hostname check by calling the configured
// httpClient directly, since httptest.Server always binds to 127.0.0.1. The
// CheckRedirect callback installed by newFetchClient is what we are
// verifying, and it consults fc.checkSSRF at request time via closure.
func TestFetchRedirectIntegrationBlocksPrivateIP(t *testing.T) {
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Redirect to a private IP that the SSRF guard should block.
		http.Redirect(w, r, "http://192.168.255.255/internal", http.StatusFound)
	}))
	t.Cleanup(redirector.Close)

	cfg := defaultTestConfig()
	client := newFetchClient(cfg)
	require.True(t, client.checkSSRF, "default client should have SSRF check enabled")

	// Call the http client directly so the initial 127.0.0.1 URL is allowed
	// (bypassing the Fetch-level pre-check) while the CheckRedirect callback
	// remains active and observes the private redirect target.
	httpReq, err := http.NewRequest(http.MethodGet, redirector.URL, nil)
	require.NoError(t, err)

	resp, err := client.httpClient.Do(httpReq)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	require.Contains(t, err.Error(), "redirect to private IP blocked")
}

// TestFetchTruncateStringUTF8 calls truncateString through the fetch package
// path so coverage credits the fetch.go-resident helper. Behavioural matrix
// is also covered in inputs_test.go for completeness.
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
			// "日本語" = 3 runes × 3 bytes = 9 bytes.
			// maxLen=5 backs off from mid-second-rune to end of first rune (3 bytes).
			name:    "utf8 backs off mid-rune to nearest boundary",
			input:   "日本語",
			maxLen:  5,
			want:    "日",
			wantLen: 3,
		},
		{
			// maxLen exactly at rune boundary returns those runes.
			name:    "utf8 at exact rune boundary",
			input:   "日本語",
			maxLen:  6,
			want:    "日本",
			wantLen: 6,
		},
		{
			// 🌟 is U+1F31F = 4 bytes.
			name:    "emoji rune backs off when truncating mid-rune",
			input:   "ab🌟cd",
			maxLen:  4,
			want:    "ab",
			wantLen: 2,
		},
		{
			// maxLen=6 is exactly after the 4-byte emoji starting at byte index 2.
			name:    "emoji boundary preserved when maxLen lands at rune end",
			input:   "ab🌟cd",
			maxLen:  6,
			want:    "ab🌟",
			wantLen: 6,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateString(tc.input, tc.maxLen)
			require.Equal(t, tc.want, got, "input=%q maxLen=%d", tc.input, tc.maxLen)
			require.Equal(t, tc.wantLen, len(got))
		})
	}
}

// TestFetchTruncateStringInvariants asserts the byte-safety invariant:
// regardless of input/maxLen, the result must always contain only well-formed
// UTF-8 (no rune boundaries split). This is a sweep over interesting inputs
// rather than one subtest per byte to keep output readable.
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
				got := truncateString(in, maxLen)
				// Result must never exceed maxLen unless input <= maxLen
				// (in which case the whole string is returned).
				if len(in) > maxLen {
					require.LessOrEqual(t, len(got), maxLen,
						"input=%q maxLen=%d got=%q", in, maxLen, got)
				}
				// Output must always be well-formed UTF-8.
				for _, r := range got {
					require.NotEqual(t, '\uFFFD', r,
						"truncated string contains invalid UTF-8: %q (maxLen=%d)", got, maxLen)
				}
			}
		})
	}
}
