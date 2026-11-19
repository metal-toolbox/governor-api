package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	_ "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5" // crdb retries and postgres interface
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	crdbmigrations "github.com/metal-toolbox/governor-api/db/crdb"
	psqlmigrations "github.com/metal-toolbox/governor-api/db/psql"
	"github.com/metal-toolbox/governor-api/pkg/configs"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate <command> [args]",
	Short: "A brief description of your command",
	Long: `Migrate provides a wrapper around the "goose" migration tool.

Commands:
up                   Migrate the DB to the most recent version available
up-by-one            Migrate the DB up by 1
up-to VERSION        Migrate the DB to a specific VERSION
down                 Roll back the version by 1
down-to VERSION      Roll back to a specific VERSION
redo                 Re-run the latest migration
reset                Roll back all migrations
status               Dump the migration status for the current DB
version              Print the current version of the database
create NAME [sql|go] Creates new migration file with the current timestamp
fix                  Apply sequential ordering to migrations
	`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return migrate(cmd.Context(), args[0], args[1:])
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	migrateCmd.PersistentFlags().String("db-driver", "postgres", "Database driver to use for migrations (default: postgres)")
	viperBindFlag("db.driver", migrateCmd.PersistentFlags().Lookup("db-driver"))
}

func migrate(ctx context.Context, command string, args []string) error {
	db, err := goose.OpenDBWithDriver("postgres", viper.GetString("db.uri"))
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}

	defer db.Close()

	return runMigrate(ctx, db, viper.GetViper(), command, args)
}

// runMigrate runs a migrate command against db with the driver and oidc
// providers configured in v.
func runMigrate(ctx context.Context, db *sql.DB, v *viper.Viper, command string, args []string) error {
	switch driver := v.GetString("db.driver"); driver {
	case "postgres":
		return migratePostgres(ctx, db, v, command, args)
	case "crdb":
		// The crdb migrations are SQL only; a Go migration registered for
		// postgres would collide with their version numbers.
		goose.ResetGlobalMigrations()
		goose.SetBaseFS(crdbmigrations.Migrations)

		if err := goose.RunContext(ctx, command, db, "migrations", args...); err != nil {
			return fmt.Errorf("migrate %s: %w", command, err)
		}

		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedDBDriver, driver)
	}
}

// migratePostgres runs a migrate command on PostgreSQL through
// psqlmigrations.RunContext, which always includes Go migration 00007. Commands
// that apply or inspect migrations need an oidc provider with identity.enabled;
// create and fix only write migration files, so they need no oidc config.
func migratePostgres(ctx context.Context, db *sql.DB, v *viper.Viper, command string, args []string) error {
	var identityIssuers []string

	if !isFileCommand(command) {
		auths, err := loadAuthConfigs(v)
		if err != nil {
			return err
		}

		identityIssuers = auths.IdentityIssuers()
	}

	err := psqlmigrations.RunContext(ctx, db, identityIssuers, command, args, psqlmigrations.WithLogger(logger.Desugar()))
	if err != nil {
		return fmt.Errorf("migrate %s: %w", command, err)
	}

	return nil
}

// isFileCommand reports whether the goose command only writes migration files
// and never applies a migration.
func isFileCommand(command string) bool {
	return command == "create" || command == "fix"
}

// RunMigration applies every pending PostgreSQL migration, as the service does
// at start-up. auths are the oidc providers; migration 00007 needs exactly one
// of them with identity.enabled when it is still pending.
func RunMigration(ctx context.Context, db *sql.DB, auths configs.AuthConfigs) error {
	return migrateTo(ctx, db, auths, "RunMigration", math.MaxInt64)
}

// migrateTo applies the pending PostgreSQL migrations up to and including
// version in a span named spanName, and logs each applied migration.
func migrateTo(ctx context.Context, db *sql.DB, auths configs.AuthConfigs, spanName string, version int64) error {
	ctx, span := otel.GetTracerProvider().Tracer("github.com/metal-toolbox/governor-api/cmd").Start(ctx, spanName,
		trace.WithAttributes(attribute.Int64("migration.target_version", version)))
	defer span.End()

	p, err := psqlmigrations.NewProvider(db, auths.IdentityIssuers(), psqlmigrations.WithLogger(logger.Desugar()))
	if err != nil {
		return spanError(span, fmt.Errorf("build migration provider: %w", err))
	}

	results, err := p.UpTo(ctx, version)
	if err != nil {
		return spanError(span, fmt.Errorf("run migrations: %w", err))
	}

	for _, r := range results {
		logger.Infow("migration applied", "version", r.Source.Version, "duration", r.Duration)
	}

	return nil
}

// spanError records err on span, marks the span failed and returns err.
func spanError(span trace.Span, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())

	return err
}
