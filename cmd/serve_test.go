package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/metal-toolbox/governor-api/pkg/configs"
)

func TestLoadAuthConfigs(t *testing.T) {
	tests := []struct {
		name        string
		cfg         string
		wantIssuers []string
		wantErr     error
		// wantErrText lists what the operator must read in the error.
		wantErrText []string
	}{
		{
			name: "one provider with identity enabled",
			cfg: `
oidc:
  - enabled: true
    audience: api
    issuer: https://hydra
    jwksuri: https://hydra/jwks
  - enabled: true
    audience: api
    issuer: https://okta
    jwksuri: https://okta/jwks
    identity:
      enabled: true
`,
			wantIssuers: []string{"https://hydra", "https://okta"},
		},
		{
			name: "no provider with identity enabled",
			cfg: `
oidc:
  - enabled: true
    audience: api
    issuer: https://hydra
    jwksuri: https://hydra/jwks
  - enabled: true
    audience: api
    issuer: https://okta
    jwksuri: https://okta/jwks
`,
			wantErr: configs.ErrNoIdentityProvider,
			wantErrText: []string{
				"set identity.enabled: true on the provider that issued the existing user IDs",
				"https://hydra",
				"https://okta",
			},
		},
		{
			name: "no enabled provider",
			cfg: `
oidc:
  - enabled: false
    audience: api
    issuer: https://okta
    jwksuri: https://okta/jwks
    identity:
      enabled: true
`,
			wantErr: ErrNoOIDCConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			v.SetConfigType("yaml")
			require.NoError(t, v.ReadConfig(strings.NewReader(tt.cfg)))

			auths, err := loadAuthConfigs(v)
			assert.ErrorIs(t, err, tt.wantErr)

			for _, text := range tt.wantErrText {
				assert.ErrorContains(t, err, text)
			}

			var issuers []string
			for _, a := range auths {
				issuers = append(issuers, a.Issuer)
			}

			assert.Equal(t, tt.wantIssuers, issuers)
		})
	}
}
