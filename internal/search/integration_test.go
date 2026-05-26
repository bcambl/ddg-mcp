//go:build integration

package search

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcambl/ddg-mcp/internal/config"
)

func TestIntegrationSearchRealDDG(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(ClientOptions{
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
		UserAgent:   config.DefaultUserAgent,
		BaseURL:     config.DefaultSearchURL,
		MaxRetries:  config.DefaultMaxRetries,
		MaxBodySize: config.DefaultMaxBodySize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := client.Search(ctx, "golang programming language")
	require.NoError(t, err)
	require.NotEmpty(t, results, "expected at least one result from DuckDuckGo")

	for _, r := range results {
		require.NotEmpty(t, r.Title, "result should have a title")
		require.NotEmpty(t, r.URL, "result should have a URL")
		t.Logf("Title: %s | URL: %s", r.Title, r.URL)
	}
}

func TestIntegrationSearchEmptyResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(ClientOptions{
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
		UserAgent:   config.DefaultUserAgent,
		BaseURL:     config.DefaultSearchURL,
		MaxRetries:  config.DefaultMaxRetries,
		MaxBodySize: config.DefaultMaxBodySize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := client.Search(ctx, "xzysq987654321nonexistent")
	require.NoError(t, err)
	require.Empty(t, results, "expected no results for nonsense query")
}

func TestIntegrationSearchSpecialCharacters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := NewClient(ClientOptions{
		HTTPClient:  &http.Client{Timeout: 30 * time.Second},
		UserAgent:   config.DefaultUserAgent,
		BaseURL:     config.DefaultSearchURL,
		MaxRetries:  config.DefaultMaxRetries,
		MaxBodySize: config.DefaultMaxBodySize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := client.Search(ctx, "c++ & java \"quotes\"")
	require.NoError(t, err)
	require.NotEmpty(t, results, "expected results for query with special characters")
}
