package dbtools

import (
	"database/sql"
	"os"
	"testing"

	"github.com/cockroachdb/cockroach-go/v2/testserver"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
	"github.com/peterldowns/pgtestdb"
)

// TestServerCRDBVersion is the version of CockroachDB that the test server is running
// v23.1.28 is the last version under BSL
const TestServerCRDBVersion = "v23.1.28"

// NewCRDBTestServer creates a new CockroachDB test server
func NewCRDBTestServer() (testserver.TestServer, error) {
	return testserver.NewTestServer(testserver.CustomVersionOpt(TestServerCRDBVersion))
}

// NewPGTestServer creates a new PostgreSQL test server
func NewPGTestServer(t *testing.T) *sql.DB {
	pghost := os.Getenv("PGHOST")
	if pghost == "" {
		pghost = "pg"
	}

	pgport := os.Getenv("PGPORT")
	if pgport == "" {
		pgport = "5432"
	}

	pguser := os.Getenv("PGUSER")
	if pguser == "" {
		pguser = "postgres"
	}

	pgpassword := os.Getenv("PGPASSWORD")
	if pgpassword == "" {
		pgpassword = "postgres"
	}

	return pgtestdb.New(
		t,
		pgtestdb.Config{
			DriverName:                "pgx",
			User:                      pguser,
			Password:                  pgpassword,
			Host:                      pghost,
			Port:                      pgport,
			Options:                   "sslmode=disable",
			ForceTerminateConnections: true,
		},
		pgtestdb.NoopMigrator{},
	)
}
