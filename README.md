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

### Docker

```bash
docker pull ghcr.io/bcambl/ddg-mcp:latest
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

#### Using Docker

```json
{
  "mcpServers": {
    "ddg-search": {
      "command": "docker",
      "args": ["run", "--rm", "-i", "ghcr.io/bcambl/ddg-mcp"]
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

- Go 1.23+

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