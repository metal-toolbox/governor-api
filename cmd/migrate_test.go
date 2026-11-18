package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
	"github.com/pressly/goose/v3"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	psqlmigrations "github.com/metal-toolbox/governor-api/db/psql"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/pkg/configs"
)

// testIdentityIssuer is the one identity provider the test databases are migrated with.
const testIdentityIssuer = "https://okta.example.com/oauth2/default"

const identityOIDCConfig = `
oidc:
  - enabled: true
    audience: api
    issuer: ` + testIdentityIssuer + `
    jwksuri: https://okta.example.com/jwks
    identity:
      enabled: true
`

const noIdentityOIDCConfig = `
oidc:
  - enabled: true
    audience: api
    issuer: https://okta.example.com/oauth2/default
    jwksuri: https://okta.example.com/jwks
`

func TestMain(m *testing.M) {
	// cobra's initConfig sets the package logger; tests call the commands directly.
	logger = zap.NewNop().Sugar()

	os.Exit(m.Run())
}

// configFor returns a viper instance with driver as db.driver and cfg as the
// rest of the config.
func configFor(t *testing.T, driver, cfg string) *viper.Viper {
	t.Helper()

	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(cfg)))
	v.Set("db.driver", driver)

	return v
}

// migratedTestDB returns a fresh database migrated to version with
// testIdentityIssuer as the only identity provider.
func migratedTestDB(t *testing.T, version int64) *sql.DB {
	t.Helper()

	db := dbtools.NewPGTestServer(t)

	p, err := psqlmigrations.NewProvider(db, []string{testIdentityIssuer})
	require.NoError(t, err)

	_, err = p.UpTo(context.Background(), version)
	require.NoError(t, err)

	return db
}

func testDBVersion(t *testing.T, db *sql.DB) int64 {
	t.Helper()

	p, err := psqlmigrations.NewProvider(db, []string{testIdentityIssuer})
	require.NoError(t, err)

	v, err := p.GetDBVersion(context.Background())
	require.NoError(t, err)

	return v
}

