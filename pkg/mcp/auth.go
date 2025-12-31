package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/metal-toolbox/governor-api/internal/auth"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

const rawTokenKey = "raw-jwt-token"

func getToken(ti *mcpauth.TokenInfo) string {
	if ti == nil {
		return ""
	}

	if raw, ok := ti.Extra[rawTokenKey]; ok {
		if tokenStr, ok := raw.(string); ok {
			return tokenStr
		}
	}

	return ""
}

// verifyJWT verifies JWT tokens and returns TokenInfo for the auth middleware.
// This function implements the TokenVerifier interface required by auth.RequireBearerToken.
func (s *GovernorMCPServer) verifyJWT(ctx context.Context, tokenString string, _ *http.Request) (*mcpauth.TokenInfo, error) {
	// Parse token to check expiration
	parser := jwt.NewParser()

	token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidTokenClaims
	}

	exp, err := claims.GetExpirationTime()
	if err != nil {
		return nil, err
	}

	if exp != nil && exp.Before(time.Now()) {
		return nil, ErrTokenExpired
	}

	userInfo, err := auth.UserInfoFromJWT(ctx, tokenString, s.authConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to verify JWT: %w", err)
	}

	return &mcpauth.TokenInfo{
		UserID:     userInfo.Sub,
		Expiration: exp.Time,
		Extra:      map[string]any{rawTokenKey: tokenString},
	}, nil
}

func (s *GovernorMCPServer) authMiddleware() func(http.Handler) http.Handler {
	return mcpauth.RequireBearerToken(s.verifyJWT, &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: fmt.Sprintf("%s%s", s.metadataBaseURL, WellKnownOAuthProtectedResourcePath),
	})
}

func (s *GovernorMCPServer) authMetadataHandler(mux *http.ServeMux) {
	metadata := &oauthex.ProtectedResourceMetadata{
		Resource: fmt.Sprintf("%s/v1alpha1", s.metadataBaseURL),
	}

	authservers := []string{}

	for _, ac := range s.authConfigs {
		if ac.Enabled {
			authservers = append(authservers, ac.Issuer)
		}
	}

	metadata.AuthorizationServers = authservers

	mux.Handle(
		WellKnownOAuthProtectedResourcePath,
		mcpauth.ProtectedResourceMetadataHandler(metadata),
	)
}
