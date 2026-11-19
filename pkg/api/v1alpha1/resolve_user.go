package v1alpha1

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/metal-toolbox/hollow-toolbox/ginjwt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/metal-toolbox/governor-api/internal/auth"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/metrics"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
	"github.com/metal-toolbox/governor-api/pkg/configs"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

const (
	// userInfoTimeout bounds the OIDC discovery and userinfo calls of a first
	// sign-in. It stays well below the 20 s server write timeout.
	userInfoTimeout = 5 * time.Second
	// resolveUserDBTimeout bounds the transaction that registers a first
	// sign-in, so that database/sql cancels a query the server-side timeouts
	// below do not end.
	resolveUserDBTimeout = 5 * time.Second

	setLockTimeout      = `SET LOCAL lock_timeout = '2s'`
	setStatementTimeout = `SET LOCAL statement_timeout = '5s'`
)

// provisioned is the user that a first sign-in linked or created, with the
// audit event that records it. outcome is OutcomeHit, with no event, when a
// concurrent sign-in had already linked the same identity.
type provisioned struct {
	user    *models.User
	outcome metrics.Outcome
	event   *models.AuditEvent
	action  string
}

// authConfigFor returns the first configured provider whose issuer is iss.
// configs.ValidateIdentityProviders rejects at startup any config whose
// entries for one issuer have different identity settings, so the first entry
// stands for all of them.
func (r *Router) authConfigFor(iss string) (configs.Auth, bool) {
	for _, ac := range r.AuthConf {
		if ac.Issuer == iss {
			return ac, true
		}
	}

	return configs.Auth{}, false
}

// resolveUser returns the governor user of the signed-in caller, found by the
// verified issuer and the username claim of the token. mods apply to the users
// query. When autoProvision is true, an unknown sign-in from a provider with
// identity.enabled may create a user. When ok is false, resolveUser has
// already sent the error response.
func (r *Router) resolveUser(c *gin.Context, autoProvision bool, mods ...qm.QueryMod) (user *models.User, ok bool) {
	ctx, span := tracer.Start(c.Request.Context(), "resolveUser")
	defer span.End()

	start := time.Now()

	sub := c.GetString("jwt.user")
	if sub == "" {
		sendError(c, http.StatusUnauthorized, "missing jwt user")
		return nil, false
	}

	iss := ginjwt.GetIssuer(c)
	if iss == "" {
		sendError(c, http.StatusUnauthorized, "missing jwt issuer")
		return nil, false
	}

	span.SetAttributes(attribute.String("iss", iss))

	ac, ok := r.authConfigFor(iss)
	if !ok {
		sendError(c, http.StatusUnauthorized, "unknown issuer")
		return nil, false
	}

	observe := func(o metrics.Outcome) {
		span.SetAttributes(attribute.String("outcome", string(o)))
		r.UserResolution.Observe(iss, o, time.Since(start))
	}

	user, err := dbtools.FindUserByExternalID(ctx, r.DB, iss, sub, mods...)
	if err == nil {
		observe(metrics.OutcomeHit)
		return user, true
	}

	if !errors.Is(err, sql.ErrNoRows) {
		observe(metrics.OutcomeError)
		span.SetStatus(codes.Error, err.Error())
		sendError(c, http.StatusInternalServerError, "error getting user: "+err.Error())

		return nil, false
	}

	if !autoProvision {
		observe(metrics.OutcomeRejectedUnregistered)
		sendError(c, http.StatusUnauthorized, "user not found")

		return nil, false
	}

	// No provider setting lets this sign-in create or link a user, so the
	// userinfo call is skipped.
	if !mayProvision(ac.Identity) {
		observe(metrics.OutcomeRejectedUnregistered)
		sendError(c, http.StatusUnauthorized, "user not registered")

		return nil, false
	}

	r.Logger.Debug("user not found in db, trying to link or register",
		zap.String("iss", iss),
		zap.String("email", c.GetString("jwt.subject")),
		zap.String("external_id", sub),
	)

	userInfo, ok := r.verifiedUserInfo(ctx, c, sub)
	if !ok {
		observe(metrics.OutcomeError)
		return nil, false
	}

	p, err := r.provisionUser(ctx, getCtxAuditID(c), ac, sub, userInfo)
	if err != nil {
		return r.handleProvisionError(ctx, c, span, observe, iss, sub, err, mods)
	}

	observe(p.outcome)

	if p.outcome != metrics.OutcomeHit && !r.announceProvisioned(ctx, c, span, iss, p) {
		return nil, false
	}

	// Read the user back with the caller's query mods, so that it carries the
	// same relations as a user found on a later sign-in.
	user, err = dbtools.FindUserByExternalID(ctx, r.DB, iss, sub, mods...)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		sendError(c, http.StatusInternalServerError, "error getting user: "+err.Error())

		return nil, false
	}

	return user, true
}

