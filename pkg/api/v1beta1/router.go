package v1beta1

import (
	"io"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/metal-toolbox/auditevent/ginaudit"
	"go.hollow.sh/toolbox/ginauth"
	"go.hollow.sh/toolbox/ginjwt"

	"github.com/metal-toolbox/governor-api/internal/eventbus"
)

const (
	// Version is the API version constant
	Version = "v1beta1"
)

// Router is the API router
type Router struct {
	AdminGroups    []string
	AuditLogWriter io.Writer
	AuditMW        *ginaudit.Middleware
	AuthMW         *ginauth.MultiTokenMiddleware
	AuthConf       []ginjwt.AuthConfig
	DB             *sqlx.DB
	EventBus       *eventbus.Client
	Logger         *zap.Logger
}

// Routes sets up protected routes and sets the scopes for said routes
func (r *Router) Routes(rg *gin.RouterGroup) {
	rg.GET(
		"/users",
		r.AuditMW.AuditWithType("ListUsers"),
		r.AuthMW.AuthRequired(readScopesWithOpenID("governor:users")),
		r.listUsers,
	)
}

func contains(list []string, item string) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}

	return false
}

// readScopesWithOpenID returns the openid scope in addition to the standard governor read scopes
func readScopesWithOpenID(sc string) []string {
	return append(ginjwt.ReadScopes(sc), oidcScope)
}
