package dbtools

import (
	"context"
	"fmt"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

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

	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:      null.StringFrom(pID),
		ActorID:       actorID,
		SubjectUserID: null.StringFrom(u.ID),
		Action:        "user.external_id.added",
		Changeset:     []string{"iss: " + p.Iss, "external_id: " + p.ID},
	}

	if err := event.Insert(ctx, exec, boil.Infer()); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return &event, nil
}
