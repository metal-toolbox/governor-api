package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx" // crdb retries and postgres interface
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/metal-toolbox/governor-api/internal/backupper"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var backupCMD = &cobra.Command{
	Use:   "backup",
	Short: "Backup the system",
	Long:  `Backup governor system data`,
	Run: func(cmd *cobra.Command, args []string) {
		conn := viper.GetString("db.uri")
		db := newBackupDB(conn)
		defer db.Close()

		// Perform backup operations using the db connection
		b := backupper.NewBackupper(db, backupper.WithLogger(logger.Desugar()))

		backup, err := b.BackupCRDB(cmd.Context())
		if err != nil {
			logger.Fatal("failed to backup governor", zap.Error(err))
		}

		j, err := json.MarshalIndent(backup, "", "  ")
		if err != nil {
			logger.Fatalw("failed marshaling application types", "error", err)
		}

		fmt.Println(string(j))
	},
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
}
