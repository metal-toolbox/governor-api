package psql

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testIssuer = "https://okta.example.com/oauth2/default"

type sourceRef struct {
	Version int64
	Type    goose.MigrationType
	Path    string
}

func TestNewProviderSources(t *testing.T) {
	// sql.Open does not connect, so no database is needed to list the sources.
	db, err := sql.Open("pgx", "postgres://unused.invalid/governor")
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })

	// A globally registered Go migration must not reach the Provider: only the
	// migrations that NewProvider passes itself are applied.
	require.NoError(t, goose.SetGlobalMigrations(goose.NewGoMigration(99, nil, nil)))
	t.Cleanup(goose.ResetGlobalMigrations)

	p, err := NewProvider(db, []string{testIssuer})
	require.NoError(t, err)

	got := []sourceRef{}
	for _, s := range p.ListSources() {
		got = append(got, sourceRef{Version: s.Version, Type: s.Type, Path: s.Path})
	}

	assert.Equal(t, []sourceRef{
		{Version: 1, Type: goose.TypeSQL, Path: "00001_initial.sql"},
		{Version: 2, Type: goose.TypeSQL, Path: "00002_add_unique_user_group_request_constraint.sql"},
		{Version: 3, Type: goose.TypeSQL, Path: "00003_extension_resource_owners.sql"},
		{Version: 4, Type: goose.TypeSQL, Path: "00004_group_metadata.sql"},
		{Version: 5, Type: goose.TypeSQL, Path: "00005_extension_resource_annotations.sql"},
		{Version: 6, Type: goose.TypeSQL, Path: "00006_user_external_ids.sql"},
		{Version: 7, Type: goose.TypeGo, Path: "00007_backfill_user_external_ids.go"},
		{Version: 8, Type: goose.TypeSQL, Path: "00008_drop_users_external_id.sql"},
	}, got)
}

func TestRunContextUp(t *testing.T) {
	tests := []struct {
		name        string
		issuers     []string
		wantErr     error
		wantVersion int64
		wantRows    []externalIDRow
	}{
		{
			name:        "one flagged provider runs the backfill",
			issuers:     []string{testIssuer},
			wantVersion: 8,
			wantRows: []externalIDRow{
				{Email: "alice@example.com", Iss: testIssuer, ExternalID: "okta-alice"},
				{Email: "bob@example.com", Iss: testIssuer, ExternalID: "okta-bob"},
			},
		},
		{
			name:        "no flagged provider",
			issuers:     nil,
			wantErr:     ErrIdentityProviderCount,
			wantVersion: 6,
			wantRows:    []externalIDRow{},
		},
		{
			name:        "two flagged providers",
			issuers:     []string{testIssuer, pingIssuer},
			wantErr:     ErrIdentityProviderCount,
			wantVersion: 6,
			wantRows:    []externalIDRow{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { goose.SetBaseFS(nil) })

			db := migratedTo(t, 6, backfillSeed)

			err := RunContext(context.Background(), db, tt.issuers, "up", nil)
			assert.ErrorIs(t, err, tt.wantErr)

			assert.Equal(t, tt.wantVersion, dbVersion(t, db))
			assert.Equal(t, tt.wantRows, externalIDRows(t, db))
		})
	}
}

func TestRunContextIgnoresEarlierRegistrations(t *testing.T) {
	t.Cleanup(func() { goose.SetBaseFS(nil) })

	db := migratedTo(t, 6, backfillSeed)

	// A Go migration registered before the call, by an earlier run or another
	// package, must not reach the run: version 99 would be applied after 00008.
	require.NoError(t, goose.SetGlobalMigrations(goose.NewGoMigration(99, nil, nil)))
	t.Cleanup(goose.ResetGlobalMigrations)

	require.NoError(t, RunContext(context.Background(), db, []string{testIssuer}, "up", nil))
	assert.Equal(t, int64(8), dbVersion(t, db))
}
