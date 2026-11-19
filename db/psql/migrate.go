// Package psql provides an embedded filesystem containing all the psql migrations
// and the two ways to run them with Go migration 00007: a goose Provider, and
// RunContext for the goose CLI commands.
package psql

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

// Migrations contain an embedded filesystem with all the sql migration files
//
//go:embed migrations/*.sql
var Migrations embed.FS

// Option configures NewProvider and RunContext.
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

// NewProvider returns the goose Provider for the PostgreSQL migrations. It
// always includes Go migration 00007, which needs identityIssuers (the issuers
// of the oidc providers with identity.enabled, in config order) and fails with
// ErrIdentityProviderCount unless there is exactly one. Globally registered
// goose Go migrations are ignored.
func NewProvider(db *sql.DB, identityIssuers []string, opts ...Option) (*goose.Provider, error) {
	cfg := newProviderConfig(opts)

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

// RunContext runs a goose CLI command (up, down, status, create, ...) on the
// PostgreSQL migrations through goose's package-level API, as
// goose.RunContext does. Apart from create and fix, it first replaces every
// globally registered goose Go migration with Go migration 00007, which needs
// identityIssuers as described for NewProvider. create and fix only write
// migration files and ignore identityIssuers: they run with no Go migration
// registered, because fix would count a registered one as a migration file.
//
// RunContext changes goose's global state (registered Go migrations, base
// filesystem, dialect), so it is not safe for concurrent use.
func RunContext(ctx context.Context, db *sql.DB, identityIssuers []string, command string, args []string, opts ...Option) error {
	cfg := newProviderConfig(opts)

	goose.ResetGlobalMigrations()

	if command != "create" && command != "fix" {
		if err := goose.SetGlobalMigrations(newBackfillMigration(identityIssuers, cfg.logger)); err != nil {
			return fmt.Errorf("register migration 00007: %w", err)
		}
	}

	goose.SetBaseFS(Migrations)

	if err := goose.SetDialect(string(goose.DialectPostgres)); err != nil {
		return err
	}

	return goose.RunContext(ctx, command, db, "migrations", args...)
}

func newProviderConfig(opts []Option) providerConfig {
	cfg := providerConfig{logger: zap.NewNop()}

	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}
