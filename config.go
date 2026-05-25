package main

import (
	"os"
	"strconv"
	"time"
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

func loadConfig() *Config {
	return &Config{
		SearchURL:      envOrDefault("DDG_SEARCH_URL", ddgLiteSearchURL),
		SearchTimeout:  envDuration("DDG_TIMEOUT", ddgTimeoutSec*time.Second),
		MaxRetries:     envInt("DDG_MAX_RETRIES", ddgDefaultMaxRetries),
		MaxBodySize:    envInt64("DDG_MAX_BODY_SIZE", ddgDefaultMaxBodySize),
		FetchUserAgent: envOrDefault("DDG_FETCH_USER_AGENT", ddgDefaultUA),
		FetchTimeout:   envDuration("DDG_FETCH_TIMEOUT", fetchDefaultTimeout),
		SSRFProtection: envBool("DDG_SSRF_PROTECTION", true),
		UserAgent:      envOrDefault("DDG_USER_AGENT", ddgDefaultUA),
		DefaultRegion:  envOrDefault("DDG_DEFAULT_REGION", ""),
		LogLevel:       envOrDefault("DDG_LOG_LEVEL", "info"),
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

func defaultTestConfig() *Config {
	return &Config{
		SearchURL:      ddgLiteSearchURL,
		SearchTimeout:  time.Duration(ddgTimeoutSec) * time.Second,
		MaxRetries:     ddgDefaultMaxRetries,
		MaxBodySize:    ddgDefaultMaxBodySize,
		FetchUserAgent: ddgDefaultUA,
		FetchTimeout:   fetchDefaultTimeout,
		SSRFProtection: true,
		UserAgent:      ddgDefaultUA,
		DefaultRegion:  "",
		LogLevel:       "info",
	}
}
