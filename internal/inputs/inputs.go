package inputs

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/bcambl/ddg-mcp/internal/utils"
)

var (
	ErrEmptyQuery        = errors.New("query must not be empty")
	ErrInvalidRegion     = errors.New("region must match format xx-xx (e.g., us-en, uk-en, de-de)")
	ErrInvalidSafeSearch = errors.New("safe_search must be one of: 0 (default), 1 (strict), -1 (moderate), -2 (off)")
	ErrInvalidTimeRange  = errors.New("time_range must be one of: d (day), w (week), m (month), y (year), or empty")
	ErrVqdRequired       = errors.New("vqd token is required for paginated requests (offset > 0)")
	ErrInvalidURL        = errors.New("url must be a valid HTTP or HTTPS URL")
	ErrPrivateIP         = utils.ErrPrivateIP
)

const MaxQueryLength = 500

type Search struct {
	Query      string `json:"query" jsonschema:"the search query string"`
	Region     string `json:"region,omitempty" jsonschema:"region code for search bias (e.g., us-en, uk-en, de-de, wt-wt for no region)"`
	SafeSearch int    `json:"safe_search,omitempty" jsonschema:"safe search level: 1=strict, -2=moderate, -2=off. Default: 0 (use DDG default)"`
	TimeRange  string `json:"time_range,omitempty" jsonschema:"time range filter: d=day, w=week, m=month, y=year. Empty=any time"`
	Offset     int    `json:"offset,omitempty" jsonschema:"result offset for pagination (0, 10, 20, etc.)"`
	Vqd        string `json:"vqd,omitempty" jsonschema:"pagination token from previous search response, required when offset > 0"`
}

func (i *Search) Validate() error {
	i.Query = strings.TrimSpace(i.Query)
	if i.Query == "" {
		return ErrEmptyQuery
	}
	if len(i.Query) > MaxQueryLength {
		return fmt.Errorf("query exceeds maximum length of %d characters", MaxQueryLength)
	}
	if i.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}
	if i.Region != "" && !isValidRegion(i.Region) {
		return ErrInvalidRegion
	}
	if !isValidSafeSearch(i.SafeSearch) {
		return ErrInvalidSafeSearch
	}
	if !isValidTimeRange(i.TimeRange) {
		return ErrInvalidTimeRange
	}
	if i.Offset > 0 && i.Vqd == "" {
		return ErrVqdRequired
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

type Fetch struct {
	URL       string `json:"url" jsonschema:"the URL to fetch and extract content from"`
	MaxLength int    `json:"max_length,omitempty" jsonschema:"maximum content length in characters (default 10000, max 50000)"`
}

func (i *Fetch) Validate() error {
	i.URL = strings.TrimSpace(i.URL)
	if i.URL == "" {
		return fmt.Errorf("url must not be empty")
	}
	parsedURL, err := url.Parse(i.URL)
	if err != nil {
		return ErrInvalidURL
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return ErrInvalidURL
	}
	if i.MaxLength == 0 {
		i.MaxLength = 10000
	}
	if i.MaxLength > 50000 {
		i.MaxLength = 50000
	}
	return nil
}
