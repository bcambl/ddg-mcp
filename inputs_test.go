package main

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestIsValidRegion(t *testing.T) {
	tests := []struct {
		name   string
		region string
		want   bool
	}{
		{
			name:   "empty string is valid",
			region: "",
			want:   true,
		},
		{
			name:   "us-en",
			region: "us-en",
			want:   true,
		},
		{
			name:   "wt-wt (no region)",
			region: "wt-wt",
			want:   true,
		},
		{
			name:   "de-de",
			region: "de-de",
			want:   true,
		},
		{
			name:   "uk-en",
			region: "uk-en",
			want:   true,
		},
		{
			name:   "three letter parts (abc-def)",
			region: "abc-def",
			want:   true,
		},
		{
			name:   "two letters only is invalid",
			region: "XX",
			want:   false,
		},
		{
			name:   "too long (us-english)",
			region: "us-english",
			want:   false,
		},
		{
			name:   "missing dash",
			region: "us",
			want:   false,
		},
		{
			name:   "uppercase US-EN",
			region: "US-EN",
			want:   false,
		},
		{
			name:   "mixed case Us-En",
			region: "Us-En",
			want:   false,
		},
		{
			name:   "single dash with empty parts",
			region: "-",
			want:   false,
		},
		{
			name:   "three parts (multiple dashes)",
			region: "us-en-extra",
			want:   false,
		},
		{
			name:   "trailing dash",
			region: "us-",
			want:   false,
		},
		{
			name:   "leading dash",
			region: "-en",
			want:   false,
		},
		{
			name:   "single letter parts",
			region: "u-e",
			want:   false,
		},
		{
			name:   "digit in region",
			region: "us-e1",
			want:   false,
		},
		{
			name:   "underscore instead of dash",
			region: "us_en",
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidRegion(tc.region)
			require.Equal(t, tc.want, got, "isValidRegion(%q)", tc.region)
		})
	}
}

func TestIsValidSafeSearch(t *testing.T) {
	tests := []struct {
		name string
		val  int
		want bool
	}{
		{"zero (default)", 0, true},
		{"one (strict)", 1, true},
		{"negative one (moderate)", -1, true},
		{"negative two (off)", -2, true},
		{"three (invalid)", 3, false},
		{"negative five (invalid)", -5, false},
		{"one hundred (invalid)", 100, false},
		{"two (invalid)", 2, false},
		{"negative three (invalid)", -3, false},
		{"large positive (invalid)", 1 << 30, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidSafeSearch(tc.val)
			require.Equal(t, tc.want, got, "isValidSafeSearch(%d)", tc.val)
		})
	}
}

