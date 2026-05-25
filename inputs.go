package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var errEmptyQuery = errors.New("query must not be empty")

const maxQueryLength = 500

type webSearchInput struct {
	Query string `json:"query" jsonschema:"the search query string"`
}

func (i *webSearchInput) validate() error {
	i.Query = strings.TrimSpace(i.Query)
	if i.Query == "" {
		return errEmptyQuery
	}
	if len(i.Query) > maxQueryLength {
		return fmt.Errorf("query exceeds maximum length of %d characters", maxQueryLength)
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
