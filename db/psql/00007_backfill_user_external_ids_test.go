package psql

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
)

const pingIssuer = "https://ping.example.com"

type seedUser struct {
	email      string
	externalID sql.NullString
	deleted    bool
}

type externalIDRow struct {
	Email      string
	Iss        string
	ExternalID string
}

type userExternalID struct {
	Email      string
	ExternalID sql.NullString
}

// backfillSeed has three users with an external ID (one of them soft-deleted)
// and one without.
var backfillSeed = []seedUser{
	{email: "alice@example.com", externalID: sql.NullString{String: "okta-alice", Valid: true}},
	{email: "bob@example.com", externalID: sql.NullString{String: "okta-bob", Valid: true}},
	{email: "carol@example.com", externalID: sql.NullString{String: "okta-carol", Valid: true}, deleted: true},
	{email: "dave@example.com"},
}

// migratedTo returns a fresh database migrated to version with testIssuer as
// the only identity provider, seeded with users once the users table exists.
func migratedTo(t *testing.T, version int64, users []seedUser) *sql.DB {
	t.Helper()

	ctx := context.Background()
	db := dbtools.NewPGTestServer(t)

	p, err := NewProvider(db, []string{testIssuer})
	require.NoError(t, err)

	_, err = p.UpTo(ctx, 6)
	require.NoError(t, err)

	for _, u := range users {
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (name, email, external_id, created_at, updated_at, deleted_at)
			VALUES ($1, $2, $3, now(), now(), CASE WHEN $4 THEN now() END)`,
			u.email, u.email, u.externalID, u.deleted,
		)
		require.NoError(t, err)
	}

	_, err = p.UpTo(ctx, version)
	require.NoError(t, err)

	return db
}

func dbVersion(t *testing.T, db *sql.DB) int64 {
	t.Helper()

	p, err := NewProvider(db, []string{testIssuer})
	require.NoError(t, err)

	v, err := p.GetDBVersion(context.Background())
	require.NoError(t, err)

	return v
}

func externalIDRows(t *testing.T, db *sql.DB) []externalIDRow {
	t.Helper()

	rows, err := db.QueryContext(context.Background(),
		`SELECT u.email, uei.iss, uei.external_id
		FROM user_external_ids uei JOIN users u ON u.id = uei.user_id
		ORDER BY u.email`)
	require.NoError(t, err)

	defer rows.Close()

	got := []externalIDRow{}

	for rows.Next() {
		var r externalIDRow

		require.NoError(t, rows.Scan(&r.Email, &r.Iss, &r.ExternalID))

		got = append(got, r)
	}

	require.NoError(t, rows.Err())

	return got
}

func userExternalIDs(t *testing.T, db *sql.DB) []userExternalID {
	t.Helper()

	rows, err := db.QueryContext(context.Background(), `SELECT email, external_id FROM users ORDER BY email`)
	require.NoError(t, err)

	defer rows.Close()

	got := []userExternalID{}

	for rows.Next() {
		var u userExternalID

		require.NoError(t, rows.Scan(&u.Email, &u.ExternalID))

		got = append(got, u)
	}

	require.NoError(t, rows.Err())

	return got
}

func TestBackfillMigrationUp(t *testing.T) {
	tests := []struct {
		name        string
		users       []seedUser
		issuers     []string
		wantErr     error
		wantVersion int64
		wantRows    []externalIDRow
	}{
		{
			name:        "no flagged provider with rows",
			users:       backfillSeed,
			issuers:     nil,
			wantErr:     ErrIdentityProviderCount,
			wantVersion: 6,
			wantRows:    []externalIDRow{},
		},
		{
			name:        "no flagged provider without rows",
			issuers:     nil,
			wantErr:     ErrIdentityProviderCount,
			wantVersion: 6,
			wantRows:    []externalIDRow{},
		},
		{
			name:        "two flagged providers with rows",
			users:       backfillSeed,
			issuers:     []string{testIssuer, pingIssuer},
			wantErr:     ErrIdentityProviderCount,
			wantVersion: 6,
			wantRows:    []externalIDRow{},
		},
		{
			name:        "two flagged providers without rows",
			issuers:     []string{testIssuer, pingIssuer},
			wantErr:     ErrIdentityProviderCount,
			wantVersion: 6,
			wantRows:    []externalIDRow{},
		},
		{
			name:        "one flagged provider copies active users' external IDs",
			users:       backfillSeed,
			issuers:     []string{testIssuer},
			wantVersion: 7,
			wantRows: []externalIDRow{
				{Email: "alice@example.com", Iss: testIssuer, ExternalID: "okta-alice"},
				{Email: "bob@example.com", Iss: testIssuer, ExternalID: "okta-bob"},
			},
		},
		{
			name:        "one flagged provider without rows",
			issuers:     []string{testIssuer},
			wantVersion: 7,
			wantRows:    []externalIDRow{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := migratedTo(t, 6, tt.users)

			p, err := NewProvider(db, tt.issuers)
			require.NoError(t, err)

			_, err = p.UpTo(context.Background(), 7)
			assert.ErrorIs(t, err, tt.wantErr)

			assert.Equal(t, tt.wantVersion, dbVersion(t, db))
			assert.Equal(t, tt.wantRows, externalIDRows(t, db))
		})
	}
}

func TestBackfillMigrationDown(t *testing.T) {
	migrated := []externalIDRow{
		{Email: "alice@example.com", Iss: testIssuer, ExternalID: "okta-alice"},
		{Email: "bob@example.com", Iss: testIssuer, ExternalID: "okta-bob"},
	}

	cleared := []userExternalID{
		{Email: "alice@example.com"},
		{Email: "bob@example.com"},
		{Email: "carol@example.com"},
		{Email: "dave@example.com"},
	}

	tests := []struct {
		name        string
		issuers     []string
		wantErr     error
		wantVersion int64
		wantRows    []externalIDRow
		wantUsers   []userExternalID
	}{
		{
			name:        "no flagged provider",
			issuers:     nil,
			wantErr:     ErrIdentityProviderCount,
			wantVersion: 7,
			wantRows:    migrated,
			wantUsers:   cleared,
		},
		{
			name:        "two flagged providers",
			issuers:     []string{testIssuer, pingIssuer},
			wantErr:     ErrIdentityProviderCount,
			wantVersion: 7,
			wantRows:    migrated,
			wantUsers:   cleared,
		},
		{
			name:        "one flagged provider refills the column and removes its records",
			issuers:     []string{testIssuer},
			wantVersion: 6,
			wantRows:    []externalIDRow{},
			wantUsers: []userExternalID{
				{Email: "alice@example.com", ExternalID: sql.NullString{String: "okta-alice", Valid: true}},
				{Email: "bob@example.com", ExternalID: sql.NullString{String: "okta-bob", Valid: true}},
				{Email: "carol@example.com"},
				{Email: "dave@example.com"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := migratedTo(t, 7, backfillSeed)

			// The down step of the column drop re-adds users.external_id empty;
			// clear it so the refill is what the assertions see.
			_, err := db.ExecContext(context.Background(), `UPDATE users SET external_id = NULL`)
			require.NoError(t, err)

			p, err := NewProvider(db, tt.issuers)
			require.NoError(t, err)

			_, err = p.DownTo(context.Background(), 6)
			assert.ErrorIs(t, err, tt.wantErr)

			assert.Equal(t, tt.wantVersion, dbVersion(t, db))
			assert.Equal(t, tt.wantRows, externalIDRows(t, db))
			assert.Equal(t, tt.wantUsers, userExternalIDs(t, db))
		})
	}
}
