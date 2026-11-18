package cmd

import "errors"

var (
	// ErrMissingNATSCreds is returned when nats creds are not provided
	ErrMissingNATSCreds = errors.New("nats creds are required")
	// ErrMissingNATSTokenURL is returned when workload identity federation config is incomplete
	ErrMissingNATSTokenURL = errors.New("workload identity federation token URL is required")
	// ErrNoOIDCConfig is returned when no enabled oidc provider is configured
	ErrNoOIDCConfig = errors.New("no oidc auth configs found")
	// ErrUnsupportedMigrateCommand is returned for a migrate command the PostgreSQL
	// migration Provider does not run
	ErrUnsupportedMigrateCommand = errors.New("unsupported migrate command")
	// ErrMigrateVersionRequired is returned when up-to or down-to has no VERSION argument
	ErrMigrateVersionRequired = errors.New("migrate command needs a VERSION argument")
	// ErrUnsupportedDBDriver is returned for a db.driver other than postgres or crdb
	ErrUnsupportedDBDriver = errors.New("unsupported database driver")
	// ErrRestoreAfterBackfill is returned by restore --migrate when the database
	// has already run migration 00007, which would leave the restored users
	// without identity records
	ErrRestoreAfterBackfill = errors.New("restore --migrate needs a database that has not run migration 00007 (the users.external_id backfill); restore into a new database")
)
