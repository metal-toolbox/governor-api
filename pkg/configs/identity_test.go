package configs

import (
	"strings"
	"testing"

	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(b bool) *bool { return &b }

func TestIdentityConfig_AutoRegisterOrDefault(t *testing.T) {
	tests := []struct {
		name string
		cfg  IdentityConfig
		want bool
	}{
		{name: "unset defaults to true", cfg: IdentityConfig{}, want: true},
		{name: "explicit true", cfg: IdentityConfig{AutoRegister: boolPtr(true)}, want: true},
		{name: "explicit false", cfg: IdentityConfig{AutoRegister: boolPtr(false)}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cfg.AutoRegisterOrDefault())
		})
	}
}

func authFor(issuer, audience string, enabled bool) Auth {
	return Auth{
		AuthConfig: ginjwt.AuthConfig{Enabled: true, Issuer: issuer, Audience: audience},
		Identity:   IdentityConfig{Enabled: enabled},
	}
}

func TestIdentityProviders(t *testing.T) {
	tests := []struct {
		name          string
		auths         []Auth
		wantIssuers   []string
		wantPrimary   string
		wantPrimaryOK bool
		wantErr       error
	}{
		{
			name:        "no providers",
			auths:       nil,
			wantIssuers: []string{},
			wantErr:     ErrNoIdentityProvider,
		},
		{
			name: "no provider flagged",
			auths: []Auth{
				authFor("https://hydra", "api", false),
				authFor("https://okta", "api", false),
			},
			wantIssuers: []string{},
			wantErr:     ErrNoIdentityProvider,
		},
		{
			name: "one provider flagged",
			auths: []Auth{
				authFor("https://okta", "api", true),
			},
			wantIssuers:   []string{"https://okta"},
			wantPrimary:   "https://okta",
			wantPrimaryOK: true,
		},
		{
			name: "two providers flagged in config order",
			auths: []Auth{
				authFor("https://okta", "api", true),
				authFor("https://ping", "api", true),
			},
			wantIssuers:   []string{"https://okta", "https://ping"},
			wantPrimary:   "https://okta",
			wantPrimaryOK: true,
		},
		{
			name: "unflagged providers before the first flagged one",
			auths: []Auth{
				authFor("https://hydra", "api", false),
				authFor("https://sts", "api", false),
				authFor("https://ping", "api", true),
				authFor("https://okta", "api", true),
			},
			wantIssuers:   []string{"https://ping", "https://okta"},
			wantPrimary:   "https://ping",
			wantPrimaryOK: true,
		},
		{
			name: "same issuer flagged on two audiences counts once",
			auths: []Auth{
				authFor("https://okta", "api://default", true),
				authFor("https://okta", "wif", true),
			},
			wantIssuers:   []string{"https://okta"},
			wantPrimary:   "https://okta",
			wantPrimaryOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantIssuers, IdentityIssuers(tt.auths))

			primary, ok := PrimaryIdentityIssuer(tt.auths)
			assert.Equal(t, tt.wantPrimary, primary)
			assert.Equal(t, tt.wantPrimaryOK, ok)

			assert.ErrorIs(t, ValidateIdentityProviders(tt.auths), tt.wantErr)
		})
	}
}

func TestGetAuthFromFlags_identity(t *testing.T) {
	// The user and wif entries share an issuer; the identity block of one must
	// not bleed onto the other.
	const cfg = `
oidc:
  - name: hydra
    enabled: true
    audience: api
    issuer: https://hydra
    jwksuri: https://hydra/jwks
  - name: okta user
    enabled: true
    audience: api://default
    issuer: https://okta
    jwksuri: https://okta/jwks
    identity:
      enabled: true
  - name: okta wif
    enabled: true
    audience: wif
    issuer: https://okta
    jwksuri: https://okta/jwks
  - name: ping
    enabled: true
    audience: api
    issuer: https://ping
    jwksuri: https://ping/jwks
    identity:
      enabled: true
      autoLink: true
      autoRegister: false
`

	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(cfg)))

	auths, err := GetAuthFromFlags(v)
	require.NoError(t, err)
	require.Len(t, auths, 4)

	assert.Equal(t, IdentityConfig{}, auths[0].Identity)
	assert.True(t, auths[0].Identity.AutoRegisterOrDefault())

	assert.Equal(t, IdentityConfig{Enabled: true}, auths[1].Identity)
	assert.True(t, auths[1].Identity.AutoRegisterOrDefault())

	assert.Equal(t, IdentityConfig{}, auths[2].Identity)

	assert.Equal(t, IdentityConfig{Enabled: true, AutoLink: true, AutoRegister: boolPtr(false)}, auths[3].Identity)
	assert.False(t, auths[3].Identity.AutoRegisterOrDefault())
}
