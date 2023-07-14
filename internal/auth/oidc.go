// Package auth provides a simple wrapper around values needed for the openidconnect package
package auth

import (
	"context"
	"errors"

	"github.com/coreos/go-oidc/v3/oidc"
	"go.hollow.sh/toolbox/ginjwt"
	"golang.org/x/oauth2"
	"gopkg.in/square/go-jose.v2/jwt"
)

// OIDCProviderConfig is used to configure the openidconnect object from the openidconnect package
// github.com/markbates/goth/providers/openidConnect
type OIDCProviderConfig struct {
	OIDCClientKey    string
	OIDCClientSecret string
	OIDCCallbackURL  string
	OIDCDiscoveryURL string
	OIDCProviderName string
	OIDCScopes       []string
}

// OIDCUserInfo provides basic user info from OIDC
type OIDCUserInfo struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Sub   string `json:"sub"`
}

// UserInfoFromJWT tries to retrieve the user info (id token claims) using the given jwt token. It will
// use the oidc provider matching the issuer in the access token
func UserInfoFromJWT(ctx context.Context, rawToken string, oidcConfigs []ginjwt.AuthConfig) (*OIDCUserInfo, error) {
	var userInfo *oidc.UserInfo

	accessToken, err := jwt.ParseSigned(rawToken)
	if err != nil {
		return nil, err
	}

	tok := jwt.Claims{}
	if err := accessToken.UnsafeClaimsWithoutVerification(&tok); err != nil {
		return nil, err
	}

	for _, ac := range oidcConfigs {
		if ac.Issuer != tok.Issuer {
			continue
		}

		provider, err := oidc.NewProvider(ctx, ac.Issuer)
		if err != nil {
			return nil, err
		}

		userInfo, err = provider.UserInfo(ctx, oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: rawToken,
		}))
		if err != nil {
			return nil, err
		}
	}

	if userInfo == nil {
		return nil, errors.New("didn't find matching oidc issuer") //nolint:goerr113
	}

	userClaims := OIDCUserInfo{}
	if err := userInfo.Claims(&userClaims); err != nil {
		return nil, err
	}

	return &userClaims, nil
}
