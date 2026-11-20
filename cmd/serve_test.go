package cmd

import (
	"context"
	"database/sql"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/pkg/configs"
)

// envOr returns the environment variable key, or def when it is unset.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return def
}

// testDBURI returns a connection URI for db, a database on the PostgreSQL
// test server that dbtools.NewPGTestServer connects to.
func testDBURI(t *testing.T, db *sql.DB) string {
	t.Helper()

	var name string

	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT current_database()`).Scan(&name))

	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(envOr("PGUSER", "postgres"), envOr("PGPASSWORD", "postgres")),
		Host:     net.JoinHostPort(envOr("PGHOST", "pg"), envOr("PGPORT", "5432")),
		Path:     name,
		RawQuery: "sslmode=disable",
	}

	return u.String()
}

// setGlobalConfig sets the oidc list of the YAML cfg and each of values on the
// global viper instance that the cobra commands read, and puts the previous
// values back when the test ends.
func setGlobalConfig(t *testing.T, cfg string, values map[string]any) {
	t.Helper()

	all := map[string]any{"oidc": configFor(t, "postgres", cfg).Get("oidc")}
	for k, v := range values {
		all[k] = v
	}

	for k, v := range all {
		prev := viper.Get(k)

		viper.Set(k, v)
		t.Cleanup(func() { viper.Set(k, prev) })
	}
}

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

// TestStartAPIWithoutIdentityProvider starts serve against an empty database
// with no oidc provider that has identity.enabled. startAPI must return the
// error before it applies any migration, and so before it starts the HTTP
// server, which only runs after the migrations.
func TestStartAPIWithoutIdentityProvider(t *testing.T) {
	db := dbtools.NewPGTestServer(t)

	auditLog := filepath.Join(t.TempDir(), "audit.log")
	require.NoError(t, os.WriteFile(auditLog, nil, 0o600))

	setGlobalConfig(t, noIdentityOIDCConfig, map[string]any{
		"db.uri":         testDBURI(t, db),
		"audit.log-path": auditLog,
		"api.listen":     "127.0.0.1:0",
	})

	// startAPI registers the database stats collector with the default registry.
	t.Cleanup(func() { prometheus.Unregister(collectors.NewDBStatsCollector(db, "governor")) })

	err := startAPI(context.Background())
	assert.ErrorIs(t, err, configs.ErrNoIdentityProvider)
	assert.ErrorContains(t, err, "set identity.enabled: true on the provider that issued the existing user IDs")
	assert.False(t, hasVersionTable(t, db), "no migration may run")
}
