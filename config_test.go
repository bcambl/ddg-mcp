package main

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("DDG_TEST_KEY", "override")
	require.Equal(t, "override", envOrDefault("DDG_TEST_KEY", "fallback"))

	// Unset returns fallback.
	require.Equal(t, "fallback", envOrDefault("DDG_TEST_KEY_UNSET_12345", "fallback"))
}

func TestEnvDuration(t *testing.T) {
	t.Setenv("DDG_TEST_DUR", "30s")
	require.Equal(t, 30*time.Second, envDuration("DDG_TEST_DUR", 5*time.Second))

	// Empty returns fallback.
	require.Equal(t, 5*time.Second, envDuration("DDG_TEST_DUR_UNSET_12345", 5*time.Second))

	// Invalid returns fallback.
	t.Setenv("DDG_TEST_DUR_BAD", "invalid")
	require.Equal(t, 5*time.Second, envDuration("DDG_TEST_DUR_BAD", 5*time.Second))
}

func TestEnvInt(t *testing.T) {
	t.Setenv("DDG_TEST_INT", "42")
	require.Equal(t, 42, envInt("DDG_TEST_INT", 0))

	require.Equal(t, 7, envInt("DDG_TEST_INT_UNSET_12345", 7))

	t.Setenv("DDG_TEST_INT_BAD", "abc")
	require.Equal(t, 7, envInt("DDG_TEST_INT_BAD", 7))
}

func TestEnvInt64(t *testing.T) {
	t.Setenv("DDG_TEST_INT64", "9999999999")
	require.Equal(t, int64(9999999999), envInt64("DDG_TEST_INT64", 0))

	require.Equal(t, int64(100), envInt64("DDG_TEST_INT64_UNSET_12345", 100))

	t.Setenv("DDG_TEST_INT64_BAD", "foo")
	require.Equal(t, int64(100), envInt64("DDG_TEST_INT64_BAD", 100))
}

func TestEnvBool(t *testing.T) {
	t.Setenv("DDG_TEST_BOOL", "false")
	require.False(t, envBool("DDG_TEST_BOOL", true))

	require.True(t, envBool("DDG_TEST_BOOL_UNSET_12345", true))

	t.Setenv("DDG_TEST_BOOL_BAD", "maybe")
	require.True(t, envBool("DDG_TEST_BOOL_BAD", true))

	t.Setenv("DDG_TEST_BOOL_1", "1")
	require.True(t, envBool("DDG_TEST_BOOL_1", false))

	t.Setenv("DDG_TEST_BOOL_0", "0")
	require.False(t, envBool("DDG_TEST_BOOL_0", true))
}

func TestLoadConfigDefaults(t *testing.T) {
	// Ensure no conflicting env vars are set.
	for _, k := range []string{
		"DDG_SEARCH_URL", "DDG_TIMEOUT", "DDG_MAX_RETRIES", "DDG_MAX_BODY_SIZE",
		"DDG_FETCH_USER_AGENT", "DDG_FETCH_TIMEOUT", "DDG_SSRF_PROTECTION",
		"DDG_USER_AGENT", "DDG_DEFAULT_REGION", "DDG_LOG_LEVEL",
	} {
		os.Unsetenv(k)
	}

	cfg := loadConfig()
	require.Equal(t, ddgLiteSearchURL, cfg.SearchURL)
	require.Equal(t, time.Duration(ddgTimeoutSec)*time.Second, cfg.SearchTimeout)
	require.Equal(t, ddgDefaultMaxRetries, cfg.MaxRetries)
	require.Equal(t, int64(ddgDefaultMaxBodySize), cfg.MaxBodySize)
	require.Equal(t, ddgDefaultUA, cfg.FetchUserAgent)
	require.Equal(t, fetchDefaultTimeout, cfg.FetchTimeout)
	require.True(t, cfg.SSRFProtection)
	require.Equal(t, ddgDefaultUA, cfg.UserAgent)
	require.Empty(t, cfg.DefaultRegion)
	require.Equal(t, "info", cfg.LogLevel)
}

func TestLoadConfigOverrides(t *testing.T) {
	t.Setenv("DDG_SEARCH_URL", "http://custom/search")
	t.Setenv("DDG_TIMEOUT", "45s")
	t.Setenv("DDG_MAX_RETRIES", "5")
	t.Setenv("DDG_MAX_BODY_SIZE", "1048576")
	t.Setenv("DDG_FETCH_USER_AGENT", "CustomBot")
	t.Setenv("DDG_FETCH_TIMEOUT", "25s")
	t.Setenv("DDG_SSRF_PROTECTION", "false")
	t.Setenv("DDG_USER_AGENT", "CustomSearchBot")
	t.Setenv("DDG_DEFAULT_REGION", "de-de")
	t.Setenv("DDG_LOG_LEVEL", "debug")

	cfg := loadConfig()
	require.Equal(t, "http://custom/search", cfg.SearchURL)
	require.Equal(t, 45*time.Second, cfg.SearchTimeout)
	require.Equal(t, 5, cfg.MaxRetries)
	require.Equal(t, int64(1048576), cfg.MaxBodySize)
	require.Equal(t, "CustomBot", cfg.FetchUserAgent)
	require.Equal(t, 25*time.Second, cfg.FetchTimeout)
	require.False(t, cfg.SSRFProtection)
	require.Equal(t, "CustomSearchBot", cfg.UserAgent)
	require.Equal(t, "de-de", cfg.DefaultRegion)
	require.Equal(t, "debug", cfg.LogLevel)
}

func TestParseLogLevel(t *testing.T) {
	require.Equal(t, slog.LevelDebug, parseLogLevel("debug"))
	require.Equal(t, slog.LevelInfo, parseLogLevel("info"))
	require.Equal(t, slog.LevelWarn, parseLogLevel("warn"))
	require.Equal(t, slog.LevelError, parseLogLevel("error"))
	require.Equal(t, slog.LevelInfo, parseLogLevel("unknown"))
	require.Equal(t, slog.LevelInfo, parseLogLevel(""))
}
