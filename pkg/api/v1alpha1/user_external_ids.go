package v1alpha1

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/gin-gonic/gin"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
)

const (
	// maxUserExternalIDs is the most identities a user request may set.
	maxUserExternalIDs = 10
	// maxIssuerLength and maxExternalIDLength match the length checks of the
	// user_external_ids table.
	maxIssuerLength     = 512
	maxExternalIDLength = 256

	// externalIDDeprecationLink is the Link header value of a response that
	// carries the deprecated external_id field.
	externalIDDeprecationLink = `<https://github.com/metal-toolbox/governor-api/blob/main/docs/multi-idp-user-identities-lld.md>; rel="deprecation"`
)

// requestedExternalIDs returns the identities that req sets, validated. ok is
// false when req sets none. The deprecated external_id becomes the one
// identity at the primary identity issuer.
func (r *Router) requestedExternalIDs(req *UserReq) (pairs []dbtools.ExternalIDPair, ok bool, err error) {
	switch {
	case req.ExternalID != "" && req.ExternalIDs != nil:
		return nil, false, errBothExternalIDFields
	case req.ExternalID != "":
		if r.PrimaryIdentityIssuer == "" {
			return nil, false, errNoPrimaryIdentityIssuer
		}

		pairs = []dbtools.ExternalIDPair{{Iss: r.PrimaryIdentityIssuer, ID: req.ExternalID}}
	case req.ExternalIDs != nil:
		pairs = *req.ExternalIDs
	default:
		return nil, false, nil
	}

	if err := validateExternalIDs(pairs, r.configuredIssuers()); err != nil {
		return nil, false, err
	}

	return pairs, true, nil
}

// configuredIssuers returns the issuers of the configured oidc providers.
func (r *Router) configuredIssuers() []string {
	issuers := make([]string, 0, len(r.AuthConf))
	for _, ac := range r.AuthConf {
		issuers = append(issuers, ac.Issuer)
	}

	return issuers
}

// allConfiguredIssuers reports whether every issuer in issuers is the issuer
// of a configured oidc provider.
func (r *Router) allConfiguredIssuers(issuers []string) bool {
	configured := r.configuredIssuers()

	for _, iss := range issuers {
		if !slices.Contains(configured, iss) {
			return false
		}
	}

	return true
}

// validateExternalIDs returns an error matching errInvalidExternalIDs when
// pairs has more than maxUserExternalIDs entries, an issuer twice, or an entry
// whose issuer is not one of issuers or whose ID is not a valid external ID.
func validateExternalIDs(pairs []dbtools.ExternalIDPair, issuers []string) error {
	if len(pairs) > maxUserExternalIDs {
		return fmt.Errorf("%w: at most %d entries are allowed", errInvalidExternalIDs, maxUserExternalIDs)
	}

	seen := make(map[string]struct{}, len(pairs))

	for _, p := range pairs {
		if utf8.RuneCountInString(p.Iss) > maxIssuerLength {
			return fmt.Errorf("%w: iss is longer than %d characters", errInvalidExternalIDs, maxIssuerLength)
		}

		if u, err := url.Parse(p.Iss); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%w: iss %s is not an absolute http or https URL", errInvalidExternalIDs, p.Iss)
		}

		if !slices.Contains(issuers, p.Iss) {
			return fmt.Errorf("%w: iss %s is not a configured issuer", errInvalidExternalIDs, p.Iss)
		}

		if _, ok := seen[p.Iss]; ok {
			return fmt.Errorf("%w: iss %s appears more than once", errInvalidExternalIDs, p.Iss)
		}

		seen[p.Iss] = struct{}{}

		if n := utf8.RuneCountInString(p.ID); n == 0 || n > maxExternalIDLength {
			return fmt.Errorf("%w: id for iss %s must be 1 to %d characters", errInvalidExternalIDs, p.Iss, maxExternalIDLength)
		}

		if strings.TrimSpace(p.ID) != p.ID {
			return fmt.Errorf("%w: id for iss %s has leading or trailing spaces", errInvalidExternalIDs, p.Iss)
		}
	}

	return nil
}

// upsertExternalID returns current with p in place of the record at p.Iss.
func upsertExternalID(current []dbtools.ExternalIDPair, p dbtools.ExternalIDPair) []dbtools.ExternalIDPair {
	out := make([]dbtools.ExternalIDPair, 0, len(current)+1)

	for _, c := range current {
		if c.Iss != p.Iss {
			out = append(out, c)
		}
	}

	return append(out, p)
}

// userExternalIDs returns the identities of the user userID, ordered by issuer.
func userExternalIDs(ctx context.Context, exec boil.ContextExecutor, userID string) ([]dbtools.ExternalIDPair, error) {
	ids, err := dbtools.ListUserExternalIDs(ctx, exec, []string{userID})
	if err != nil {
		return nil, err
	}

	return ids[userID], nil
}

// auditExternalIDChanges writes one audit event for each identity added to
// and removed from u, and returns the events.
func auditExternalIDChanges(
	ctx context.Context, exec boil.ContextExecutor, auditID string, actor, u *models.User, added, removed []dbtools.ExternalIDPair,
) ([]*models.AuditEvent, error) {
	evs := make([]*models.AuditEvent, 0, len(added)+len(removed))

	for _, p := range removed {
		ev, err := dbtools.AuditUserExternalIDRemoved(ctx, exec, auditID, actor, u, p)
		if err != nil {
			return nil, err
		}

		evs = append(evs, ev)
	}

	for _, p := range added {
		ev, err := dbtools.AuditUserExternalIDAdded(ctx, exec, auditID, actor, u, p)
		if err != nil {
			return nil, err
		}

		evs = append(evs, ev)
	}

	return evs, nil
}

// newUserResponse returns u with its identities pairs and the deprecated
// external_id computed from them.
func (r *Router) newUserResponse(u *models.User, pairs []dbtools.ExternalIDPair) *User {
	if pairs == nil {
		pairs = []dbtools.ExternalIDPair{}
	}

	return &User{
		User:        u,
		ExternalIDs: pairs,
		ExternalID:  dbtools.PrimaryExternalID(pairs, r.PrimaryIdentityIssuer),
	}
}

// setExternalIDDeprecation marks a response that carries the deprecated
// external_id field.
func setExternalIDDeprecation(c *gin.Context) {
	c.Header("Deprecation", "true")
	c.Header("Link", externalIDDeprecationLink)
}

// rollbackAndSendError rolls tx back and sends msg with code, adding the
// rollback error to msg when there is one.
func rollbackAndSendError(c *gin.Context, tx *sql.Tx, code int, msg string) {
	if err := tx.Rollback(); err != nil {
		msg += "error rolling back transaction: " + err.Error()
	}

	sendError(c, code, msg)
}
