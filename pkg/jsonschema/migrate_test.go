package jsonschema

import (
	"context"
	"database/sql"

	dbm "github.com/metal-toolbox/governor-api/db/psql"
)

// testIdentityIssuer is the one identity provider the test databases are migrated with.
const testIdentityIssuer = "https://idp.example.com"

// migrateTestDB applies every PostgreSQL migration to a test database.
func migrateTestDB(db *sql.DB) error {
	p, err := dbm.NewProvider(db, []string{testIdentityIssuer})
	if err != nil {
		return err
	}

	_, err = p.Up(context.Background())

	return err
}
