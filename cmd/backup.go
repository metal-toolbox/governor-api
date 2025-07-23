package cmd

import (
	"database/sql"
	"io"
	"os"

	_ "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx" // crdb retries and postgres interface
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/metal-toolbox/governor-api/internal/backupper"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	backupCMD = &cobra.Command{
		Use:   "backup",
		Short: "Backup the system",
		Long:  `Backup governor system data`,
		Run: func(cmd *cobra.Command, _ []string) {
			driverstr := viper.GetString("backup.driver")
			conn := viper.GetString("db.uri")

			db := newBackupDB(conn)
			defer db.Close()

			driver := backupper.DBDriverCRDB
			if driverstr == "postgres" {
				driver = backupper.DBDriverPostgres
			}

			// Perform backup operations using the db connection
			b := backupper.New(db, driver, backupper.WithLogger(logger.Desugar()))

			err := b.Backup(cmd.Context(), outputWriter())
			if err != nil {
				logger.Fatal("failed to backup governor", zap.Error(err))
			}
		},
	}

	restoreCMD = &cobra.Command{
		Use:   "restore",
		Short: "Restore the system",
		Long:  `Restore governor system data`,
		Run: func(cmd *cobra.Command, _ []string) {
			driverstr := viper.GetString("restore.driver")
			conn := viper.GetString("db.uri")

			db := newBackupDB(conn)
			defer db.Close()

			driver := backupper.DBDriverCRDB
			if driverstr == "postgres" {
				driver = backupper.DBDriverPostgres
			}

			if viper.GetBool("restore.migrate") {
				RunMigration(db.DB)
			}

			// Perform restore operations using the db connection
			r := backupper.New(db, driver, backupper.WithLogger(logger.Desugar()))

			err := r.Restore(cmd.Context(), inputReader())
			if err != nil {
				logger.Fatal("failed to restore governor", zap.Error(err))
			}
		},
	}
)

func outputWriter() io.Writer {
	if viper.GetString("backup.output") == "stdout" {
		return os.Stdout
	}

	file, err := os.Create(viper.GetString("backup.output"))
	if err != nil {
		logger.Fatalw("failed to create backup output file", "error", err)
	}

	return file
}

func inputReader() io.Reader {
	if viper.GetString("restore.input") == "stdin" {
		return os.Stdin
	}

	file, err := os.Open(viper.GetString("restore.input"))
	if err != nil {
		logger.Fatalw("failed to open restore input file", "error", err)
	}

	return file
}

func newBackupDB(conn string) *sqlx.DB {
	connector, err := pq.NewConnector(conn)
	if err != nil {
		logger.Fatalw("failed initializing sql connector", "error", err)
	}

	innerDB := sql.OpenDB(connector)
	db := sqlx.NewDb(innerDB, "postgres")

	if err := db.Ping(); err != nil {
		logger.Fatalw("failed verifying database connection", "error", err)
	}

	return db
}

func init() {
	rootCmd.AddCommand(backupCMD)
	rootCmd.AddCommand(restoreCMD)

	backupCMD.Flags().String("driver", "crdb", "Database driver to use for backup")
	viperBindFlag("backup.driver", backupCMD.Flags().Lookup("driver"))
	backupCMD.Flags().String("output", "stdout", "Output destination for backup")
	viperBindFlag("backup.output", backupCMD.Flags().Lookup("output"))

	restoreCMD.Flags().String("driver", "crdb", "Database driver to use for restore")
	viperBindFlag("restore.driver", restoreCMD.Flags().Lookup("driver"))
	restoreCMD.Flags().String("input", "stdin", "Input source for restore")
	viperBindFlag("restore.input", restoreCMD.Flags().Lookup("input"))
	restoreCMD.Flags().Bool("migrate", false, "Enable migration during restore")
	viperBindFlag("restore.migrate", restoreCMD.Flags().Lookup("migrate"))
}
