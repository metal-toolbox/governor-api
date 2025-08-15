package cmd

import (
	"context"
	"database/sql"

	"github.com/XSAM/otelsql"
	_ "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx" // crdb retries and postgres interface
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	// Import postgres driver. Needed since cockroach-go stopped importing it in v2.2.10
	"github.com/lib/pq"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func initTracingAndDB(ctx context.Context) *sqlx.DB {
	dbDriverName := "postgres"

	connector, err := pq.NewConnector(viper.GetString("db.uri"))
	if err != nil {
		logger.Fatalw("failed initializing sql connector", "error", err)
	}

	var innerDB *sql.DB

	if viper.GetBool("tracing.enabled") {
		initTracer()

		spanOptions := otelsql.SpanOptions{
			Ping: true,
		}

		innerDB = otelsql.OpenDB(
			connector,
			otelsql.WithAttributes(
				semconv.DBSystemPostgreSQL,
			),
			otelsql.WithSpanOptions(
				spanOptions,
			),
		)
	} else {
		innerDB = sql.OpenDB(connector)
	}

	if viper.GetBool("debug") {
		if viper.GetString("db.uri") == "postgresql://root@localhost:26257/governor?sslmode=disable" {
			logger.Debug("Using the default database connection string")
		}
	}

	db := sqlx.NewDb(innerDB, dbDriverName)

	if err := db.PingContext(ctx); err != nil {
		logger.Fatalw("failed verifying database connection", "error", err)
	}

	db.SetMaxOpenConns(viper.GetInt("db.connections.max_open"))
	db.SetMaxIdleConns(viper.GetInt("db.connections.max_idle"))
	db.SetConnMaxIdleTime(viper.GetDuration("db.connections.max_lifetime"))

	collector := collectors.NewDBStatsCollector(db.DB, "governor")

	if err := prometheus.Register(collector); err != nil {
		logger.Fatalw("failed initializing prometheus collector", "error", err)
	}

	return db
}

// initTracer returns an OpenTelemetry TracerProvider.
func initTracer() *tracesdk.TracerProvider {
	exp, err := otlptrace.New(context.Background(), otlptracegrpc.NewClient())
	if err != nil {
		logger.Fatalw("failed to initialize tracing exporter", "error", err)
	}

	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in an Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("governor"),
			attribute.String("environment", viper.GetString("tracing.environment")),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp
}
