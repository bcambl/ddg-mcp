package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsPrivateIP(t *testing.T) {
	t.Run("localhost is blocked", func(t *testing.T) {
		err := IsPrivateIP("localhost")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPrivateIP)
	})

	t.Run("127.0.0.1 is blocked", func(t *testing.T) {
		err := IsPrivateIP("127.0.0.1")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPrivateIP)
	})

	t.Run("IPv6 ::1 is blocked", func(t *testing.T) {
		err := IsPrivateIP("::1")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPrivateIP)
	})

	t.Run("unresolvable hostname returns DNS error", func(t *testing.T) {
		err := IsPrivateIP("this-hostname-should-not-exist-12345abc.invalid")
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrPrivateIP)
		require.Contains(t, err.Error(), "failed to resolve hostname")
	})

	t.Run("IPv4 private range 192.168.x.x is blocked", func(t *testing.T) {
		err := IsPrivateIP("192.168.1.1")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPrivateIP)
	})

	t.Run("IPv4 private range 10.x.x.x is blocked", func(t *testing.T) {
		err := IsPrivateIP("10.0.0.1")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPrivateIP)
	})

	t.Run("IPv4 private range 172.16.x.x is blocked", func(t *testing.T) {
		err := IsPrivateIP("172.16.0.1")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPrivateIP)
	})

	t.Run("link-local IPv4 169.254.x.x is blocked", func(t *testing.T) {
		err := IsPrivateIP("169.254.169.254")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPrivateIP)
	})

	t.Run("0.0.0.0 unspecified address is blocked", func(t *testing.T) {
		err := IsPrivateIP("0.0.0.0")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPrivateIP)
	})

	t.Run("0.x.x.x special-use range is blocked", func(t *testing.T) {
		err := IsPrivateIP("0.1.2.3")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPrivateIP)
	})

	t.Run("multicast IPv4 224.x.x.x is blocked", func(t *testing.T) {
		err := IsPrivateIP("224.0.0.1")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPrivateIP)
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
			name:    "multi-byte UTF-8 truncated mid-rune backs off",
			input:   "日本語",
			maxLen:  4,
			want:    "日",
			wantLen: 3,
		},
		{
			name:    "multi-byte UTF-8 truncated at exact rune boundary",
			input:   "日本語",
			maxLen:  6,
			want:    "日本",
			wantLen: 6,
		},
		{
			name:    "multi-byte UTF-8 full content at exact byte length",
			input:   "日本語",
			maxLen:  9,
			want:    "日本語",
			wantLen: 9,
		},
		{
			name:    "multi-byte UTF-8 with excess maxLen",
			input:   "日本語",
			maxLen:  100,
			want:    "日本語",
			wantLen: 9,
		},
		{
			name:    "emoji truncation backs off from mid-rune",
			input:   "Hello 🌍 World",
			maxLen:  8,
			want:    "Hello ",
			wantLen: 6,
		},
		{
			name:    "emoji truncation at exact boundary after emoji",
			input:   "Hello 🌍 World",
			maxLen:  10,
			want:    "Hello 🌍",
			wantLen: 10,
		},
		{
			name:    "emoji at start truncated cleanly to empty",
			input:   "🌍hi",
			maxLen:  2,
			want:    "",
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateString(tc.input, tc.maxLen)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.wantLen, len(got))
		})
	}
}

func TestTruncateStringInvariants(t *testing.T) {
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
				got := TruncateString(in, maxLen)
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
