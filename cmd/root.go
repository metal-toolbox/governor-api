// Package cmd is our cobra/viper cli implementation
package cmd

import (
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const appName = "governor-api"

var (
	cfgFile string
	logger  *zap.SugaredLogger
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "governor",
	Short: "Governs IAM and IDP",
	Long:  `Governor is a microservice that allows management of users, groups, and applications for a consistent IAM/IDP experience across providers`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.governor.yaml)")

	rootCmd.PersistentFlags().Bool("debug", false, "enable debug logging")
	viperBindFlag("logging.debug", rootCmd.PersistentFlags().Lookup("debug"))

	rootCmd.PersistentFlags().Bool("pretty", false, "enable pretty (human readable) logging output")
	viperBindFlag("logging.pretty", rootCmd.PersistentFlags().Lookup("pretty"))

	rootCmd.PersistentFlags().String("db-uri", "postgresql://root@localhost:26257/governor?sslmode=disable", "URI for database connection")
	viperBindFlag("db.uri", rootCmd.PersistentFlags().Lookup("db-uri"))

	rootCmd.PersistentFlags().String("audit-log-path", "/app-audit/audit.log", "file path to write audit logs to.")
	viperBindFlag("audit.log-path", rootCmd.PersistentFlags().Lookup("audit-log-path"))

	rootCmd.PersistentFlags().Bool("development", false, "enable development settings")
	viperBindFlag("development", rootCmd.PersistentFlags().Lookup("development"))

	// NATS flags
	rootCmd.PersistentFlags().String("nats-url", "nats://127.0.0.1:4222", "NATS server connection url")
	viperBindFlag("nats.url", rootCmd.PersistentFlags().Lookup("nats-url"))

	rootCmd.PersistentFlags().String("nats-creds-file", "", "Path to the file containing the NATS credentials file")
	viperBindFlag("nats.creds-file", rootCmd.PersistentFlags().Lookup("nats-creds-file"))

	rootCmd.PersistentFlags().String("nats-subject-prefix", "governor.events", "prefix for NATS subjects")
	viperBindFlag("nats.subject-prefix", rootCmd.PersistentFlags().Lookup("nats-subject-prefix"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".governor" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".governor")
	}

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.SetEnvPrefix("governor")
	viper.AutomaticEnv() // read in environment variables that match

	err := viper.ReadInConfig()

	setupLogging()

	if err == nil {
		logger.Infow("using config file", "file", viper.ConfigFileUsed())
	}
}

func setupLogging() {
	cfg := zap.NewProductionConfig()
	if viper.GetBool("logging.pretty") {
		cfg = zap.NewDevelopmentConfig()
	}

	if viper.GetBool("logging.debug") {
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	l, err := cfg.Build()
	if err != nil {
		panic(err)
	}

	logger = l.Sugar().With("app", appName)
	defer logger.Sync() //nolint:errcheck
}

// viperBindFlag provides a wrapper around the viper bindings that handles error checks
func viperBindFlag(name string, flag *pflag.Flag) {
	err := viper.BindPFlag(name, flag)
	if err != nil {
		panic(err)
	}
}
