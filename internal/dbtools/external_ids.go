package dbtools

import (
	"context"
	"fmt"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/aarondl/sqlboiler/v4/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	models "github.com/metal-toolbox/governor-api/internal/models/psql"
)

var tracer = otel.GetTracerProvider().Tracer("governor-api/dbtools")

// ExternalIDPair is one identity of a user at an identity provider.
type ExternalIDPair struct {
	Iss string `json:"iss"`
	ID  string `json:"id"`
}

// FindUserByExternalID returns the active (not deleted) user that has the
// identity externalID at the issuer iss. It returns sql.ErrNoRows when there
// is none. mods are applied to the users query, for example to load relations.
func FindUserByExternalID(ctx context.Context, exec boil.ContextExecutor, iss, externalID string, mods ...qm.QueryMod) (*models.User, error) {
	ctx, span := tracer.Start(ctx, "FindUserByExternalID")
	defer span.End()

	span.SetAttributes(attribute.String("iss", iss))

	queryMods := append([]qm.QueryMod{
		qm.InnerJoin(models.TableNames.UserExternalIds + " uei ON uei.user_id = " + models.TableNames.Users + ".id"),
		qm.Where("uei.iss = ? AND uei.external_id = ?", iss, externalID),
	}, mods...)

	user, err := models.Users(queryMods...).One(ctx, exec)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return user, nil
}

// AddUserExternalID records the identity externalID at the issuer iss for the
// user userID. It returns an error matching ErrExternalIDConflict when the
// identity belongs to another user or the user already has one for iss.
func AddUserExternalID(ctx context.Context, exec boil.ContextExecutor, userID, iss, externalID string) (*models.UserExternalID, error) {
	ctx, span := tracer.Start(ctx, "AddUserExternalID")
	defer span.End()

	span.SetAttributes(attribute.String("iss", iss))

	rec := &models.UserExternalID{
		UserID:     userID,
		Iss:        iss,
		ExternalID: externalID,
	}

	if err := rec.Insert(ctx, exec, boil.Infer()); err != nil {
		span.SetStatus(codes.Error, err.Error())

		if _, ok := UniqueViolation(err); ok {
			return nil, fmt.Errorf("%w: %v", ErrExternalIDConflict, err)
		}

		return nil, err
	}

	return rec, nil
}

// AuditUserExternalIDAdded inserts an audit event for an identity added to the user u.
func AuditUserExternalIDAdded(ctx context.Context, exec boil.ContextExecutor, pID string, actor, u *models.User, p ExternalIDPair) (*models.AuditEvent, error) {
	ctx, span := tracer.Start(ctx, "AuditUserExternalIDAdded")
	defer span.End()

	return auditUserExternalID(ctx, span, exec, "user.external_id.added", pID, actor, u, p)
}

// AuditUserExternalIDRemoved inserts an audit event for an identity removed from the user u.
func AuditUserExternalIDRemoved(ctx context.Context, exec boil.ContextExecutor, pID string, actor, u *models.User, p ExternalIDPair) (*models.AuditEvent, error) {
	ctx, span := tracer.Start(ctx, "AuditUserExternalIDRemoved")
	defer span.End()

	return auditUserExternalID(ctx, span, exec, "user.external_id.removed", pID, actor, u, p)
}

func auditUserExternalID(
	ctx context.Context, span trace.Span, exec boil.ContextExecutor, action, pID string, actor, u *models.User, p ExternalIDPair,
) (*models.AuditEvent, error) {
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:      null.StringFrom(pID),
		ActorID:       actorID,
		SubjectUserID: null.StringFrom(u.ID),
		Action:        action,
		Changeset:     []string{"iss: " + p.Iss, "external_id: " + p.ID},
	}

	if err := event.Insert(ctx, exec, boil.Infer()); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return &event, nil
}

