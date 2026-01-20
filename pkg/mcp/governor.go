package mcp

import (
	"context"

	"github.com/metal-toolbox/governor-api/pkg/client"
	"golang.org/x/oauth2"
)

// governor client tokener
type gcTokener struct {
	rawToken string
}

func (t *gcTokener) Token(ctx context.Context) (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: t.rawToken,
		TokenType:   "Bearer",
	}, nil
}

func newGCTokener(rawToken string) client.Tokener {
	return &gcTokener{
		rawToken: rawToken,
	}
}

func (s *GovernorMCPServer) newGovernorClient(rawToken string) (*client.Client, error) {
	return client.NewClient(
		client.WithURL(s.govURL),
		client.WithTokener(newGCTokener(rawToken)),
		client.WithHTTPClient(s.httpclient),
		client.WithLogger(s.logger),
	)
}
