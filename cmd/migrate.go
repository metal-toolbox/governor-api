package cmd

import (
	"database/sql"

	_ "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx" // crdb retries and postgres interface
	// Import postgres driver. Needed since cockroach-go stopped importing it in v2.2.10
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	dbm "github.com/metal-toolbox/governor-api/db"
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
	Run: func(_ *cobra.Command, args []string) {
		migrate(args[0], args[1:])
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

func migrate(command string, args []string) {
	db, err := goose.OpenDBWithDriver("postgres", viper.GetString("db.uri"))
	if err != nil {
		logger.Fatalw("failed to open DB", "error", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			logger.Fatalw("failed to close DB", "error", err)
		}
	}()

	goose.SetBaseFS(dbm.Migrations)

	if err := goose.Run(command, db, "migrations", args...); err != nil {
		logger.Fatalw("migrate command failed", "command", command, "error", err)
	}
}

// RunMigration is meant to assist when manually running a migration or when the migration is embedded.
func RunMigration(db *sql.DB) {
	goose.SetBaseFS(dbm.Migrations)

	if err := goose.Up(db, "migrations"); err != nil {
		logger.Fatalw("migration failed", "error", err)
	}
}
