package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const (
	fetchDefaultTimeout = 15 * time.Second
	fetchMaxBodySize    = 2 * 1024 * 1024
	fetchDefaultMaxLen  = 10000
	fetchMaxMaxLen      = 50000
	fetchUserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

type FetchResult struct {
	URL           string `json:"url"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	ContentLength int    `json:"content_length"`
	ContentType   string `json:"content_type"`
	Truncated     bool   `json:"truncated"`
}

type FetchClient struct {
	httpClient *http.Client
	userAgent  string
	maxBody    int64
	checkSSRF  bool
}

func newFetchClient() *FetchClient {
	fc := &FetchClient{
		userAgent: fetchUserAgent,
		maxBody:   fetchMaxBodySize,
		checkSSRF: true,
	}
	fc.httpClient = &http.Client{
		Timeout: fetchDefaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if fc.checkSSRF {
				if err := isPrivateIP(req.URL.Hostname()); err != nil {
					return fmt.Errorf("redirect to private IP blocked: %w", err)
				}
			}
			return nil
		},
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}
	return fc
}

func (c *FetchClient) Fetch(ctx context.Context, rawURL string, maxLength int) (*FetchResult, error) {
	if c.checkSSRF {
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		if err := isPrivateIP(parsedURL.Hostname()); err != nil {
			return nil, err
		}
	}
	if maxLength <= 0 {
		maxLength = fetchDefaultMaxLen
	}
	if maxLength > fetchMaxMaxLen {
		maxLength = fetchMaxMaxLen
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.5")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch returned HTTP %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !isAllowedContentType(contentType) {
		return nil, fmt.Errorf("unsupported content type: %s (only text/html and text/plain are supported)", contentType)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	title, content := extractContent(string(body), contentType)

	truncated := false
	if len(content) > maxLength {
		content = truncateString(content, maxLength)
		truncated = true
	}

	content = strings.TrimSpace(content)

	return &FetchResult{
		URL:           rawURL,
		Title:         strings.TrimSpace(title),
		Content:       content,
		ContentLength: len(content),
		ContentType:   contentType,
		Truncated:     truncated,
	}, nil
}

func isAllowedContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "text/plain") || strings.Contains(ct, "application/xhtml")
}

func extractContent(htmlContent, contentType string) (string, string) {
	if strings.Contains(strings.ToLower(contentType), "text/plain") {
		return "", htmlContent
	}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", htmlContent
	}

	var title string
	var bodyContent strings.Builder
	removeTags := map[string]bool{
		"script": true,
		"style":  true,
		"nav":    true,
		"footer": true,
		"header": true,
	}

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && removeTags[n.Data] {
			return
		}

		if n.Type == html.ElementNode && n.Data == "title" {
			title = collectText(n)
			return
		}

		if n.Type == html.TextNode {
			text := n.Data
			if text != "" {
				bodyContent.WriteString(text)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}

		if n.Type == html.ElementNode {
			switch n.Data {
			case "br", "p", "div", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6":
				bodyContent.WriteString("\n")
			}
		}
	}
	f(doc)

	content := bodyContent.String()
	content = collapseWhitespace(content)

	return title, content
}

func collapseWhitespace(s string) string {
	var b strings.Builder
	prevNewline := false
	prevSpace := false

	for _, r := range s {
		if r == '\n' {
			if prevNewline {
				continue
			}
			b.WriteRune('\n')
			prevNewline = true
			prevSpace = false
		} else if r == ' ' || r == '\t' {
			if prevNewline || prevSpace {
				continue
			}
			b.WriteRune(' ')
			prevSpace = true
			prevNewline = false
		} else {
			b.WriteRune(r)
			prevNewline = false
			prevSpace = false
		}
	}

	return strings.TrimSpace(b.String())
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	for maxLen > 0 && !utf8.RuneStart(s[maxLen]) {
		maxLen--
	}
	return s[:maxLen]
}
