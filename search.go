package main

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	ddgHTMLSearchURL = "https://html.duckduckgo.com/html/"
	ddgDefaultUA     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	ddgTimeoutSec    = 15
	maxRetries       = 2
	retryBaseDelay   = 2 * time.Second
)

// SearchResult holds a single search result from DuckDuckGo.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// SearchClient performs web searches against the DuckDuckGo HTML endpoint.
type SearchClient struct {
	httpClient *http.Client
	userAgent  string
	baseURL    string
	maxRetries int
	retryDelay time.Duration
}

func newSearchClient() *SearchClient {
	return &SearchClient{
		httpClient: &http.Client{
			Timeout: ddgTimeoutSec * 1e9,
		},
		userAgent:  ddgDefaultUA,
		baseURL:    ddgHTMLSearchURL,
		maxRetries: maxRetries,
		retryDelay: retryBaseDelay,
	}
}

func (c *SearchClient) Search(ctx context.Context, query string) ([]SearchResult, error) {
	reqURL := c.baseURL + "?q=" + url.QueryEscape(query)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay * time.Duration(1<<(attempt-1))
			jitter := time.Duration(rand.Int64N(int64(delay) / 2))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay + jitter):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create search request: %w", err)
		}

		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Referer", "https://html.duckduckgo.com/")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("search request failed: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limited by DuckDuckGo (HTTP 429), please retry later")
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("DuckDuckGo returned HTTP %d: %s", resp.StatusCode, string(body))
		}

		return parseResults(resp.Body)
	}

	return nil, lastErr
}

func parseResults(body io.Reader) ([]SearchResult, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []SearchResult
	var inResult bool
	var inTitle bool
	var inSnippet bool
	var inURL bool
	var current SearchResult

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			classes := getAttr(n, "class")

			if isResultDiv(n, classes) {
				inResult = true
				current = SearchResult{}
			}

			if inResult {
				if hasClass(classes, "result__a") && n.Data == "a" {
					inTitle = true
					href := getAttr(n, "href")
					current.URL = extractRealURL(href)
				}
				if hasClass(classes, "result__snippet") && n.Data == "a" {
					inSnippet = true
				}
				if hasClass(classes, "result__url") && n.Data == "a" {
					inURL = true
				}
			}
		}

		if n.Type == html.TextNode {
			text := n.Data
			if inTitle {
				current.Title += text
			}
			if inSnippet {
				current.Snippet += text
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}

		if n.Type == html.ElementNode {
			classes := getAttr(n, "class")
			if inResult && isResultDiv(n, classes) {
				if current.Title != "" || current.URL != "" {
					current.Snippet = strings.TrimSpace(current.Snippet)
					current.Title = strings.TrimSpace(current.Title)
					results = append(results, current)
				}
				inResult = false
				current = SearchResult{}
			}
			if inTitle && hasClass(classes, "result__a") && n.Data == "a" {
				inTitle = false
			}
			if inSnippet && hasClass(classes, "result__snippet") && n.Data == "a" {
				inSnippet = false
			}
			if inURL && hasClass(classes, "result__url") && n.Data == "a" {
				inURL = false
			}
		}
	}
	f(doc)

	return results, nil
}

func isResultDiv(n *html.Node, classes string) bool {
	if n.Data != "div" {
		return false
	}
	return hasClass(classes, "result") && (hasClass(classes, "results_links") || hasClass(classes, "web-result"))
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func hasClass(classes, target string) bool {
	for _, c := range strings.Fields(classes) {
		if c == target {
			return true
		}
	}
	return false
}

func extractRealURL(href string) string {
	if href == "" {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	uddg := u.Query().Get("uddg")
	if uddg != "" {
		decoded, err := url.QueryUnescape(uddg)
		if err == nil {
			return decoded
		}
		return uddg
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	return href
}
