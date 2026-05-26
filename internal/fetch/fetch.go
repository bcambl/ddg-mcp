package fetch

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/bcambl/ddg-mcp/internal/utils"
)

const (
	defaultMaxLen = 10000
	maxMaxLen     = 50000
)

type FetchResult struct {
	URL           string `json:"url"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	ContentLength int    `json:"content_length"`
	ContentType   string `json:"content_type"`
	Truncated     bool   `json:"truncated"`
}

type ClientOptions struct {
	HTTPClient  *http.Client
	UserAgent   string
	MaxBodySize int64
	CheckSSRF   bool
	Timeout     time.Duration
}

type Client struct {
	httpClient *http.Client
	userAgent  string
	maxBody    int64
	checkSSRF  bool
}

func NewClient(opts ClientOptions) *Client {
	checkSSRF := opts.CheckSSRF
	fc := &Client{
		userAgent: opts.UserAgent,
		maxBody:   opts.MaxBodySize,
		checkSSRF: checkSSRF,
	}
	if opts.HTTPClient != nil {
		fc.httpClient = opts.HTTPClient
	} else {
		fc.httpClient = &http.Client{
			Timeout: opts.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				if fc.checkSSRF {
					if err := utils.IsPrivateIP(req.URL.Hostname()); err != nil {
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
	}
	return fc
}

func (c *Client) Fetch(ctx context.Context, rawURL string, maxLength int) (*FetchResult, error) {
	if c.checkSSRF {
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		if err := utils.IsPrivateIP(parsedURL.Hostname()); err != nil {
			return nil, err
		}
	}
	if maxLength <= 0 {
		maxLength = defaultMaxLen
	}
	if maxLength > maxMaxLen {
		maxLength = maxMaxLen
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
		content = utils.TruncateString(content, maxLength)
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