func TestIsValidTimeRange(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{"empty (any time)", "", true},
		{"day", "d", true},
		{"week", "w", true},
		{"month", "m", true},
		{"year", "y", true},
		{"invalid letter x", "x", false},
		{"full word day", "day", false},
		{"uppercase M", "M", false},
		{"uppercase D", "D", false},
		{"week (full)", "week", false},
		{"hour", "h", false},
		{"whitespace", " ", false},
		{"trailing whitespace", "d ", false},
		{"leading whitespace", " d", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isValidTimeRange(tc.val)
			require.Equal(t, tc.want, got, "isValidTimeRange(%q)", tc.val)
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	t.Run("localhost is blocked", func(t *testing.T) {
		err := isPrivateIP("localhost")
		require.Error(t, err)
		require.ErrorIs(t, err, errPrivateIP)
	})

	t.Run("127.0.0.1 is blocked", func(t *testing.T) {
		err := isPrivateIP("127.0.0.1")
		require.Error(t, err)
		require.ErrorIs(t, err, errPrivateIP)
	})

	t.Run("IPv6 ::1 is blocked", func(t *testing.T) {
		err := isPrivateIP("::1")
		require.Error(t, err)
		require.ErrorIs(t, err, errPrivateIP)
	})

	t.Run("unresolvable hostname returns DNS error", func(t *testing.T) {
		// .invalid TLD is reserved per RFC 2606 and guaranteed not to resolve.
		err := isPrivateIP("this-hostname-should-not-exist-12345abc.invalid")
		require.Error(t, err)
		// This must NOT be the private IP error - it should be a DNS lookup error.
		require.NotErrorIs(t, err, errPrivateIP)
		require.Contains(t, err.Error(), "failed to resolve hostname")
	})

	t.Run("IPv4 private range 192.168.x.x is blocked", func(t *testing.T) {
		err := isPrivateIP("192.168.1.1")
		require.Error(t, err)
		require.ErrorIs(t, err, errPrivateIP)
	})

	t.Run("IPv4 private range 10.x.x.x is blocked", func(t *testing.T) {
		err := isPrivateIP("10.0.0.1")
		require.Error(t, err)
		require.ErrorIs(t, err, errPrivateIP)
	})

	t.Run("IPv4 private range 172.16.x.x is blocked", func(t *testing.T) {
		err := isPrivateIP("172.16.0.1")
		require.Error(t, err)
		require.ErrorIs(t, err, errPrivateIP)
	})

	t.Run("link-local IPv4 169.254.x.x is blocked", func(t *testing.T) {
		err := isPrivateIP("169.254.169.254")
		require.Error(t, err)
		require.ErrorIs(t, err, errPrivateIP)
	})

	t.Run("0.0.0.0 unspecified address is blocked", func(t *testing.T) {
		err := isPrivateIP("0.0.0.0")
		require.Error(t, err)
		require.ErrorIs(t, err, errPrivateIP)
	})

	t.Run("0.x.x.x special-use range is blocked", func(t *testing.T) {
		// Per the implementation, any IPv4 starting with 0 is rejected.
		err := isPrivateIP("0.1.2.3")
		require.Error(t, err)
		require.ErrorIs(t, err, errPrivateIP)
	})

	t.Run("multicast IPv4 224.x.x.x is blocked", func(t *testing.T) {
		err := isPrivateIP("224.0.0.1")
		require.Error(t, err)
		require.ErrorIs(t, err, errPrivateIP)
	})
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		maxLen  int
		want    string
		wantLen int
	}{
		{
			name:    "shorter than max returned unchanged",
			input:   "hello",
			maxLen:  100,
			want:    "hello",
			wantLen: 5,
		},
		{
			name:    "exact length boundary returned unchanged",
			input:   "Hello, World!",
			maxLen:  13,
			want:    "Hello, World!",
			wantLen: 13,
		},
		{
			name:    "ASCII truncation at clean boundary",
			input:   "Hello, World!",
			maxLen:  5,
			want:    "Hello",
			wantLen: 5,
		},
		{
			name:    "zero maxLen returns empty",
			input:   "abc",
			maxLen:  0,
			want:    "",
			wantLen: 0,
		},
		{
			name:    "empty input returns empty",
			input:   "",
			maxLen:  10,
			want:    "",
			wantLen: 0,
		},
		{
			// "日" is 3 bytes each; maxLen=4 lands mid-second-rune so we back
			// off to the byte position that ends the first rune (3 bytes).
			name:    "multi-byte UTF-8 truncated mid-rune backs off",
			input:   "日本語",
			maxLen:  4,
			want:    "日",
			wantLen: 3,
		},
		{
			// 6 bytes = exactly two complete 3-byte runes.
			name:    "multi-byte UTF-8 truncated at exact rune boundary",
			input:   "日本語",
			maxLen:  6,
			want:    "日本",
			wantLen: 6,
		},
		{
			// 9 byte string; maxLen=9 returns whole string.
			name:    "multi-byte UTF-8 full content at exact byte length",
			input:   "日本語",
			maxLen:  9,
			want:    "日本語",
			wantLen: 9,
		},
		{
			// maxLen above content length returns whole string.
			name:    "multi-byte UTF-8 with excess maxLen",
			input:   "日本語",
			maxLen:  100,
			want:    "日本語",
			wantLen: 9,
		},
		{
			// "Hello 🌍 World" - 🌍 is 4 bytes (U+1F30D).
			// "Hello " is 6 bytes, then 🌍 takes bytes 6-9.
			// maxLen=8 lands inside the emoji; should back off to 6 bytes.
			name:    "emoji truncation backs off from mid-rune",
			input:   "Hello 🌍 World",
			maxLen:  8,
			want:    "Hello ",
			wantLen: 6,
		},
		{
			// "Hello " (6 bytes) + "🌍" (4 bytes) = 10 bytes after emoji.
			name:    "emoji truncation at exact boundary after emoji",
			input:   "Hello 🌍 World",
			maxLen:  10,
			want:    "Hello 🌍",
			wantLen: 10,
		},
		{
			// 🌍 is 4 bytes; maxLen=2 lands mid-emoji and backs off to 0.
			name:    "emoji at start truncated cleanly to empty",
			input:   "🌍hi",
			maxLen:  2,
			want:    "",
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateString(tc.input, tc.maxLen)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.wantLen, len(got))
		})
	}
}

