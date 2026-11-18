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
		got = append(got, sourceRef{Version: s.Version, Type: s.Type})
	}

	assert.Equal(t, []sourceRef{
		{Version: 1, Type: goose.TypeSQL},
		{Version: 2, Type: goose.TypeSQL},
		{Version: 3, Type: goose.TypeSQL},
		{Version: 4, Type: goose.TypeSQL},
		{Version: 5, Type: goose.TypeSQL},
		{Version: 6, Type: goose.TypeSQL},
		{Version: 7, Type: goose.TypeGo},
	}, got)
}