// announceProvisioned logs a linked or registered user, adds its audit event
// to the request audit data and publishes the users event. When it returns
// false, it has already sent the error response.
func (r *Router) announceProvisioned(ctx context.Context, c *gin.Context, span trace.Span, iss string, p *provisioned) bool {
	r.logResolution(c, span, zap.InfoLevel, iss, p.user.ID, p.outcome)

	if err := updateContextWithAuditEventData(c, p.event); err != nil {
		sendError(c, http.StatusBadRequest, "error writing user audit data: "+err.Error())
		return false
	}

	if err := r.EventBus.Publish(ctx, events.GovernorUsersEventSubject, &events.Event{
		Version: events.Version,
		Action:  p.action,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: "",
		UserID:  p.user.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish user "+p.action+" event, downstream changes may be delayed "+err.Error())
		return false
	}

	return true
}

// handleProvisionError sends the response for a failed first sign-in. A
// unique violation means that a concurrent first sign-in of the same identity
// won the race, so the lookup runs once more before the request fails.
func (r *Router) handleProvisionError(
	ctx context.Context, c *gin.Context, span trace.Span, observe func(metrics.Outcome),
	iss, sub string, err error, mods []qm.QueryMod,
) (*models.User, bool) {
	if errors.Is(err, errEmailExists) || errors.Is(err, dbtools.ErrExternalIDConflict) {
		user, lookupErr := dbtools.FindUserByExternalID(ctx, r.DB, iss, sub, mods...)
		if lookupErr == nil {
			observe(metrics.OutcomeHit)
			return user, true
		}

		if !errors.Is(lookupErr, sql.ErrNoRows) {
			observe(metrics.OutcomeError)
			span.SetStatus(codes.Error, lookupErr.Error())
			sendError(c, http.StatusInternalServerError, "error getting user: "+lookupErr.Error())

			return nil, false
		}
	}

	span.SetStatus(codes.Error, err.Error())

	switch {
	case errors.Is(err, errEmailExists):
		observe(metrics.OutcomeRejectedConflict)
		r.logResolution(c, span, zap.WarnLevel, iss, "", metrics.OutcomeRejectedConflict)
		sendError(c, http.StatusConflict, "a user with this email exists; linking is disabled for this issuer")
	case errors.Is(err, errLinkedToOtherIdentity):
		observe(metrics.OutcomeRejectedConflict)
		r.logResolution(c, span, zap.WarnLevel, iss, "", metrics.OutcomeRejectedConflict)
		sendError(c, http.StatusConflict, "user already linked to a different identity for this issuer")
	case errors.Is(err, errNotRegistered):
		observe(metrics.OutcomeRejectedUnregistered)
		sendError(c, http.StatusUnauthorized, "user not registered")
	case dbtools.IsTimeout(err):
		observe(metrics.OutcomeError)
		sendError(c, http.StatusInternalServerError, "error getting user: timeout")
	default:
		observe(metrics.OutcomeError)
		sendError(c, http.StatusInternalServerError, "error getting user: "+err.Error())
	}

	return nil, false
}

// mayProvision reports whether a provider's identity settings let an unknown
// sign-in link or create a user.
func mayProvision(id configs.IdentityConfig) bool {
	return mayLink(id) || mayRegister(id)
}

// mayLink reports whether an unknown sign-in may be linked by email to an existing user.
func mayLink(id configs.IdentityConfig) bool {
	return id.Enabled && id.AutoLink
}

// mayRegister reports whether an unknown sign-in may create a user.
func mayRegister(id configs.IdentityConfig) bool {
	return id.Enabled && id.AutoRegisterOrDefault()
}

// verifiedUserInfo fetches the OIDC userinfo of the bearer token and checks it
// against the token claims. When ok is false, it has already sent the error
// response.
func (r *Router) verifiedUserInfo(ctx context.Context, c *gin.Context, sub string) (*auth.OIDCUserInfo, bool) {
	authzHeader := strings.Split(c.Request.Header.Get("Authorization"), " ")
	if len(authzHeader) != expectedAuthzHeaderParts {
		sendError(c, http.StatusUnauthorized, "invalid authorization header")
		return nil, false
	}

	infoCtx, cancel := context.WithTimeout(ctx, userInfoTimeout)
	defer cancel()

	userInfo, err := auth.UserInfoFromJWT(infoCtx, authzHeader[expectedAuthzHeaderParts-1], r.AuthConf)
	if err != nil {
		r.Logger.Debug("error retrieving userinfo", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error retrieving userinfo")

		return nil, false
	}

	if userInfo.Name == "" {
		sendError(c, http.StatusBadRequest, "invalid name from oidc userinfo")
		return nil, false
	}

	// make sure the email we got matches the email in the jwt.subject
	if userInfo.Email != c.GetString("jwt.subject") {
		sendError(c, http.StatusBadRequest, "invalid email from oidc userinfo")
		return nil, false
	}

	// make sure the subject we got matches the external_id in jwt.user
	if userInfo.Sub != sub {
		sendError(c, http.StatusBadRequest, "invalid subject from oidc userinfo")
		return nil, false
	}

	return userInfo, true
}

// provisionUser links an unknown sign-in from the provider ac to the user
// with the same email, or creates a user for it, with its (issuer, sub) record
// and audit rows, in one transaction. It returns errEmailExists when a user
// that may not be linked already has the email, errLinkedToOtherIdentity when
// that user has another identity at the issuer, errNotRegistered when nothing
// was linked and the provider may not create users, and an error matching
// dbtools.ErrExternalIDConflict when the identity was recorded concurrently.
func (r *Router) provisionUser(
	ctx context.Context, auditID string, ac configs.Auth, sub string, userInfo *auth.OIDCUserInfo,
) (*provisioned, error) {
	ctx, span := tracer.Start(ctx, "provisionUser")
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, resolveUserDBTimeout)
	defer cancel()

	// READ COMMITTED, the PostgreSQL default.
	tx, err := r.DB.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Rollback after a successful Commit returns sql.ErrTxDone and changes nothing.
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range []string{setLockTimeout, setStatementTimeout} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return nil, err
		}
	}

	pair := dbtools.ExternalIDPair{Iss: ac.Issuer, ID: sub}

	var (
		p      *provisioned
		linked bool
	)

	if mayLink(ac.Identity) {
		p, linked, err = linkUser(ctx, tx, auditID, pair, userInfo.Email)
		if err != nil {
			return nil, err
		}
	}

	if !linked {
		if !mayRegister(ac.Identity) {
			return nil, errNotRegistered
		}

		p, err = registerUser(ctx, tx, auditID, pair, userInfo)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return p, nil
}