func hasVersionTable(t *testing.T, db *sql.DB) bool {
	t.Helper()

	var exists bool

	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT to_regclass('public.goose_db_version') IS NOT NULL`).Scan(&exists))

	return exists
}

// unconnectedDB returns a *sql.DB that never connects, for commands that do
// not touch the database.
func unconnectedDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("pgx", "postgres://unused.invalid/governor")
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	return db
}

// captureGooseLog sends the goose package log to a buffer for the test.
func captureGooseLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}

	goose.SetLogger(log.New(buf, "", 0))
	t.Cleanup(func() {
		goose.SetLogger(log.Default())
		goose.SetBaseFS(nil)
	})

	return buf
}

func TestRunProviderCommand(t *testing.T) {
	tests := []struct {
		name        string
		start       int64
		command     string
		args        []string
		wantErr     error
		wantVersion int64
		wantOut     []string
	}{
		{name: "up", start: 5, command: "up", wantVersion: 7, wantOut: []string{"00006_user_external_ids.sql", "00007_backfill_user_external_ids.go"}},
		{name: "up-by-one", start: 5, command: "up-by-one", wantVersion: 6, wantOut: []string{"00006_user_external_ids.sql"}},
		{name: "up-to", start: 3, command: "up-to", args: []string{"5"}, wantVersion: 5, wantOut: []string{"00004_group_metadata.sql", "00005_extension_resource_annotations.sql"}},
		{name: "up-to without version", start: 3, command: "up-to", wantErr: ErrMigrateVersionRequired, wantVersion: 3},
		{name: "up-to with invalid version", start: 3, command: "up-to", args: []string{"five"}, wantErr: strconv.ErrSyntax, wantVersion: 3},
		{name: "down", start: 7, command: "down", wantVersion: 6, wantOut: []string{"down 00007_backfill_user_external_ids.go"}},
		{name: "down-to", start: 7, command: "down-to", args: []string{"5"}, wantVersion: 5, wantOut: []string{"down 00007_backfill_user_external_ids.go", "down 00006_user_external_ids.sql"}},
		{name: "down-to without version", start: 7, command: "down-to", wantErr: ErrMigrateVersionRequired, wantVersion: 7},
		{name: "redo", start: 7, command: "redo", wantVersion: 7, wantOut: []string{"down 00007_backfill_user_external_ids.go", "up 00007_backfill_user_external_ids.go"}},
		{name: "reset", start: 7, command: "reset", wantVersion: 0, wantOut: []string{"down 00001_initial.sql"}},
		{name: "status", start: 6, command: "status", wantVersion: 6, wantOut: []string{"6\tapplied\t", "7\tpending\t-\n"}},
		{name: "version", start: 6, command: "version", wantVersion: 6, wantOut: []string{"version 6\n"}},
		{name: "create is not a provider command", start: 6, command: "create", args: []string{"x", "sql"}, wantErr: ErrUnsupportedMigrateCommand, wantVersion: 6},
		{name: "fix is not a provider command", start: 6, command: "fix", wantErr: ErrUnsupportedMigrateCommand, wantVersion: 6},
		{name: "unknown command", start: 6, command: "sideways", wantErr: ErrUnsupportedMigrateCommand, wantVersion: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := migratedTestDB(t, tt.start)

			p, err := psqlmigrations.NewProvider(db, []string{testIdentityIssuer})
			require.NoError(t, err)

			out := &bytes.Buffer{}

			err = runProviderCommand(context.Background(), p, tt.command, tt.args, out)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantVersion, testDBVersion(t, db))

			for _, want := range tt.wantOut {
				assert.Contains(t, out.String(), want)
			}
		})
	}
}

func TestRunMigratePostgresChecksIdentityProviders(t *testing.T) {
	tests := []struct {
		name        string
		cfg         string
		wantErr     error
		wantApplied bool
	}{
		{
			name:        "provider with identity enabled",
			cfg:         identityOIDCConfig,
			wantApplied: true,
		},
		{
			name:    "no provider with identity enabled",
			cfg:     noIdentityOIDCConfig,
			wantErr: configs.ErrNoIdentityProvider,
		},
		{
			name:    "no oidc config",
			cfg:     "logging:\n  debug: false\n",
			wantErr: ginjwt.ErrMissingAuthConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := dbtools.NewPGTestServer(t)

			err := runMigrate(context.Background(), db, configFor(t, "postgres", tt.cfg), "up", nil, &bytes.Buffer{})
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantApplied, hasVersionTable(t, db))
		})
	}
}

func TestRunMigrateCRDBSkipsIdentityCheck(t *testing.T) {
	gooseLog := captureGooseLog(t)
	db := dbtools.NewPGTestServer(t)

	// No oidc config at all: the crdb branch must not ask for one.
	err := runMigrate(context.Background(), db, configFor(t, "crdb", "logging:\n  debug: false\n"), "status", nil, &bytes.Buffer{})
	require.NoError(t, err)

	// 00002_user_gh_id.sql exists only in the crdb migrations, so goose.RunContext
	// listed the crdb set and not the psql Provider's.
	assert.Contains(t, gooseLog.String(), "00002_user_gh_id.sql")
}

func TestRunMigrateUnsupportedDriver(t *testing.T) {
	err := runMigrate(context.Background(), unconnectedDB(t), configFor(t, "mysql", identityOIDCConfig), "up", nil, &bytes.Buffer{})
	assert.ErrorIs(t, err, ErrUnsupportedDBDriver)
}

func TestRunMigratePostgresCreate(t *testing.T) {
	captureGooseLog(t)

	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.Mkdir("migrations", 0o750))

	// create writes a file and applies nothing, so it needs no oidc config.
	err := runMigrate(context.Background(), unconnectedDB(t), configFor(t, "postgres", "logging:\n  debug: false\n"),
		"create", []string{"add_foo", "sql"}, &bytes.Buffer{})
	require.NoError(t, err)

	files, err := filepath.Glob(filepath.Join(dir, "migrations", "*_add_foo.sql"))
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestRunMigratePostgresFix(t *testing.T) {
	captureGooseLog(t)

	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.Mkdir("migrations", 0o750))
	require.NoError(t, os.WriteFile(filepath.Join("migrations", "00001_initial.sql"), []byte("-- +goose Up\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join("migrations", "20260101000000_add_foo.sql"), []byte("-- +goose Up\n"), 0o600))

	err := runMigrate(context.Background(), unconnectedDB(t), configFor(t, "postgres", "logging:\n  debug: false\n"),
		"fix", nil, &bytes.Buffer{})
	require.NoError(t, err)

	entries, err := os.ReadDir(filepath.Join(dir, "migrations"))
	require.NoError(t, err)

	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}

	assert.Equal(t, []string{"00001_initial.sql", "00002_add_foo.sql"}, names)
}

var errTestRestore = errors.New("restore broke")

func TestRestoreWithMigration(t *testing.T) {
	tests := []struct {
		name        string
		restoreErr  error
		wantErr     error
		wantVersion int64
		wantRows    int
	}{
		{
			name:        "restored users are backfilled",
			wantVersion: psqlmigrations.BackfillVersion,
			wantRows:    1,
		},
		{
			name:        "a failed restore stops before the backfill",
			restoreErr:  errTestRestore,
			wantErr:     errTestRestore,
			wantVersion: psqlmigrations.BackfillVersion - 1,
			wantRows:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db := dbtools.NewPGTestServer(t)
			auths := []configs.Auth{{
				AuthConfig: ginjwt.AuthConfig{Enabled: true, Issuer: testIdentityIssuer},
				Identity:   configs.IdentityConfig{Enabled: true},
			}}

			restore := func(ctx context.Context) error {
				_, err := db.ExecContext(ctx,
					`INSERT INTO users (name, email, external_id, created_at, updated_at)
					VALUES ('Alice', 'alice@example.com', 'okta-alice', now(), now())`)
				require.NoError(t, err)

				return tt.restoreErr
			}

			err := restoreWithMigration(ctx, db, auths, restore)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantVersion, testDBVersion(t, db))

			var rows int

			require.NoError(t, db.QueryRowContext(ctx,
				`SELECT count(*) FROM user_external_ids WHERE iss = $1 AND external_id = 'okta-alice'`,
				testIdentityIssuer).Scan(&rows))
			assert.Equal(t, tt.wantRows, rows)
		})
	}
}

func TestPrintStatus(t *testing.T) {
	out := &bytes.Buffer{}

	printStatus(out, []*goose.MigrationStatus{
		{Source: &goose.Source{Version: 6}, State: goose.StateApplied, AppliedAt: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)},
		{Source: &goose.Source{Version: 7}, State: goose.StatePending},
	})

	assert.Equal(t, "6\tapplied\t2026-11-18T12:00:00Z\n7\tpending\t-\n", out.String())
}