func TestWebFetchInputValidate(t *testing.T) {
	tests := []struct {
		name      string
		input     webFetchInput
		expectErr bool
		errMsg    string
		assertOK  func(t *testing.T, i webFetchInput)
	}{
		{
			name:      "valid https URL with default max length",
			input:     webFetchInput{URL: "https://example.com"},
			expectErr: false,
			assertOK: func(t *testing.T, i webFetchInput) {
				require.Equal(t, 10000, i.MaxLength)
			},
		},
		{
			name:      "empty URL rejected",
			input:     webFetchInput{URL: ""},
			expectErr: true,
			errMsg:    "url must not be empty",
		},
		{
			name:      "whitespace-only URL rejected (trimmed to empty)",
			input:     webFetchInput{URL: "   "},
			expectErr: true,
			errMsg:    "url must not be empty",
		},
		{
			name:      "ftp scheme rejected",
			input:     webFetchInput{URL: "ftp://example.com"},
			expectErr: true,
			errMsg:    "valid HTTP or HTTPS URL",
		},
		{
			name:      "file scheme rejected",
			input:     webFetchInput{URL: "file:///etc/passwd"},
			expectErr: true,
			errMsg:    "valid HTTP or HTTPS URL",
		},
		{
			name:      "MaxLength above ceiling is clamped",
			input:     webFetchInput{URL: "https://example.com", MaxLength: 100000},
			expectErr: false,
			assertOK: func(t *testing.T, i webFetchInput) {
				require.Equal(t, 50000, i.MaxLength)
			},
		},
		{
			name:      "MaxLength within range is preserved",
			input:     webFetchInput{URL: "https://example.com", MaxLength: 5000},
			expectErr: false,
			assertOK: func(t *testing.T, i webFetchInput) {
				require.Equal(t, 5000, i.MaxLength)
			},
		},
		{
			name:      "URL surrounding whitespace is trimmed",
			input:     webFetchInput{URL: "  https://example.com  "},
			expectErr: false,
			assertOK: func(t *testing.T, i webFetchInput) {
				require.Equal(t, "https://example.com", i.URL)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.validate()
			if tc.expectErr {
				require.Error(t, err)
				if tc.errMsg != "" {
					require.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				require.NoError(t, err)
				if tc.assertOK != nil {
					tc.assertOK(t, tc.input)
				}
			}
		})
	}
}

func TestErrorResultHelper(t *testing.T) {
	res := errorResult(errEmptyQuery)
	require.NotNil(t, res)
	require.True(t, res.IsError)
	require.Len(t, res.Content, 1)

	tc, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected *mcp.TextContent, got %T", res.Content[0])
	require.Contains(t, tc.Text, errEmptyQuery.Error())
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