// ReplaceUserExternalIDs makes ids the full set of identities of the user
// userID. It deletes the records that are not in ids, then inserts the new
// ones, and returns both, so the caller can audit them. Both are empty when
// ids equals the stored set. It returns an error matching
// ErrExternalIDConflict when an identity in ids belongs to another user.
// Run it in a transaction: an error leaves part of the change written.
func ReplaceUserExternalIDs(
	ctx context.Context, exec boil.ContextExecutor, userID string, ids []ExternalIDPair,
) (added, removed []ExternalIDPair, err error) {
	ctx, span := tracer.Start(ctx, "ReplaceUserExternalIDs")
	defer span.End()

	current, err := models.UserExternalIds(
		models.UserExternalIDWhere.UserID.EQ(userID),
		qm.OrderBy(models.UserExternalIDColumns.Iss),
	).All(ctx, exec)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, nil, err
	}

	want := make(map[ExternalIDPair]struct{}, len(ids))
	for _, p := range ids {
		want[p] = struct{}{}
	}

	have := make(map[ExternalIDPair]struct{}, len(current))
	removed = []ExternalIDPair{}

	for _, rec := range current {
		p := ExternalIDPair{Iss: rec.Iss, ID: rec.ExternalID}
		have[p] = struct{}{}

		if _, ok := want[p]; ok {
			continue
		}

		if _, err := rec.Delete(ctx, exec); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return nil, nil, err
		}

		removed = append(removed, p)
	}

	added = []ExternalIDPair{}

	for _, p := range ids {
		if _, ok := have[p]; ok {
			continue
		}

		if _, err := AddUserExternalID(ctx, exec, userID, p.Iss, p.ID); err != nil {
			span.SetStatus(codes.Error, err.Error())
			return nil, nil, err
		}

		added = append(added, p)
	}

	return added, removed, nil
}

// DeleteUserExternalIDs deletes every identity of the user userID.
func DeleteUserExternalIDs(ctx context.Context, exec boil.ContextExecutor, userID string) error {
	ctx, span := tracer.Start(ctx, "DeleteUserExternalIDs")
	defer span.End()

	if _, err := models.UserExternalIds(models.UserExternalIDWhere.UserID.EQ(userID)).DeleteAll(ctx, exec); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

// ListUserExternalIDs returns the identities of the users userIDs, ordered by
// issuer, in one query. The map has an entry, possibly empty, for every ID in
// userIDs.
func ListUserExternalIDs(ctx context.Context, exec boil.ContextExecutor, userIDs []string) (map[string][]ExternalIDPair, error) {
	ctx, span := tracer.Start(ctx, "ListUserExternalIDs")
	defer span.End()

	span.SetAttributes(attribute.Int("users", len(userIDs)))

	out := make(map[string][]ExternalIDPair, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	for _, id := range userIDs {
		out[id] = []ExternalIDPair{}
	}

	recs, err := models.UserExternalIds(
		qm.Where(models.UserExternalIDColumns.UserID+" = ANY(?::uuid[])", types.StringArray(userIDs)),
		qm.OrderBy(models.UserExternalIDColumns.UserID+", "+models.UserExternalIDColumns.Iss),
	).All(ctx, exec)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	for _, rec := range recs {
		out[rec.UserID] = append(out[rec.UserID], ExternalIDPair{Iss: rec.Iss, ID: rec.ExternalID})
	}

	return out, nil
}

// WhereUserHasExternalID selects the users that have a record at one of
// issuers with one of externalIDs. An empty issuers matches any issuer, and an
// empty externalIDs any external ID.
func WhereUserHasExternalID(issuers, externalIDs []string) qm.QueryMod {
	clause := "EXISTS (SELECT 1 FROM " + models.TableNames.UserExternalIds + " uei WHERE uei.user_id = " +
		models.TableNames.Users + ".id"
	args := []any{}

	if len(issuers) > 0 {
		clause += " AND uei.iss = ANY(?::text[])"

		args = append(args, types.StringArray(issuers))
	}

	if len(externalIDs) > 0 {
		clause += " AND uei.external_id = ANY(?::text[])"

		args = append(args, types.StringArray(externalIDs))
	}

	return qm.Where(clause+")", args...)
}

// PrimaryExternalID returns the external ID in pairs of the primary identity
// issuer primaryIss, for the deprecated external_id API field. It is null
// when there is no primary issuer or pairs has no record for it.
func PrimaryExternalID(pairs []ExternalIDPair, primaryIss string) null.String {
	if primaryIss == "" {
		return null.String{}
	}

	for _, p := range pairs {
		if p.Iss == primaryIss {
			return null.StringFrom(p.ID)
		}
	}

	return null.String{}
}
