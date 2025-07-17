//go:build testtools
// +build testtools

package dbtools

import (
	"context"
	"os"
	"testing"

	// import the crdbpgx for automatic retries of errors for crdb that support retry
	_ "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	// Import postgres driver. Needed since cockroach-go stopped importing it in v2.2.10
	_ "github.com/lib/pq"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	models "github.com/metal-toolbox/governor-api/internal/models/psql"
)

var (
	// TestDBURI is the URI for the test database
	TestDBURI = os.Getenv("GOVERNOR_DB_URI")
	testDB    *sqlx.DB
)

// init sets a default TestDBURI if one isn't supplied in environment
func init() {
	if TestDBURI == "" {
		TestDBURI = "host=localhost port=26257 user=root sslmode=disable"
	}
}

func testDatastore() error {
	// don't setup the datastore if we already have one
	if testDB != nil {
		return nil
	}

	// Uncomment when you are having database issues with your tests and need to see the db logs
	// Hidden by default because it can be noisy and make it harder to read normal failures.
	// You can also enable at the beginning of your test and then disable it again at the end
	// boil.DebugMode = true

	db, err := sqlx.Open("postgres", TestDBURI)
	if err != nil {
		return err
	}

	testDB = db

	cleanDB()

	return addFixtures()
}

// DatabaseTest allows you to run tests that interact with the database
func DatabaseTest(t *testing.T) *sqlx.DB {
	RegisterHooks()

	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}

	t.Cleanup(func() {
		cleanDB()

		err := addFixtures()

		require.NoError(t, err, "Unexpected error setting up fixture data")
	})

	err := testDatastore()
	require.NoError(t, err, "Unexpected error getting connection to test datastore")

	return testDB
}

// nolint
func cleanDB() {
	ctx := context.TODO()
	// Make sure the deletion goes in order so you don't break the databases foreign key constraints
	testDB.Exec("SET sql_safe_updates = false;")
	models.GroupMemberships().DeleteAll(ctx, testDB)
	models.GroupMembershipRequests().DeleteAll(ctx, testDB)
	models.Groups().DeleteAll(ctx, testDB, true)
	models.Users().DeleteAll(ctx, testDB, true)
	testDB.Exec("SET sql_safe_updates = true;")
}
