package inputs

import (
	"strings"
	"testing"

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

func TestSearchValidate(t *testing.T) {
	tests := []struct {
		name      string
		input     Search
		expectErr bool
		errMsg    string
	}{
		{
			name:      "empty query",
			input:     Search{Query: ""},
			expectErr: true,
			errMsg:    "query must not be empty",
		},
		{
			name:      "valid query",
			input:     Search{Query: "test"},
			expectErr: false,
		},
		{
			name:      "offset without vqd",
			input:     Search{Query: "test", Offset: 10},
			expectErr: true,
			errMsg:    "vqd token is required",
		},
		{
			name:      "offset with vqd",
			input:     Search{Query: "test", Offset: 10, Vqd: "token"},
			expectErr: false,
		},
		{
			name:      "invalid region",
			input:     Search{Query: "test", Region: "invalid"},
			expectErr: true,
			errMsg:    "region must match format",
		},
		{
			name:      "valid region",
			input:     Search{Query: "test", Region: "us-en"},
			expectErr: false,
		},
		{
			name:      "invalid safe search",
			input:     Search{Query: "test", SafeSearch: 5},
			expectErr: true,
			errMsg:    "safe_search must be one of",
		},
		{
			name:      "valid safe search strict",
			input:     Search{Query: "test", SafeSearch: 1},
			expectErr: false,
		},
		{
			name:      "invalid time range",
			input:     Search{Query: "test", TimeRange: "x"},
			expectErr: true,
			errMsg:    "time_range must be one of",
		},
		{
			name:      "valid time range day",
			input:     Search{Query: "test", TimeRange: "d"},
			expectErr: false,
		},
		{
			name:      "query too long",
			input:     Search{Query: strings.Repeat("a", 501)},
			expectErr: true,
			errMsg:    "maximum length",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSearchValidateNegativeOffset(t *testing.T) {
	tests := []struct {
		name   string
		offset int
		want   bool
	}{
		{"offset -1 rejected", -1, true},
		{"offset -100 rejected", -100, true},
		{"offset 0 allowed", 0, false},
		{"offset 10 with vqd allowed", 10, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := Search{
				Query:  "test",
				Offset: tc.offset,
			}
			if tc.offset > 0 {
				input.Vqd = "token"
			}
			err := input.Validate()
			if tc.want {
				require.Error(t, err)
				require.Contains(t, err.Error(), "offset must be non-negative")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSearchValidateAllOptionsTogether(t *testing.T) {
	input := Search{
		Query:      "search me",
		Region:     "us-en",
		SafeSearch: 1,
		TimeRange:  "w",
		Offset:     20,
		Vqd:        "vqd-token-xyz",
	}
	err := input.Validate()
	require.NoError(t, err)
	require.Equal(t, "search me", input.Query)
	require.Equal(t, "us-en", input.Region)
	require.Equal(t, 1, input.SafeSearch)
	require.Equal(t, "w", input.TimeRange)
	require.Equal(t, 20, input.Offset)
	require.Equal(t, "vqd-token-xyz", input.Vqd)
}

func TestSearchValidateQueryWhitespaceTrimmed(t *testing.T) {
	input := Search{Query: "  golang  "}
	err := input.Validate()
	require.NoError(t, err)
	require.Equal(t, "golang", input.Query)
}

func TestSearchValidateAllSafeSearchValues(t *testing.T) {
	for _, val := range []int{0, 1, -1, -2} {
		input := Search{Query: "test", SafeSearch: val}
		err := input.Validate()
		require.NoError(t, err, "safe_search=%d should be valid", val)
	}
}

func TestSearchValidateAllTimeRangeValues(t *testing.T) {
	for _, val := range []string{"", "d", "w", "m", "y"} {
		input := Search{Query: "test", TimeRange: val}
		err := input.Validate()
		require.NoError(t, err, "time_range=%q should be valid", val)
	}
}

func TestFetchValidate(t *testing.T) {
	tests := []struct {
		name      string
		input     Fetch
		expectErr bool
		errMsg    string
		assertOK  func(t *testing.T, i Fetch)
	}{
		{
			name:      "valid https URL with default max length",
			input:     Fetch{URL: "https://example.com"},
			expectErr: false,
			assertOK: func(t *testing.T, i Fetch) {
				require.Equal(t, 10000, i.MaxLength)
			},
		},
		{
			name:      "empty URL rejected",
			input:     Fetch{URL: ""},
			expectErr: true,
			errMsg:    "url must not be empty",
		},
		{
			name:      "whitespace-only URL rejected (trimmed to empty)",
			input:     Fetch{URL: "   "},
			expectErr: true,
			errMsg:    "url must not be empty",
		},
		{
			name:      "ftp scheme rejected",
			input:     Fetch{URL: "ftp://example.com"},
			expectErr: true,
			errMsg:    "valid HTTP or HTTPS URL",
		},
		{
			name:      "file scheme rejected",
			input:     Fetch{URL: "file:///etc/passwd"},
			expectErr: true,
			errMsg:    "valid HTTP or HTTPS URL",
		},
		{
			name:      "MaxLength above ceiling is clamped",
			input:     Fetch{URL: "https://example.com", MaxLength: 100000},
			expectErr: false,
			assertOK: func(t *testing.T, i Fetch) {
				require.Equal(t, 50000, i.MaxLength)
			},
		},
		{
			name:      "MaxLength within range is preserved",
			input:     Fetch{URL: "https://example.com", MaxLength: 5000},
			expectErr: false,
			assertOK: func(t *testing.T, i Fetch) {
				require.Equal(t, 5000, i.MaxLength)
			},
		},
		{
			name:      "URL surrounding whitespace is trimmed",
			input:     Fetch{URL: "  https://example.com  "},
			expectErr: false,
			assertOK: func(t *testing.T, i Fetch) {
				require.Equal(t, "https://example.com", i.URL)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
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

func TestFetchValidatePrivateIPDetection(t *testing.T) {
	tests := []struct {
		name      string
		input     Fetch
		expectErr bool
	}{
		{
			name:      "empty URL",
			input:     Fetch{URL: ""},
			expectErr: true,
		},
		{
			name:      "valid https URL",
			input:     Fetch{URL: "https://example.com"},
			expectErr: false,
		},
		{
			name:      "valid http URL",
			input:     Fetch{URL: "http://example.com"},
			expectErr: false,
		},
		{
			name:      "ftp URL rejected",
			input:     Fetch{URL: "ftp://example.com"},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
