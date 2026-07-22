package authz

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/hollow-toolbox/ginauth"
)

// gin context keys populated on a successful verification. These mirror the
// keys set by hollow-toolbox/ginjwt so downstream middleware and handlers read
// identity the same way regardless of which verifier succeeded.
const (
	contextKeySubject = "jwt.subject"
	contextKeyUser    = "jwt.user"
	contextKeyRoles   = "jwt.roles"
)

// oidcScope is the scope that distinguishes user (OIDC) tokens from workload
// tokens; it never appears in an authorization action.
const oidcScope = "openid"

// TokenVerifier authenticates a request token and returns its claims. It is
// satisfied by *ginjwt.Middleware (via VerifyToken) and stubbed in tests.
type TokenVerifier interface {
	VerifyToken(*gin.Context) (ginauth.ClaimMetadata, error)
}

// Verifier is a ginauth.GenericAuthMiddleware that authenticates a token with a
// TokenVerifier and then authorizes it with an external Decider, in place of
// the usual scope check. It authenticates against a single issuer only, so
// tokens from other issuers fail authentication fast and return before any
// Decider evaluation.
type Verifier struct {
	jwt     TokenVerifier
	decider Decider
}

// Verifier implements [ginauth.GenericAuthMiddleware]
var _ ginauth.GenericAuthMiddleware = (*Verifier)(nil)

// NewVerifier returns a Verifier that authenticates with jwt (configured for a
// single issuer) and authorizes with decider.
func NewVerifier(jwt TokenVerifier, decider Decider) ginauth.GenericAuthMiddleware {
	return &Verifier{jwt: jwt, decider: decider}
}

// VerifyTokenWithScopes authenticates the token against its issuer, then
// consults the Decider for the most specific scope required by the route. On
// success it populates the identity context keys itself — the
// MultiTokenMiddleware only calls SetMetadata on the error path.
func (v *Verifier) VerifyTokenWithScopes(c *gin.Context, scopes []string) (ginauth.ClaimMetadata, error) {
	cm, err := v.jwt.VerifyToken(c)
	if err != nil {
		return cm, err
	}

	allow, err := v.decider.Eval(c.Request.Context(), AuthzRequest{
		Principal: cm.Subject,
		Scope:     mostSpecificScope(scopes),
	})
	if err != nil {
		// Decider unavailable/errored: this verifier fails, but a scoped token
		// still wins via its own ginjwt verifier (first-success-wins).
		return cm, err
	}

	if !allow {
		return cm, ginauth.NewAuthorizationError("not permitted by policy")
	}

	v.setContext(c, cm)

	return cm, nil
}

// SetMetadata is called by the MultiTokenMiddleware only on the error path, so
// cm is the (typically empty) claims of a token this verifier failed to verify.
// The MultiTokenMiddleware runs every verifier concurrently against a shared
// gin.Context, so this must not clobber identity a sibling verifier already set
// on success. Mirror ginjwt's defensive semantics: write subject/user only when
// non-empty and never touch roles here. The success path populates context via
// setContext instead.
func (v *Verifier) SetMetadata(c *gin.Context, cm ginauth.ClaimMetadata) {
	if cm.Subject != "" {
		c.Set(contextKeySubject, cm.Subject)
	}

	if cm.User != "" {
		c.Set(contextKeyUser, cm.User)
	}
}

func (v *Verifier) setContext(c *gin.Context, cm ginauth.ClaimMetadata) {
	c.Set(contextKeySubject, cm.Subject)
	c.Set(contextKeyUser, cm.User)
	c.Set(contextKeyRoles, cm.Roles)
}

// mostSpecificScope returns the single resource-qualified scope a route
// requires — the one entry containing a colon that is not the openid scope
// (e.g. "create:governor:users" out of ["write","create","create:governor:users","openid"]).
// It returns "" when there is none, which the Decider treats as an unknown
// action and denies.
func mostSpecificScope(scopes []string) string {
	for _, s := range scopes {
		if s == oidcScope {
			continue
		}

		if strings.Contains(s, ":") {
			return s
		}
	}

	return ""
}
