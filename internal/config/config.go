package config

import (
	"os"
	"strconv"
	"time"
)

const (
	DefaultSearchURL      = "https://lite.duckduckgo.com/lite/"
	DefaultUserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	DefaultTimeoutSec     = 15
	DefaultMaxRetries     = 2
	DefaultMaxBodySize    = 2 * 1024 * 1024
	DefaultFetchTimeout   = 15 * time.Second
	DefaultSSRFProtection = true
	DefaultRegion         = ""
	DefaultLogLevel       = "info"
)

type Config struct {
	SearchURL      string
	SearchTimeout  time.Duration
	MaxRetries     int
	MaxBodySize    int64
	FetchUserAgent string
	FetchTimeout   time.Duration
	SSRFProtection bool
	UserAgent      string
	DefaultRegion  string
	LogLevel       string
}

func Load() *Config {
	return &Config{
		SearchURL:      envOrDefault("DDG_SEARCH_URL", DefaultSearchURL),
		SearchTimeout:  envDuration("DDG_TIMEOUT", DefaultTimeoutSec*time.Second),
		MaxRetries:     envInt("DDG_MAX_RETRIES", DefaultMaxRetries),
		MaxBodySize:    envInt64("DDG_MAX_BODY_SIZE", DefaultMaxBodySize),
		FetchUserAgent: envOrDefault("DDG_FETCH_USER_AGENT", DefaultUserAgent),
		FetchTimeout:   envDuration("DDG_FETCH_TIMEOUT", DefaultFetchTimeout),
		SSRFProtection: envBool("DDG_SSRF_PROTECTION", DefaultSSRFProtection),
		UserAgent:      envOrDefault("DDG_USER_AGENT", DefaultUserAgent),
		DefaultRegion:  envOrDefault("DDG_DEFAULT_REGION", DefaultRegion),
		LogLevel:       envOrDefault("DDG_LOG_LEVEL", DefaultLogLevel),
	}
}

func DefaultTestConfig() *Config {
	return &Config{
		SearchURL:      DefaultSearchURL,
		SearchTimeout:  DefaultTimeoutSec * time.Second,
		MaxRetries:     DefaultMaxRetries,
		MaxBodySize:    DefaultMaxBodySize,
		FetchUserAgent: DefaultUserAgent,
		FetchTimeout:   DefaultFetchTimeout,
		SSRFProtection: true,
		UserAgent:      DefaultUserAgent,
		DefaultRegion:  "",
		LogLevel:       "info",
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
