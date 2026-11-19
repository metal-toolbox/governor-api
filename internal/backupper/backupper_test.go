package backupper

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	psqlmigrations "github.com/metal-toolbox/governor-api/db/psql"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
)

const (
	testOktaIssuer = "https://okta.example.com/oauth2/default"
	testPingIssuer = "https://ping.example.com"
	testUserID     = "00000004-0000-0000-0000-000000000001"
)

// migratedDB returns a new PostgreSQL test database with every migration applied.
func migratedDB(t *testing.T) *sqlx.DB {
	t.Helper()

	db := dbtools.NewPGTestServer(t)

	p, err := psqlmigrations.NewProvider(db, []string{testOktaIssuer})
	require.NoError(t, err)

	_, err = p.Up(context.Background())
	require.NoError(t, err)

	return sqlx.NewDb(db, "postgres")
}

// identity is one user_external_ids row without its generated columns.
type identity struct {
	UserID     string `db:"user_id"`
	Iss        string `db:"iss"`
	ExternalID string `db:"external_id"`
}

func identities(t *testing.T, db *sqlx.DB) []identity {
	t.Helper()

	var got []identity

	require.NoError(t, db.SelectContext(context.Background(), &got,
		`SELECT user_id, iss, external_id FROM user_external_ids ORDER BY iss`))

	return got
}

func TestPostgresBackupRestoresUserExternalIDs(t *testing.T) {
	ctx := context.Background()
	source := migratedDB(t)

	_, err := source.ExecContext(ctx, `INSERT INTO users (id, name, email, created_at, updated_at)
		VALUES ($1, 'Alice', 'alice@example.com', now(), now())`, testUserID)
	require.NoError(t, err)

	_, err = source.ExecContext(ctx, `INSERT INTO user_external_ids (user_id, iss, external_id)
		VALUES ($1, $2, 'okta-alice'), ($1, $3, 'ping-alice')`, testUserID, testOktaIssuer, testPingIssuer)
	require.NoError(t, err)

	backup := &bytes.Buffer{}
	require.NoError(t, New(source, DBDriverPostgres).Backup(ctx, backup))

	target := migratedDB(t)
	require.NoError(t, New(target, DBDriverPostgres).Restore(ctx, backup))

	assert.Equal(t, []identity{
		{UserID: testUserID, Iss: testOktaIssuer, ExternalID: "okta-alice"},
		{UserID: testUserID, Iss: testPingIssuer, ExternalID: "ping-alice"},
	}, identities(t, target))
}

func TestPostgresRestoreRefusesBackupWithoutExternalIDs(t *testing.T) {
	ctx := context.Background()
	target := migratedDB(t)

	legacy := `{"users": [{"id": "` + testUserID + `", "name": "Alice", "email": "alice@example.com",
		"external_id": "okta-alice", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
		"metadata": {}}]}`

	err := New(target, DBDriverPostgres).Restore(ctx, strings.NewReader(legacy))
	assert.ErrorIs(t, err, ErrBackupWithoutExternalIDs)

	var users int

	require.NoError(t, target.GetContext(ctx, &users, `SELECT count(*) FROM users`))
	assert.Zero(t, users, "nothing is restored")
}
