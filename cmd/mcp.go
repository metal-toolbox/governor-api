package cmd

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/metal-toolbox/governor-api/pkg/mcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.hollow.sh/toolbox/ginjwt"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

const (
	defaultMCPGovernorRequestTimeout = 10 * time.Second
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "starts governor MCP server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return startMCPServer(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)

	// MCP server flags
	mcpCmd.Flags().String("listen", "0.0.0.0:3001", "sse server listens on")
	viperBindFlag("mcp.listen", mcpCmd.Flags().Lookup("listen"))
	mcpCmd.Flags().String("metadata-base-url", "http://localhost:3001", "base URL for MCP metadata")
	viperBindFlag("mcp.metadata-base-url", mcpCmd.Flags().Lookup("metadata-base-url"))

	// Governor flags
	mcpCmd.Flags().String("governor-url", "https://api.iam.equinixmetal.net", "url of the governor api")
	viperBindFlag("governor.url", mcpCmd.Flags().Lookup("governor-url"))
	mcpCmd.Flags().Duration("governor-timeout", defaultMCPGovernorRequestTimeout, "timeout for requests to governor api")
	viperBindFlag("governor.timeout", mcpCmd.Flags().Lookup("governor-timeout"))

	ginjwt.RegisterViperOIDCFlags(viper.GetViper(), mcpCmd)
}

func startMCPServer(ctx context.Context) error {
	logger := logger.Desugar()
	logger.Info("starting MCP server")

	if viper.GetBool("tracing.enabled") {
		initTracer()
	}

	tracer := otel.GetTracerProvider().Tracer("governor-api/mcp")

	authcfgs, err := ginjwt.GetAuthConfigsFromFlags(viper.GetViper())
	if err != nil {
		logger.Fatal("failed getting JWT configurations", zap.Error(err))
	}

	if len(authcfgs) == 0 {
		logger.Fatal("no oidc auth configs found")
	}

	logger.Debug("loaded oidc config(s)", zap.Int("count", len(authcfgs)))

	for _, ac := range authcfgs {
		logger.Info(
			"OIDC Config",
			zap.Bool("enabled", ac.Enabled),
			zap.String("audience", ac.Audience),
			zap.String("issuer", ac.Issuer),
			zap.String("jwksuri", ac.JWKSURI),
			zap.String("roles", ac.RolesClaim),
			zap.String("username", ac.UsernameClaim),
		)
	}

	httpclient := &http.Client{
		Timeout:   viper.GetDuration("governor.timeout"),
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	mcpserver := mcp.NewGovernorMCPServer(
		&http.Server{Addr: viper.GetString("mcp.listen")},
		viper.GetString("governor.url"),
		mcp.WithLogger(logger),
		mcp.WithTracer(tracer),
		mcp.WithAuthConfigs(authcfgs),
		mcp.WithMetadataBaseURL(viper.GetString("mcp.metadata-base-url")),
		mcp.WithHTTPClient(httpclient),
	)

	go func() {
		if err := mcpserver.Start(); err != nil {
			logger.Fatal("MCP server failed: ", zap.Error(err))
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)
	signal.Notify(sig, syscall.SIGTERM)

	s := <-sig

	logger.Debug("received shutdown signal", zap.Any("signal", s))

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if err := mcpserver.Shutdown(ctx); err != nil {
		logger.Fatal("failed to shutdown MCP server: ", zap.Error(err))
	}

	logger.Info("bye")

	return nil
}
