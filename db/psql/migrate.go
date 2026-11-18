// Package psql provides an embedded filesystem containing all the psql migrations
// and the goose Provider that runs them.
package psql

import (
	"database/sql"
	"embed"
	"io/fs"

	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

// Migrations contain an embedded filesystem with all the sql migration files
//
//go:embed migrations/*.sql
var Migrations embed.FS

// Option configures NewProvider.
type Option func(*providerConfig)

type providerConfig struct {
	logger *zap.Logger
}

// WithLogger sets the logger the Go migrations write their results to. The
// default discards them.
func WithLogger(l *zap.Logger) Option {
	return func(c *providerConfig) {
		c.logger = l
	}
}

// NewProvider returns the goose Provider for the PostgreSQL migrations. It is the
// only supported way to run them: it always includes Go migration 00007, which
// needs identityIssuers (the issuers of the oidc providers with identity.enabled,
// in config order) and fails with ErrIdentityProviderCount unless there is
// exactly one. Globally registered goose Go migrations are ignored.
func NewProvider(db *sql.DB, identityIssuers []string, opts ...Option) (*goose.Provider, error) {
	cfg := providerConfig{logger: zap.NewNop()}

	for _, opt := range opts {
		opt(&cfg)
	}

	// The embedded paths start with migrations/, goose expects the files at the root.
	fsys, err := fs.Sub(Migrations, "migrations")
	if err != nil {
		return nil, err
	}

	return goose.NewProvider(goose.DialectPostgres, db, fsys,
		goose.WithDisableGlobalRegistry(true),
		goose.WithGoMigrations(newBackfillMigration(identityIssuers, cfg.logger)),
	)
}
