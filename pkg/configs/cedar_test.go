package configs

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCedarConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     CedarConfig
		wantErr error
	}{
		{
			name: "disabled is always valid",
			cfg:  CedarConfig{Enabled: false},
		},
		{
			name: "enabled with url is valid",
			cfg:  CedarConfig{Enabled: true, URL: "http://127.0.0.1:8180", Timeout: 250 * time.Millisecond},
		},
		{
			name:    "enabled without url is invalid",
			cfg:     CedarConfig{Enabled: true},
			wantErr: ErrCedarURLRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestCedarConfig_TimeoutOrDefault(t *testing.T) {
	assert.Equal(t, DefaultCedarTimeout, CedarConfig{}.TimeoutOrDefault())
	assert.Equal(t, DefaultCedarTimeout, CedarConfig{Timeout: 0}.TimeoutOrDefault())
	assert.Equal(t, 5*time.Second, CedarConfig{Timeout: 5 * time.Second}.TimeoutOrDefault())
}

func TestGetAuthFromFlags(t *testing.T) {
	const cfg = `
oidc:
  - enabled: true
    audience: aud-a
    issuer: https://issuer-a
    jwksuri: https://issuer-a/jwks
    cedar:
      enabled: true
      url: http://127.0.0.1:8180
      timeout: 250ms
  - enabled: true
    audience: aud-b
    issuer: https://issuer-b
    jwksuri: https://issuer-b/jwks
  - enabled: false
    audience: aud-c
    issuer: https://issuer-c
    jwksuri: https://issuer-c/jwks
`

	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(cfg)))

	auths, err := GetAuthFromFlags(v)
	require.NoError(t, err)

	// disabled provider (issuer-c) is filtered out
	require.Len(t, auths, 2)

	assert.Equal(t, "https://issuer-a", auths[0].Issuer)
	assert.True(t, auths[0].Cedar.Enabled)
	assert.Equal(t, "http://127.0.0.1:8180", auths[0].Cedar.URL)
	assert.Equal(t, 250*time.Millisecond, auths[0].Cedar.Timeout)

	assert.Equal(t, "https://issuer-b", auths[1].Issuer)
	assert.False(t, auths[1].Cedar.Enabled)
}

func TestGetAuthFromFlags_sharedIssuer(t *testing.T) {
	// Providers can share an issuer and differ only by audience; a cedar block
	// on one must not bleed onto its same-issuer siblings.
	const cfg = `
oidc:
  - name: user
    enabled: true
    audience: api://default
    issuer: https://issuer
    jwksuri: https://issuer/jwks
  - name: wif
    enabled: true
    audience: wiftest
    issuer: https://issuer
    jwksuri: https://issuer/jwks
    cedar:
      enabled: true
      url: http://127.0.0.1:8188
`

	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(cfg)))

	auths, err := GetAuthFromFlags(v)
	require.NoError(t, err)
	require.Len(t, auths, 2)

	// same issuer, distinguished by audience
	assert.Equal(t, "api://default", auths[0].Audience)
	assert.False(t, auths[0].Cedar.Enabled, "user provider must not inherit wif's cedar block")

	assert.Equal(t, "wiftest", auths[1].Audience)
	assert.True(t, auths[1].Cedar.Enabled)
	assert.Equal(t, "http://127.0.0.1:8188", auths[1].Cedar.URL)
}

func TestGetAuthFromFlags_invalidCedar(t *testing.T) {
	const cfg = `
oidc:
  - enabled: true
    audience: aud-a
    issuer: https://issuer-a
    jwksuri: https://issuer-a/jwks
    cedar:
      enabled: true
`

	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(cfg)))

	_, err := GetAuthFromFlags(v)
	require.ErrorIs(t, err, ErrCedarURLRequired)
}