// linkUser adds the identity pair to the active user whose email equals email,
// ignoring case. found is false when there is no such user. The user row
// stays locked until the transaction ends, so a concurrent first sign-in of
// the same identity waits here and then sees the committed record.
func linkUser(
	ctx context.Context, tx *sqlx.Tx, auditID string, pair dbtools.ExternalIDPair, email string,
) (p *provisioned, found bool, err error) {
	user, err := models.Users(
		qm.Where("lower(email) = lower(?)", email),
		qm.Where("(status IS NULL OR status <> ?)", UserStatusSuspended),
		qm.For("UPDATE"),
	).One(ctx, tx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}

	existing, err := models.UserExternalIds(
		models.UserExternalIDWhere.UserID.EQ(user.ID),
		models.UserExternalIDWhere.Iss.EQ(pair.Iss),
	).One(ctx, tx)

	switch {
	case err == nil && existing.ExternalID == pair.ID:
		return &provisioned{user: user, outcome: metrics.OutcomeHit}, true, nil
	case err == nil:
		return nil, true, errLinkedToOtherIdentity
	case !errors.Is(err, sql.ErrNoRows):
		return nil, true, err
	}

	if _, err := dbtools.AddUserExternalID(ctx, tx, user.ID, pair.Iss, pair.ID); err != nil {
		return nil, true, err
	}

	event, err := dbtools.AuditUserExternalIDAdded(ctx, tx, auditID, user, user, pair)
	if err != nil {
		return nil, true, err
	}

	return &provisioned{
		user:    user,
		outcome: metrics.OutcomeLinked,
		event:   event,
		action:  events.GovernorEventUpdate,
	}, true, nil
}

// registerUser inserts a new user for userInfo with the identity pair.
func registerUser(
	ctx context.Context, tx *sqlx.Tx, auditID string, pair dbtools.ExternalIDPair, userInfo *auth.OIDCUserInfo,
) (*provisioned, error) {
	newUser := &models.User{
		Email:       userInfo.Email,
		Name:        userInfo.Name,
		LastLoginAt: null.TimeFrom(time.Now()),
	}

	if err := newUser.Insert(ctx, tx, boil.Infer()); err != nil {
		if _, ok := dbtools.UniqueViolation(err); ok {
			return nil, errEmailExists
		}

		return nil, err
	}

	if _, err := dbtools.AddUserExternalID(ctx, tx, newUser.ID, pair.Iss, pair.ID); err != nil {
		return nil, err
	}

	event, err := dbtools.AuditUserCreatedWithActor(ctx, tx, auditID, newUser, newUser)
	if err != nil {
		return nil, err
	}

	if _, err := dbtools.AuditUserExternalIDAdded(ctx, tx, auditID, newUser, newUser, pair); err != nil {
		return nil, err
	}

	return &provisioned{
		user:    newUser,
		outcome: metrics.OutcomeRegistered,
		event:   event,
		action:  events.GovernorEventCreate,
	}, nil
}

// logResolution logs a first-sign-in outcome. External IDs and emails are
// left out: they are logged at debug level only.
func (r *Router) logResolution(c *gin.Context, span trace.Span, level zapcore.Level, iss, userID string, outcome metrics.Outcome) {
	if ce := r.Logger.Check(level, "user resolution"); ce != nil {
		ce.Write(
			zap.String("iss", iss),
			zap.String("user_id", userID),
			zap.String("outcome", string(outcome)),
			zap.String("audit_id", getCtxAuditID(c)),
			zap.String("trace_id", span.SpanContext().TraceID().String()),
		)
	}
}
