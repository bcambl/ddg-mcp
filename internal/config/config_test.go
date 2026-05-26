package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Load()

	require.Equal(t, DefaultSearchURL, cfg.SearchURL)
	require.Equal(t, DefaultTimeoutSec*time.Second, cfg.SearchTimeout)
	require.Equal(t, DefaultMaxRetries, cfg.MaxRetries)
	require.Equal(t, int64(DefaultMaxBodySize), cfg.MaxBodySize)
	require.Equal(t, DefaultUserAgent, cfg.FetchUserAgent)
	require.Equal(t, DefaultFetchTimeout, cfg.FetchTimeout)
	require.Equal(t, DefaultSSRFProtection, cfg.SSRFProtection)
	require.Equal(t, DefaultUserAgent, cfg.UserAgent)
	require.Equal(t, DefaultRegion, cfg.DefaultRegion)
	require.Equal(t, DefaultLogLevel, cfg.LogLevel)
}

func TestDefaultTestConfig(t *testing.T) {
	cfg := DefaultTestConfig()
	require.NotNil(t, cfg)
	require.Equal(t, DefaultSearchURL, cfg.SearchURL)
	require.Equal(t, DefaultTimeoutSec*time.Second, cfg.SearchTimeout)
	require.Equal(t, DefaultMaxRetries, cfg.MaxRetries)
	require.Equal(t, int64(DefaultMaxBodySize), cfg.MaxBodySize)
	require.Equal(t, DefaultFetchTimeout, cfg.FetchTimeout)
	require.Equal(t, DefaultUserAgent, cfg.UserAgent)
	require.Equal(t, DefaultUserAgent, cfg.FetchUserAgent)
	require.True(t, cfg.SSRFProtection)
	require.Equal(t, "", cfg.DefaultRegion)
	require.Equal(t, "info", cfg.LogLevel)
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("DDG_LOG_LEVEL", "debug")
	t.Setenv("DDG_MAX_RETRIES", "5")
	t.Setenv("DDG_TIMEOUT", "30s")
	t.Setenv("DDG_SSRF_PROTECTION", "false")

	cfg := Load()
	require.Equal(t, "debug", cfg.LogLevel)
	require.Equal(t, 5, cfg.MaxRetries)
	require.Equal(t, 30*time.Second, cfg.SearchTimeout)
	require.False(t, cfg.SSRFProtection)
}

func TestEnvInvalidValues(t *testing.T) {
	t.Setenv("DDG_MAX_RETRIES", "not-a-number")
	t.Setenv("DDG_TIMEOUT", "invalid-duration")
	t.Setenv("DDG_MAX_BODY_SIZE", "not-a-number")
	t.Setenv("DDG_SSRF_PROTECTION", "not-a-bool")

	cfg := Load()
	require.Equal(t, DefaultMaxRetries, cfg.MaxRetries)
	require.Equal(t, DefaultTimeoutSec*time.Second, cfg.SearchTimeout)
	require.Equal(t, int64(DefaultMaxBodySize), cfg.MaxBodySize)
	require.Equal(t, DefaultSSRFProtection, cfg.SSRFProtection)
}
