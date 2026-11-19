package psql

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
)

// externalIDSchema is what the database has of users.external_id: the column
// and its partial unique index.
type externalIDSchema struct {
	Column bool
	Index  bool
}

func usersExternalIDSchema(t *testing.T, db *sql.DB) externalIDSchema {
	t.Helper()

	ctx := context.Background()

	var got externalIDSchema

	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_name = 'users' AND column_name = 'external_id')`).Scan(&got.Column))
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_indexes
		WHERE tablename = 'users' AND indexname = 'users_external_id_key_1')`).Scan(&got.Index))

	return got
}

func TestDropUsersExternalIDUp(t *testing.T) {
	db := migratedTo(t, 8, backfillSeed)

	assert.Equal(t, int64(8), dbVersion(t, db))
	assert.Equal(t, externalIDSchema{}, usersExternalIDSchema(t, db))
	assert.Equal(t, []externalIDRow{
		{Email: "alice@example.com", Iss: testIssuer, ExternalID: "okta-alice"},
		{Email: "bob@example.com", Iss: testIssuer, ExternalID: "okta-bob"},
	}, externalIDRows(t, db))
}

func TestDropUsersExternalIDDown(t *testing.T) {
	tests := []struct {
		name      string
		downTo    int64
		wantUsers []userExternalID
	}{
		{
			name:   "column comes back empty",
			downTo: 7,
			wantUsers: []userExternalID{
				{Email: "alice@example.com"},
				{Email: "bob@example.com"},
				{Email: "carol@example.com"},
				{Email: "dave@example.com"},
			},
		},
		{
			name:   "the backfill down step refills it",
			downTo: 5,
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
			db := migratedTo(t, 8, backfillSeed)

			p, err := NewProvider(db, []string{testIssuer})
			require.NoError(t, err)

			_, err = p.DownTo(context.Background(), tt.downTo)
			require.NoError(t, err)

			assert.Equal(t, tt.downTo, dbVersion(t, db))
			assert.Equal(t, externalIDSchema{Column: true, Index: true}, usersExternalIDSchema(t, db))
			assert.Equal(t, tt.wantUsers, userExternalIDs(t, db))
		})
	}
}

func TestDropUsersExternalIDDownIndex(t *testing.T) {
	tests := []struct {
		name           string
		otherDeleted   bool
		wantConstraint string
		wantViolation  bool
	}{
		{
			name:           "two active users with the same external id",
			wantConstraint: "users_external_id_key_1",
			wantViolation:  true,
		},
		{
			name:         "a deleted and an active user with the same external id",
			otherDeleted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			db := migratedTo(t, 8, nil)

			p, err := NewProvider(db, []string{testIssuer})
			require.NoError(t, err)

			_, err = p.DownTo(ctx, 7)
			require.NoError(t, err)

			_, err = db.ExecContext(ctx,
				`INSERT INTO users (name, email, external_id, created_at, updated_at, deleted_at)
				VALUES ('Other', 'other@example.com', 'shared', now(), now(), CASE WHEN $1 THEN now() END)`,
				tt.otherDeleted)
			require.NoError(t, err)

			_, err = db.ExecContext(ctx,
				`INSERT INTO users (name, email, external_id, created_at, updated_at)
				VALUES ('Active', 'active@example.com', 'shared', now(), now())`)

			constraint, violation := dbtools.UniqueViolation(err)
			assert.Equal(t, []any{tt.wantConstraint, tt.wantViolation}, []any{constraint, violation})
		})
	}
}
