package psql

import (
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
