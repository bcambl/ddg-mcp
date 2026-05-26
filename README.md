# ddg-mcp

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server that provides web search capabilities via [DuckDuckGo](https://duckduckgo.com).

Exposes a `web_search` tool that MCP-compatible clients (Claude Desktop, Cursor, Windsurf, opencode, etc.) can use to search the web.

## Features

- Web search via DuckDuckGo HTML endpoint (no API key required)
- Returns titles, URLs, and snippets for ~10 results per query
- Extracts real URLs from DuckDuckGo redirect links
- Automatic retry with exponential backoff on rate limiting (HTTP 429)
- Query validation with length limits
- Stdio transport for MCP compatibility
- Cross-platform binaries via GoReleaser

## Installation

### Pre-built binaries

Download the latest release from the [releases page](https://github.com/bcambl/ddg-mcp/releases).

### Go install

```bash
go install github.com/bcambl/ddg-mcp@latest
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
      "args": ["run", "github.com/bcambl/ddg-mcp@latest"]
    }
  }
}
```

### Tool: `web_search`

**Input:**

| Parameter | Type   | Required | Description                          |
|-----------|--------|----------|--------------------------------------|
| `query`   | string | Yes      | The search query (max 500 characters) |

**Output:**

```json
{
  "query": "golang mcp",
  "result_count": 3,
  "results": [
    {
      "title": "Example Result",
      "url": "https://example.com/page",
      "snippet": "A brief description of the search result."
    }
  ]
}
```

**Error cases:**

- Empty query: `"query must not be empty"`
- Query too long: `"query exceeds maximum length of 500 characters"`
- Rate limited: `"rate limited by DuckDuckGo (HTTP 429), please retry later"`
- Server error: `"DuckDuckGo returned HTTP 500: ..."`

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

The server scrapes DuckDuckGo's HTML search endpoint (`html.duckduckgo.com/html/`) rather than using a formal API. This approach:

- Requires no API key
- Returns results similar to a browser search
- Is subject to DuckDuckGo's rate limiting

DuckDuckGo wraps result URLs in redirect links (e.g., `//duckduckgo.com/l/?uddg=<encoded-url>`). The server extracts the real URL from the `uddg` query parameter and falls back to the raw `href` for direct links.

## Limitations

- Results are limited to the first page (~10 results)
- Subject to DuckDuckGo rate limiting; avoid rapid successive queries
- HTML scraping may break if DuckDuckGo changes their page structure
- No support for image search, news search, or other DuckDuckGo features

## License

[MIT](LICENSE)
