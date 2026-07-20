package auth

import (
	"testing"

	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/metal-toolbox/governor-api/pkg/configs"
)

func TestVerifierFromAuth(t *testing.T) {
	// A disabled OIDC config lets NewAuthMiddleware skip JWKS fetching, so we
	// can exercise the plain-vs-cedar selection without a JWKS server.
	base := ginjwt.AuthConfig{Enabled: false, Audience: "aud", Issuer: "iss"}

	t.Run("no cedar returns plain ginjwt middleware", func(t *testing.T) {
		mw, err := verifierFromAuth(configs.Auth{AuthConfig: base}, nil, nil)
		require.NoError(t, err)

		_, ok := mw.(*ginjwt.Middleware)
		assert.True(t, ok, "expected a *ginjwt.Middleware")
	})

	t.Run("cedar enabled returns the cedar verifier", func(t *testing.T) {
		a := configs.Auth{AuthConfig: base, Cedar: configs.CedarConfig{Enabled: true, URL: "http://127.0.0.1:8180"}}

		mw, err := verifierFromAuth(a, nil, nil)
		require.NoError(t, err)

		_, ok := mw.(*ginjwt.Middleware)
		assert.False(t, ok, "expected the cedar verifier, not a *ginjwt.Middleware")
	})
}

func TestMultiTokenMiddlewareFromConfigs(t *testing.T) {
	base := ginjwt.AuthConfig{Enabled: false, Audience: "aud", Issuer: "iss"}

	auths := []configs.Auth{
		{AuthConfig: base},
		{
			AuthConfig: ginjwt.AuthConfig{Enabled: false, Audience: "aud2", Issuer: "iss2"},
			Cedar:      configs.CedarConfig{Enabled: true, URL: "http://127.0.0.1:8180"},
		},
	}

	mtm, err := MultiTokenMiddlewareFromConfigs(auths, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, mtm)
}
