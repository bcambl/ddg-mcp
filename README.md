# ddg-mcp

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server that provides web search and web fetch capabilities via [DuckDuckGo](https://duckduckgo.com).

Exposes `web_search` and `web_fetch` tools that MCP-compatible clients (Claude Desktop, Cursor, Windsurf, opencode, etc.) can use to search the web and retrieve page content.

## Features

- **Web search** via DuckDuckGo Lite endpoint (no API key required)
  - Titles, URLs, snippets, and domain info per result
  - Zero-click / instant answer extraction
  - Pagination via offset/vqd tokens
  - Region bias (`kl`), safe search (`kp`), and time range (`df`) filters
  - Sponsored result detection
  - Parser breakage detection with warning when HTML structure changes
  - Automatic retry with exponential backoff + jitter on rate limiting (HTTP 429)
- **Web fetch** — retrieve and extract readable content from URLs
  - Strips scripts, styles, nav, header, footer elements
  - Content truncation with configurable max length
  - SSRF protection (blocks redirects to private/reserved IPs)
- Query validation with length limits
- Stdio transport for MCP compatibility
- Cross-platform binaries via GoReleaser

## Installation

### Pre-built binaries

Download the latest release from the [releases page](https://github.com/bcambl/ddg-mcp/releases).

### Go install

```bash
go install github.com/bcambl/ddg-mcp/cmd/ddg-mcp@latest
```

### Build from source

```bash
git clone https://github.com/bcambl/ddg-mcp.git
cd ddg-mcp
make build
```

## Usage

### Running the server

```bash
ddg-mcp
```

The server communicates over stdio using the MCP protocol. It is designed to be launched by MCP-compatible clients.

### Environment Variables

The following environment variables can be used to customize server behavior without modifying the binary or client configuration:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DDG_SEARCH_URL` | string | `https://lite.duckduckgo.com/lite/` | DuckDuckGo search endpoint URL |
| `DDG_TIMEOUT` | duration | `15s` | HTTP timeout for search requests |
| `DDG_FETCH_TIMEOUT` | duration | `15s` | HTTP timeout for fetch requests |
| `DDG_MAX_RETRIES` | int | `2` | Max retries on HTTP 429 (rate limited) for search |
| `DDG_MAX_BODY_SIZE` | int (bytes) | `2097152` (2 MiB) | Maximum response body size for search and fetch |
| `DDG_USER_AGENT` | string | Chrome 120 UA | User-Agent for search requests |
| `DDG_FETCH_USER_AGENT` | string | Chrome 120 UA | User-Agent for fetch requests |
| `DDG_DEFAULT_REGION` | string | *(none)* | Default region bias (e.g., `us-en`, `uk-en`, `de-de`) when not specified per-request |
| `DDG_SSRF_PROTECTION` | bool | `true` | Enable SSRF protection for `web_fetch` (redirects to private IPs blocked) |
| `DDG_LOG_LEVEL` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |

Duration values accept Go-style strings: `15s`, `1m`, `500ms`.

Boolean values accept: `true`, `false`, `1`, `0`.

**Example with Go install:**

```bash
DDG_DEFAULT_REGION=de-de DDG_TIMEOUT=30s ddg-mcp
```

### MCP client configuration

Add the server to your MCP client configuration.

#### Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "ddg-search": {
      "command": "ddg-mcp"
    }
  }
}
```

#### Cursor

Add to `.cursor/mcp.json` in your project or global config:

```json
{
  "mcpServers": {
    "ddg-search": {
      "command": "ddg-mcp"
    }
  }
}
```

#### Windsurf

Add to your Windsurf MCP settings:

```json
{
  "mcpServers": {
    "ddg-search": {
      "command": "ddg-mcp"
    }
  }
}
```

#### opencode

Add to your `opencode.json`:

```json
{
  "mcp": {
    "ddg-search": {
      "command": "ddg-mcp"
    }
  }
}
```

#### Using Go install path

If installed via `go install`, use the full path:

```json
{
  "mcpServers": {
    "ddg-search": {
      "command": "go",
      "args": ["run", "github.com/bcambl/ddg-mcp/cmd/ddg-mcp@latest"]
    }
  }
}
```

### Tool: `web_search`

Search the web using DuckDuckGo. Returns search results with titles, URLs, snippets, and domain info. Supports pagination, region bias, safe search, and time range filters. Zero-click / instant answer results are included when available.

**Input:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | The search query (max 500 characters) |
| `region` | string | No | Region code for search bias (e.g., `us-en`, `uk-en`, `de-de`, `wt-wt` for no region) |
| `safe_search` | int | No | Safe search level: `1`=strict, `-1`=moderate, `-2`=off. Default: `0` (DuckDuckGo default) |
| `time_range` | string | No | Time range filter: `d`=day, `w`=week, `m`=month, `y`=year. Empty=any time |
| `offset` | int | No | Result offset for pagination (0, 10, 20, etc.) |
| `vqd` | string | No | Pagination token from previous search response, required when `offset > 0` |

**Output:**

```json
{
  "query": "golang mcp",
  "result_count": 3,
  "results": [
    {
      "title": "Example Result",
      "url": "https://example.com/page",
      "snippet": "A brief description of the search result.",
      "domain": "example.com",
      "sponsored": false
    }
  ],
  "zero_click": {
    "title": "Instant Answer Title",
    "url": "https://example.com/answer",
    "description": "Brief answer from DuckDuckGo's instant answers."
  },
  "has_next_page": true,
  "next_offset": 10,
  "vqd": "3-123-456",
  "parser_warning": ""
}
```

**Pagination:**

To get the next page of results, pass `offset` and `vqd` from the previous response:

```json
{
  "query": "golang mcp",
  "offset": 10,
  "vqd": "3-123-456"
}
```

**Error cases:**

- Empty query: `"query must not be empty"`
- Query too long: `"query exceeds maximum length of 500 characters"`
- Invalid region: `"region must match format xx-xx (e.g., us-en, uk-en, de-de)"`
- Invalid safe_search: `"safe_search must be one of: 0 (default), 1 (strict), -1 (moderate), -2 (off)"`
- Invalid time_range: `"time_range must be one of: d (day), w (week), m (month), y (year), or empty"`
- Paginated request without vqd: `"vqd token is required for paginated requests (offset > 0)"`
- Rate limited: `"rate limited by DuckDuckGo (HTTP 429), please retry later"`
- Server error: `"DuckDuckGo returned HTTP 500: ..."`
- Parser breakage: `"parser may be broken: HTML contains result markers but no results were extracted"`

### Tool: `web_fetch`

Fetch and extract readable content from a URL. Returns the page title and text content with scripts, styles, navigation, header, and footer elements removed. Only `text/html` and `text/plain` content types are supported.

**Input:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | The URL to fetch (must be HTTP or HTTPS) |
| `max_length` | int | No | Maximum content length in characters (default 10000, max 50000) |

**Output:**

```json
{
  "url": "https://example.com/page",
  "title": "Example Page Title",
  "content": "Extracted readable text content from the page...",
  "content_length": 8234,
  "content_type": "text/html; charset=utf-8",
  "truncated": false
}
```

**Error cases:**

- Empty URL: `"url must not be empty"`
- Invalid URL: `"url must be a valid HTTP or HTTPS URL"`
- Private IP (SSRF protection enabled): `"url must not point to a private or reserved IP address"`
- Unsupported content type: `"unsupported content type: application/pdf (only text/html and text/plain are supported)"`
- Server error: `"fetch returned HTTP 404"`

## Development

### Prerequisites

- Go 1.26+

### Build

```bash
make build
```

### Test

```bash
make test
```

### Run all checks

```bash
make check
```

### Integration tests

Integration tests hit the real DuckDuckGo endpoint and require a build tag:

```bash
go test -tags=integration -v ./...
```

## How it works

The server scrapes DuckDuckGo's Lite search endpoint (`lite.duckduckgo.com/lite/`) rather than using a formal API. This approach:

- Requires no API key
- Returns results similar to a browser search
- Is subject to DuckDuckGo's rate limiting

### Search

DuckDuckGo wraps result URLs in redirect links (e.g., `//duckduckgo.com/l/?uddg=<encoded-url>`). The server extracts the real URL from the `uddg` query parameter and falls back to the raw `href` for direct links.

Pagination works by POSTing form data (including a `vqd` token and offset) back to the Lite endpoint when `offset > 0`. The `vqd` token identifies the search session and is extracted from a hidden input in the results page.

The parser detects potential breakage — if the response HTML contains result markers (CSS classes, redirect link patterns) but no results were extracted, a `parser_warning` is included in the response.

### Fetch

The fetch tool makes a GET request to the provided URL, strips unwanted HTML elements (scripts, styles, nav, header, footer), and returns the readable text content. It enforces a configurable max content length and blocks requests to private/reserved IPs when SSRF protection is enabled (default).

## Limitations

- Search results are limited to the first page per request (~10 results); pagination requires explicit offset/vqd tokens
- Subject to DuckDuckGo rate limiting; avoid rapid successive queries
- HTML scraping may break if DuckDuckGo changes their page structure (a `parser_warning` is included when detected)
- No support for image search, news search, or other DuckDuckGo features
- Fetch only supports `text/html` and `text/plain` content types

## License

[MIT](LICENSE)