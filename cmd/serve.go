package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	audithelpers "github.com/metal-toolbox/auditevent/helpers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.hollow.sh/toolbox/ginjwt"

	"github.com/metal-toolbox/governor-api/internal/api"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/eventbus"
	"github.com/metal-toolbox/governor-api/pkg/configs"
)

// serveCmd invokes the governor api
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "starts the governor api server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return startAPI(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().String("listen", "0.0.0.0:3001", "address to listen on")
	viperBindFlag("api.listen", serveCmd.Flags().Lookup("listen"))

	serveCmd.Flags().StringSlice("admin-groups", []string{"delivery-engineering"}, "The slug of the groups that have admin functions")
	viperBindFlag("admin-groups", serveCmd.Flags().Lookup("admin-groups"))

	ginjwt.RegisterViperOIDCFlags(viper.GetViper(), serveCmd)
}

func startAPI(ctx context.Context) error {
	logger.Debug("initializing tracer and database")

	db := initTracingAndDB(ctx)

	dbtools.RegisterHooks()

	// Run the embedded migration in the event that this is the first run or first run since a new migration was added.
	RunMigration(db.DB)

	// NOTE: oidc config only works when loading from config file, not env variables,
	// since GetAuthConfigsFromFlags expects a slice of oidc structs
	authcfgs, err := ginjwt.GetAuthConfigsFromFlags(viper.GetViper())
	if err != nil {
		logger.Fatalw("failed getting JWT configurations", "error", err)
	}

	// we shouldn't continue if no oidc configs were provided, since that
	// will allow any unauthenticated requests to succeed in the auth middleware
	if len(authcfgs) == 0 {
		logger.Fatalln("no oidc auth configs found")
	}

	logger.Debugf("loaded %d oidc config(s)", len(authcfgs))

	for _, ac := range authcfgs {
		logger.Infow(
			"OIDC Config",
			"Enabled", ac.Enabled,
			"Audience", ac.Audience,
			"Issuer", ac.Issuer,
			"JWKSURI", ac.JWKSURI,
			"RolesClaim", ac.RolesClaim,
			"UsernameClaim", ac.UsernameClaim,
		)
	}

	adminGroups := viper.GetStringSlice("admin-groups")
	if len(adminGroups) == 0 {
		logger.Warn("No admin groups specified!")
	} else {
		logger.Infof("using admin group(s): %v", adminGroups)
	}

	conf := &api.Conf{
		AdminGroups: adminGroups,
		AuthConf:    authcfgs,
		Debug:       viper.GetBool("logging.debug"),
		Listen:      viper.GetString("api.listen"),
		Logger:      logger.Desugar(),
	}

	auditpath := viper.GetString("audit.log-path")

	if auditpath == "" {
		return errors.New("failed starting server. Audit log file path can't be empty") //nolint:err113
	}

	if conf.Debug {
		logger.Debugf("polling for audit log at: %s", auditpath)

		_, err := os.Stat(auditpath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				logger.Debug("audit path file does not exist, this will cause the process to never become available")
				logger.Debug("check that the log file is being created")
			} else {
				logger.Debugf("failed to stat audit log: %s", err.Error())
			}
		} else {
			logger.Debug("audit file exists")
		}
	}

	// WARNING(jaosorior): This will block until the file is available;
	// make sure an initContainer creates the file
	auf, auerr := audithelpers.OpenAuditLogFileUntilSuccess(auditpath)
	if auerr != nil {
		return fmt.Errorf("couldn't open audit file. error: %s", auerr) //nolint:err113
	}
	defer auf.Close()

	logger.Debugw("intializing nats connection",
		"nats.url", viper.GetString("nats.url"),
		"nats.nkey", viper.GetString("nats.nkey"),
		"nats.subject-prefix", viper.GetString("nats.subject-prefix"),
	)

	nc, err := appConfig.NATSConn(ctx, appName, configs.WithLogger(logger.Desugar()))
	if err != nil {
		return err
	}

	defer nc.Close()

	eb := eventbus.NewClient(
		eventbus.WithLogger(logger.Desugar()),
		eventbus.WithNATSConn(nc),
		eventbus.WithNATSPrefix(viper.GetString("nats.subject-prefix")),
	)

	logger.Debug("building api server and router")

	apiServer := &api.Server{
		AuditLogWriter: auf,
		Conf:           conf,
		DB:             db,
		EventBus:       eb,
	}

	return apiServer.Run()
}
