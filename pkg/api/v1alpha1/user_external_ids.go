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
	apitypes "github.com/metal-toolbox/governor-api/pkg/api/types"
)

const (
	// maxUserExternalIDs is the most identities a user request may set.
	maxUserExternalIDs = 10
	// maxIssuerLength and maxExternalIDLength match the length checks of the
	// user_external_ids table.
	maxIssuerLength     = 512
	maxExternalIDLength = 256
)

// requestedExternalIDs returns the identities that req sets, validated. ok is
// false when req sets none. The deprecated external_id becomes the one
// identity at the primary identity issuer. Whether each identity may be
// written is checked against the user's records by checkIdentityIssuers.
func (r *Router) requestedExternalIDs(req *UserReq) (pairs []dbtools.ExternalID, ok bool, err error) {
	switch {
	case req.ExternalID != "" && req.ExternalIDs != nil:
		return nil, false, errBothExternalIDFields
	case req.ExternalID != "":
		if r.PrimaryIdentityIssuer == "" {
			return nil, false, errNoPrimaryIdentityIssuer
		}

		pairs = []dbtools.ExternalID{{Iss: r.PrimaryIdentityIssuer, ID: req.ExternalID}}
	case req.ExternalIDs != nil:
		pairs = *req.ExternalIDs
	default:
		return nil, false, nil
	}

	if err := validateExternalIDs(pairs, r.authConf.Issuers()); err != nil {
		return nil, false, err
	}

	return pairs, true, nil
}

// checkIdentityIssuers returns an error matching errInvalidExternalIDs when a
// pair in pairs is at an issuer without identity.enabled and is not one of
// current, the user's stored records. A record at such an issuer can never
// sign in, so it cannot be added or changed, but an unchanged one may be sent
// back so that a full-set update keeps it.
func (r *Router) checkIdentityIssuers(pairs, current []dbtools.ExternalID) error {
	identityIssuers := r.authConf.IdentityIssuers()

	for _, p := range pairs {
		if slices.Contains(identityIssuers, p.Iss) || slices.Contains(current, p) {
			continue
		}

		return fmt.Errorf("%w: iss %s has no oidc provider with identity.enabled", errInvalidExternalIDs, p.Iss)
	}

	return nil
}

// allConfiguredIssuers reports whether every issuer in issuers is the issuer
// of a configured oidc provider.
func (r *Router) allConfiguredIssuers(issuers []string) bool {
	configured := r.authConf.Issuers()

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
func validateExternalIDs(pairs []dbtools.ExternalID, issuers []string) error {
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
func upsertExternalID(current []dbtools.ExternalID, p dbtools.ExternalID) []dbtools.ExternalID {
	out := make([]dbtools.ExternalID, 0, len(current)+1)

	for _, c := range current {
		if c.Iss != p.Iss {
			out = append(out, c)
		}
	}

	return append(out, p)
}

// userExternalIDs returns the identities of the user userID, ordered by issuer.
func userExternalIDs(ctx context.Context, exec boil.ContextExecutor, userID string) ([]dbtools.ExternalID, error) {
	ids, err := dbtools.ListUserExternalIDs(ctx, exec, []string{userID})
	if err != nil {
		return nil, err
	}

	return ids[userID], nil
}

// auditExternalIDChanges writes one audit event for each identity added to
// and removed from u, and returns the events.
func auditExternalIDChanges(
	ctx context.Context, exec boil.ContextExecutor, auditID string, actor, u *models.User, added, removed []dbtools.ExternalID,
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
func (r *Router) newUserResponse(u *models.User, pairs []dbtools.ExternalID) *User {
	if pairs == nil {
		pairs = []dbtools.ExternalID{}
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
	c.Header("Link", apitypes.ExternalIDDeprecationLink)
}

// rollbackAndSendError rolls tx back and sends msg with code, adding the
// rollback error to msg when there is one.
func rollbackAndSendError(c *gin.Context, tx *sql.Tx, code int, msg string) {
	if err := tx.Rollback(); err != nil {
		msg += "error rolling back transaction: " + err.Error()
	}

	sendError(c, code, msg)
}
