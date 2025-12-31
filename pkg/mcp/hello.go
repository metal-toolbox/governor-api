package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const tokenTruncateLen = 20

func SayHi(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
	tokeninfo := req.Extra.TokenInfo

	rawToken := getToken(tokeninfo)
	if rawToken == "" {
		return nil, nil, ErrNoTokenFound
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("hello, you're %s (token: %s...)", tokeninfo.UserID, truncate(rawToken, tokenTruncateLen))},
		},
	}, nil, nil
}

func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}

	return s[:length]
}
