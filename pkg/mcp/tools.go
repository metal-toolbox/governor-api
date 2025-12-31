package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *GovernorMCPServer) v1alpha1() *mcp.StreamableHTTPHandler {
	v1alpha1 := mcp.NewServer(&mcp.Implementation{Name: "governor-v1alpha1"}, nil)

	mcp.AddTool(v1alpha1, &mcp.Tool{Name: "hello", Description: "Hello"}, SayHi)

	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return v1alpha1
	}, nil)
}
