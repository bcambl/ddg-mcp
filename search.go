package main

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	ddgLiteSearchURL = "https://lite.duckduckgo.com/lite/"
	ddgDefaultUA     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	ddgTimeoutSec    = 15
	maxRetries       = 2
	retryBaseDelay   = 2 * time.Second
	maxResponseBody  = 2 * 1024 * 1024
)

type SearchResult struct {
	Title     string `json:"title"`
	URL       string `json:"url"`
	Snippet   string `json:"snippet"`
	Domain    string `json:"domain,omitempty"`
	Sponsored bool   `json:"sponsored,omitempty"`
}

type SearchResponse struct {
	Query         string           `json:"query"`
	ResultCount   int              `json:"result_count"`
	Results       []SearchResult   `json:"results"`
	ZeroClick     *ZeroClickResult `json:"zero_click,omitempty"`
	HasNextPage   bool             `json:"has_next_page"`
	NextOffset    int              `json:"next_offset,omitempty"`
	Vqd           string           `json:"vqd,omitempty"`
	ParserWarning string           `json:"parser_warning,omitempty"`
}

type ZeroClickResult struct {
	Title       string `json:"title,omitempty"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

type SearchOptions struct {
	Region     string
	SafeSearch int
	TimeRange  string
}

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
			Timeout: time.Duration(ddgTimeoutSec) * time.Second,
		},
		userAgent:  envOrDefault("DDG_USER_AGENT", ddgDefaultUA),
		baseURL:    ddgLiteSearchURL,
		maxRetries: maxRetries,
		retryDelay: retryBaseDelay,
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (c *SearchClient) Search(ctx context.Context, query string, opts SearchOptions) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("q", query)
	if opts.Region != "" {
		params.Set("kl", opts.Region)
	}
	if opts.SafeSearch != 0 {
		params.Set("kp", strconv.Itoa(opts.SafeSearch))
	}
	if opts.TimeRange != "" {
		params.Set("df", opts.TimeRange)
	}

	reqURL := c.baseURL + "?" + params.Encode()

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
		req.Header.Set("Referer", "https://lite.duckduckgo.com/")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("search request failed: %w", err)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limited by DuckDuckGo (HTTP 429), please retry later")
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("DuckDuckGo returned HTTP %d", resp.StatusCode)
		}

		results, zeroClick, _ := parseLiteResults(string(body))
		hasNextPage, nextOffset, vqd := parsePaginationInfo(string(body))
		warning := detectBreakage(string(body), len(results))

		searchResp := &SearchResponse{
			Query:         query,
			ResultCount:   len(results),
			Results:       results,
			ZeroClick:     zeroClick,
			HasNextPage:   hasNextPage,
			NextOffset:    nextOffset,
			Vqd:           vqd,
			ParserWarning: warning,
		}

		return searchResp, nil
	}

	return nil, lastErr
}

func (c *SearchClient) SearchWithOffset(ctx context.Context, query string, offset int, vqd string, opts SearchOptions) (*SearchResponse, error) {
	formData := url.Values{}
	formData.Set("q", query)
	formData.Set("s", strconv.Itoa(offset))
	formData.Set("dc", strconv.Itoa(offset+1))
	formData.Set("vqd", vqd)
	formData.Set("v", "l")
	formData.Set("o", "json")
	formData.Set("api", "d.js")
	if opts.Region != "" {
		formData.Set("kl", opts.Region)
	}
	if opts.SafeSearch != 0 {
		formData.Set("kp", strconv.Itoa(opts.SafeSearch))
	}
	if opts.TimeRange != "" {
		formData.Set("df", opts.TimeRange)
	}

	encoded := formData.Encode()

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

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, strings.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("failed to create search request: %w", err)
		}

		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Referer", "https://lite.duckduckgo.com/")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("search request failed: %w", err)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limited by DuckDuckGo (HTTP 429), please retry later")
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("DuckDuckGo returned HTTP %d", resp.StatusCode)
		}

		results, zeroClick, _ := parseLiteResults(string(body))
		hasNextPage, nextOffset, newVqd := parsePaginationInfo(string(body))
		warning := detectBreakage(string(body), len(results))

		searchResp := &SearchResponse{
			Query:         query,
			ResultCount:   len(results),
			Results:       results,
			ZeroClick:     zeroClick,
			HasNextPage:   hasNextPage,
			NextOffset:    nextOffset,
			Vqd:           newVqd,
			ParserWarning: warning,
		}

		return searchResp, nil
	}

	return nil, lastErr
}

func parseLiteResults(htmlContent string) ([]SearchResult, *ZeroClickResult, string) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, nil, htmlContent
	}

	var results []SearchResult
	var zeroClick *ZeroClickResult
	var current SearchResult
	var hasResult bool
	var inSnippet bool
	var inLinkText bool

	commitResult := func() {
		if hasResult && (current.Title != "" || current.URL != "") {
			current.Snippet = strings.TrimSpace(current.Snippet)
			current.Title = strings.TrimSpace(current.Title)
			current.Domain = strings.TrimSpace(current.Domain)
			results = append(results, current)
		}
		current = SearchResult{}
		hasResult = false
		inSnippet = false
		inLinkText = false
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			classes := getAttr(n, "class")

			if n.Data == "a" && hasClass(classes, "result-link") {
				commitResult()
				hasResult = true
				current = SearchResult{}
				href := getAttr(n, "href")
				current.URL = extractRealURL(href)
				titleText := collectText(n)
				current.Title = strings.TrimSpace(titleText)
				if strings.Contains(current.Title, "(Sponsored link") {
					current.Sponsored = true
					current.Title = strings.TrimSpace(strings.Split(current.Title, "(Sponsored link")[0])
				}
			}

			if n.Data == "td" && hasClass(classes, "result-snippet") {
				inSnippet = true
			}

			if n.Data == "span" && hasClass(classes, "link-text") {
				inLinkText = true
			}

			if n.Data == "td" && strings.Contains(getTextContent(n), "Zero-click info:") {
				zeroClick = &ZeroClickResult{}
				links := collectLinks(n)
				if len(links) > 0 {
					zeroClick.Title = links[0].text
					zeroClick.URL = links[0].href
				}
			}
		}

		if n.Type == html.TextNode {
			text := n.Data
			if inSnippet {
				current.Snippet += text
			}
			if inLinkText {
				current.Domain += text
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}

		if n.Type == html.ElementNode && n.Data == "td" && hasClass(getAttr(n, "class"), "result-snippet") {
			inSnippet = false
		}

		if n.Type == html.ElementNode && n.Data == "span" && hasClass(getAttr(n, "class"), "link-text") {
			inLinkText = false
		}
	}
	f(doc)

	commitResult()

	return results, zeroClick, htmlContent
}

type linkInfo struct {
	text string
	href string
}

func collectLinks(n *html.Node) []linkInfo {
	var links []linkInfo
	var f func(*html.Node)
	f = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			href := getAttr(node, "href")
			text := strings.TrimSpace(collectText(node))
			if text != "" {
				links = append(links, linkInfo{text: text, href: extractRealURL(href)})
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return links
}

func collectText(n *html.Node) string {
	var b strings.Builder
	var f func(*html.Node)
	f = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return b.String()
}

func getTextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(getTextContent(c))
	}
	return b.String()
}

func parsePaginationInfo(htmlContent string) (hasNextPage bool, nextOffset int, vqd string) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return false, 0, ""
	}

	var findForm func(*html.Node)
	findForm = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "form" {
			classes := getAttr(n, "class")
			if hasClass(classes, "next_form") {
				hasNextPage = true
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "input" {
						name := getAttr(c, "name")
						value := getAttr(c, "value")
						switch name {
						case "s":
							nextOffset, _ = strconv.Atoi(value)
						case "vqd":
							vqd = value
						}
					}
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findForm(c)
		}
	}
	findForm(doc)

	return hasNextPage, nextOffset, vqd
}

func detectBreakage(htmlContent string, resultCount int) string {
	if resultCount > 0 {
		return ""
	}
	markers := []string{
		`class="result-link"`,
		`class="link-text"`,
		`class="result-snippet"`,
		`duckduckgo.com/l/?`,
	}
	for _, marker := range markers {
		if strings.Contains(htmlContent, marker) {
			return "parser may be broken: HTML contains result markers but no results were extracted"
		}
	}
	return ""
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
		if err != nil {
			return uddg
		}
		return decoded
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	return href
}
