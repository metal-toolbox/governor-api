// Package api provides an http server for governor
package api

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/auditevent/ginaudit"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"go.hollow.sh/toolbox/ginauth"
	"go.hollow.sh/toolbox/ginjwt"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/internal/eventbus"
	v1alpha "github.com/metal-toolbox/governor-api/pkg/api/v1alpha1"
	v1beta "github.com/metal-toolbox/governor-api/pkg/api/v1beta1"
)

var (
	readTimeout  = 10 * time.Second
	writeTimeout = 20 * time.Second
	corsMaxAge   = 12 * time.Hour
)

// Conf allows other packages to compose their api configuration and use our NewAPI factor to put it together for them
type Conf struct {
	AdminGroups []string
	AuthConf    []ginjwt.AuthConfig
	Debug       bool
	Listen      string
	Logger      *zap.Logger
}

// Server holds data necessary to run the API and has associated methods
type Server struct {
	AuthMW         *ginauth.MultiTokenMiddleware
	Conf           *Conf
	DB             *sqlx.DB
	Router         *gin.Engine
	AuditLogWriter io.Writer
	aumdw          *ginaudit.Middleware
	EventBus       *eventbus.Client
}

func (s *Server) setupRoutes(router *gin.Engine) {
	s.Conf.Logger.Sugar().Info("Setting up routes")

	v1alphaRtr := v1alpha.Router{
		AdminGroups: s.Conf.AdminGroups,
		AuthMW:      s.AuthMW,
		AuditMW:     s.aumdw,
		AuthConf:    s.Conf.AuthConf,
		Logger:      s.Conf.Logger,
		DB:          s.DB,
		EventBus:    s.EventBus,
	}

	v1alpha1 := router.Group("/api/v1alpha1")
	v1alphaRtr.Routes(v1alpha1)

	v1betaRtr := v1beta.Router{
		AdminGroups: s.Conf.AdminGroups,
		AuthMW:      s.AuthMW,
		AuditMW:     s.aumdw,
		AuthConf:    s.Conf.AuthConf,
		Logger:      s.Conf.Logger,
		DB:          s.DB,
		EventBus:    s.EventBus,
	}

	v1beta1 := router.Group("/api/v1beta1")
	v1betaRtr.Routes(v1beta1)
}

// setup builds our api router and expects that an AuthConf exists on the api.Conf object
func (s *Server) setup() *gin.Engine {
	router := gin.New()

	s.Conf.Logger.Sugar().Info("Setting up AuditLogWriter")
	s.aumdw = ginaudit.NewJSONMiddleware("governor-api", s.AuditLogWriter)

	router.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		AllowAllOrigins:  true,
		AllowCredentials: true,
		MaxAge:           corsMaxAge,
	}))

	s.Conf.Logger.Sugar().Info("Setting up auth middleware")

	authMW, err := ginjwt.NewMultiTokenMiddlewareFromConfigs(s.Conf.AuthConf...)
	if err != nil {
		s.Conf.Logger.Sugar().Fatal("failed to initialize auth middleware", "error", err)
	}

	s.AuthMW = authMW

	s.Conf.Logger.Sugar().Info("Setting up prometheus")

	prom := ginprometheus.NewPrometheus("gin")

	// Remove any params from the URL string to keep the number of labels down
	prom.ReqCntURLLabelMappingFn = func(c *gin.Context) string {
		return c.FullPath()
	}

	prom.Use(router)

	customLogger := s.Conf.Logger.With(zap.String("component", "api"))
	router.Use(
		ginzap.GinzapWithConfig(customLogger, &ginzap.Config{
			TimeFormat: time.RFC3339,
			SkipPaths:  []string{"/healthz", "/healthz/readiness", "/healthz/liveness"},
			UTC:        true,
		}),
	)

	router.Use(ginzap.RecoveryWithZap(s.Conf.Logger.With(zap.String("component", "api")), true))

	tp := otel.GetTracerProvider()
	if tp != nil {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}

		router.Use(otelgin.Middleware(hostname, otelgin.WithTracerProvider(tp)))
	}

	s.Conf.Logger.Sugar().Info("Setting up healthz endpoints")

	// Health endpoints
	router.GET("/healthz", s.livenessCheck)
	router.GET("/healthz/liveness", s.livenessCheck)
	router.GET("/healthz/readiness", s.readinessCheck)

	s.setupRoutes(router)

	return router
}

// NewAPI returns an http Server constructed from an api.Server object
func (s *Server) NewAPI() *http.Server {
	if s.Conf == nil {
		s.Conf = &Conf{}
	}

	if s.Conf.Logger == nil {
		s.Conf.Logger = zap.NewNop()
	}

	if s.Router == nil {
		s.Router = s.setup()
	}

	return &http.Server{
		Handler:      s.Router,
		Addr:         s.Conf.Listen,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
}

// Run will start the server listening on the specified address
func (s *Server) Run() error {
	if !s.Conf.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	return s.setup().Run(s.Conf.Listen)
}
