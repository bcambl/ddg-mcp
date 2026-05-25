package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	errEmptyQuery        = errors.New("query must not be empty")
	errInvalidRegion     = errors.New("region must match format xx-xx (e.g., us-en, uk-en, de-de)")
	errInvalidSafeSearch = errors.New("safe_search must be one of: 0 (default), 1 (strict), -1 (moderate), -2 (off)")
	errInvalidTimeRange  = errors.New("time_range must be one of: d (day), w (week), m (month), y (year), or empty")
	errVqdRequired       = errors.New("vqd token is required for paginated requests (offset > 0)")
	errInvalidURL        = errors.New("url must be a valid HTTP or HTTPS URL")
	errPrivateIP         = errors.New("url must not point to a private or reserved IP address")
)

const maxQueryLength = 500

type webSearchInput struct {
	Query      string `json:"query" jsonschema:"the search query string"`
	Region     string `json:"region,omitempty" jsonschema:"region code for search bias (e.g., us-en, uk-en, de-de, wt-wt for no region)"`
	SafeSearch int    `json:"safe_search,omitempty" jsonschema:"safe search level: 1=strict, -1=moderate, -2=off. Default: 0 (use DDG default)"`
	TimeRange  string `json:"time_range,omitempty" jsonschema:"time range filter: d=day, w=week, m=month, y=year. Empty=any time"`
	Offset     int    `json:"offset,omitempty" jsonschema:"result offset for pagination (0, 10, 20, etc.)"`
	Vqd        string `json:"vqd,omitempty" jsonschema:"pagination token from previous search response, required when offset > 0"`
}

func (i *webSearchInput) validate() error {
	i.Query = strings.TrimSpace(i.Query)
	if i.Query == "" {
		return errEmptyQuery
	}
	if len(i.Query) > maxQueryLength {
		return fmt.Errorf("query exceeds maximum length of %d characters", maxQueryLength)
	}
	if i.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}
	if i.Region != "" && !isValidRegion(i.Region) {
		return errInvalidRegion
	}
	if !isValidSafeSearch(i.SafeSearch) {
		return errInvalidSafeSearch
	}
	if !isValidTimeRange(i.TimeRange) {
		return errInvalidTimeRange
	}
	if i.Offset > 0 && i.Vqd == "" {
		return errVqdRequired
	}
	return nil
}

func isValidRegion(region string) bool {
	if region == "" {
		return true
	}
	parts := strings.Split(region, "-")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if len(p) < 2 || len(p) > 3 {
			return false
		}
		for _, c := range p {
			if c < 'a' || c > 'z' {
				return false
			}
		}
	}
	return true
}

func isValidSafeSearch(val int) bool {
	switch val {
	case 0, 1, -1, -2:
		return true
	default:
		return false
	}
}

func isValidTimeRange(val string) bool {
	switch val {
	case "", "d", "w", "m", "y":
		return true
	default:
		return false
	}
}

type webFetchInput struct {
	URL       string `json:"url" jsonschema:"the URL to fetch and extract content from"`
	MaxLength int    `json:"max_length,omitempty" jsonschema:"maximum content length in characters (default 10000, max 50000)"`
}

func (i *webFetchInput) validate() error {
	i.URL = strings.TrimSpace(i.URL)
	if i.URL == "" {
		return fmt.Errorf("url must not be empty")
	}
	parsedURL, err := url.Parse(i.URL)
	if err != nil {
		return errInvalidURL
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errInvalidURL
	}
	if i.MaxLength == 0 {
		i.MaxLength = 10000
	}
	if i.MaxLength > 50000 {
		i.MaxLength = 50000
	}
	return nil
}

func isPrivateIP(hostname string) error {
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return errPrivateIP
	}
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %w", hostname, err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return errPrivateIP
		}
		if ip4 := ip.To4(); ip4 != nil && ip4[0] == 0 {
			return errPrivateIP
		}
	}
	return nil
}

func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Error: %s", err.Error())},
		},
		IsError: true,
	}
}

func successResult(result any) *mcp.CallToolResult {
	resultBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("%v", result)},
			},
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultBytes)},
		},
	}
}
