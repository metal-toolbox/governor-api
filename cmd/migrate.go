package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"time"

	_ "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5" // crdb retries and postgres interface
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"

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
		return migrate(cmd.Context(), args[0], args[1:], cmd.OutOrStdout())
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	migrateCmd.PersistentFlags().String("db-driver", "postgres", "Database driver to use for migrations (default: postgres)")
	viperBindFlag("db.driver", migrateCmd.PersistentFlags().Lookup("db-driver"))
}

func migrate(ctx context.Context, command string, args []string, out io.Writer) error {
	db, err := goose.OpenDBWithDriver("postgres", viper.GetString("db.uri"))
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}

	defer db.Close()

	switch driver := viper.GetString("db.driver"); driver {
	case "postgres":
		return migratePostgres(ctx, db, command, args, out)
	case "crdb":
		goose.SetBaseFS(crdbmigrations.Migrations)

		return runGooseCommand(ctx, db, command, args)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedDBDriver, driver)
	}
}

// migratePostgres runs a migrate command on PostgreSQL. Commands that apply or
// inspect migrations go through psqlmigrations.NewProvider, so they always
// include Go migration 00007.
func migratePostgres(ctx context.Context, db *sql.DB, command string, args []string, out io.Writer) error {
	auths, err := loadAuthConfigs(viper.GetViper())
	if err != nil {
		return err
	}

	if isFileCommand(command) {
		goose.SetBaseFS(psqlmigrations.Migrations)

		return runGooseCommand(ctx, db, command, args)
	}

	p, err := psqlmigrations.NewProvider(db, configs.IdentityIssuers(auths), psqlmigrations.WithLogger(logger.Desugar()))
	if err != nil {
		return fmt.Errorf("build migration provider: %w", err)
	}

	return runProviderCommand(ctx, p, command, args, out)
}

func runGooseCommand(ctx context.Context, db *sql.DB, command string, args []string) error {
	if err := goose.RunContext(ctx, command, db, "migrations", args...); err != nil {
		return fmt.Errorf("migrate %s: %w", command, err)
	}

	return nil
}

// isFileCommand reports whether the goose command only writes migration files
// and never applies a migration.
func isFileCommand(command string) bool {
	return command == "create" || command == "fix"
}

// migrationProvider is the part of *goose.Provider that the migrate command calls.
type migrationProvider interface {
	Up(ctx context.Context) ([]*goose.MigrationResult, error)
	UpByOne(ctx context.Context) (*goose.MigrationResult, error)
	UpTo(ctx context.Context, version int64) ([]*goose.MigrationResult, error)
	Down(ctx context.Context) (*goose.MigrationResult, error)
	DownTo(ctx context.Context, version int64) ([]*goose.MigrationResult, error)
	Status(ctx context.Context) ([]*goose.MigrationStatus, error)
	GetDBVersion(ctx context.Context) (int64, error)
}

// runProviderCommand maps a goose CLI command to the Provider call and prints
// the result to out. It returns ErrUnsupportedMigrateCommand for any command
// that is not a Provider call, including create and fix.
func runProviderCommand(ctx context.Context, p migrationProvider, command string, args []string, out io.Writer) error {
	switch command {
	case "up":
		results, err := p.Up(ctx)
		printResults(out, results...)

		return err
	case "up-by-one":
		return runOne(out, func() (*goose.MigrationResult, error) { return p.UpByOne(ctx) })
	case "up-to":
		v, err := versionArg(args)
		if err != nil {
			return err
		}

		results, err := p.UpTo(ctx, v)
		printResults(out, results...)

		return err
	case "down":
		return runOne(out, func() (*goose.MigrationResult, error) { return p.Down(ctx) })
	case "down-to":
		v, err := versionArg(args)
		if err != nil {
			return err
		}

		results, err := p.DownTo(ctx, v)
		printResults(out, results...)

		return err
	case "redo":
		if err := runOne(out, func() (*goose.MigrationResult, error) { return p.Down(ctx) }); err != nil {
			return err
		}

		return runOne(out, func() (*goose.MigrationResult, error) { return p.UpByOne(ctx) })
	case "reset":
		results, err := p.DownTo(ctx, 0)
		printResults(out, results...)

		return err
	case "status":
		statuses, err := p.Status(ctx)
		if err != nil {
			return err
		}

		printStatus(out, statuses)

		return nil
	case "version":
		v, err := p.GetDBVersion(ctx)
		if err != nil {
			return err
		}

		fmt.Fprintf(out, "version %d\n", v)

		return nil
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedMigrateCommand, command)
	}
}

func runOne(out io.Writer, fn func() (*goose.MigrationResult, error)) error {
	result, err := fn()
	if result != nil {
		printResults(out, result)
	}

	return err
}

func versionArg(args []string) (int64, error) {
	if len(args) == 0 {
		return 0, ErrMigrateVersionRequired
	}

	v, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse VERSION: %w", err)
	}

	return v, nil
}

func printResults(out io.Writer, results ...*goose.MigrationResult) {
	for _, r := range results {
		fmt.Fprintln(out, r)
	}
}

// printStatus prints one line per migration: version, state and applied time,
// or "-" for a migration that has not been applied.
func printStatus(out io.Writer, statuses []*goose.MigrationStatus) {
	for _, s := range statuses {
		applied := "-"
		if !s.AppliedAt.IsZero() {
			applied = s.AppliedAt.UTC().Format(time.RFC3339)
		}

		fmt.Fprintf(out, "%d\t%s\t%s\n", s.Source.Version, s.State, applied)
	}
}

// RunMigration applies every pending PostgreSQL migration, as the service does
// at start-up. auths are the oidc providers; migration 00007 needs exactly one
// of them with identity.enabled when it is still pending.
func RunMigration(ctx context.Context, db *sql.DB, auths []configs.Auth) error {
	ctx, span := otel.GetTracerProvider().Tracer("github.com/metal-toolbox/governor-api/cmd").Start(ctx, "RunMigration")
	defer span.End()

	p, err := psqlmigrations.NewProvider(db, configs.IdentityIssuers(auths), psqlmigrations.WithLogger(logger.Desugar()))
	if err != nil {
		return fmt.Errorf("build migration provider: %w", err)
	}

	results, err := p.Up(ctx)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	for _, r := range results {
		logger.Infow("migration applied", "version", r.Source.Version, "duration", r.Duration)
	}

	return nil
}
