package v1alpha1

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/governor-api/internal/dbtools"
	models "github.com/metal-toolbox/governor-api/internal/models/psql"
)

const (
	inactiveUserMetadataKey = "retention-ctl"
	inactiveUserMetadataVal = "expired"
)

func anonymizeUser(user models.User) (models.User, error) {
	const randomSuffixLen = 5

	random := make([]byte, randomSuffixLen)

	_, err := rand.Read(random)
	if err != nil {
		return models.User{}, err
	}

	randomStr := hex.EncodeToString(random)

	inactiveUserMetadata := map[string]interface{}{
		"annotations": map[string]string{
			inactiveUserMetadataKey: inactiveUserMetadataVal,
		},
	}

	metadatajson, err := json.Marshal(inactiveUserMetadata)
	if err != nil {
		return models.User{}, err
	}

	user.Name = fmt.Sprintf("Deleted User %s", randomStr)
	user.Email = fmt.Sprintf("deleted-user@%s.com", randomStr)
	user.AvatarURL = null.NewString("", false)
	user.GithubID = null.NewInt64(0, false)
	user.GithubUsername = null.NewString("", false)
	user.Metadata = metadatajson

	return user, nil
}

// deleteUserRecord removes PII data for a user that has been soft deleted
// governor API cannot simply delete a user from the database as there may be
// foreign key references to that user in other tables. Instead, we anonymize
// the user data by removing PII fields such as email, name, etc.
func (r *Router) deleteUserRecord(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "deleteUserRecord")
	defer span.End()

	id := c.Param("id")

	user, err := models.Users(
		qm.WithDeleted(),
		qm.Where("id = ?", id),
		qm.Where(
			"metadata#>>? IS DISTINCT FROM ?",
			fmt.Sprintf("{annotations,%s}", inactiveUserMetadataKey),
			inactiveUserMetadataVal,
		),
	).One(ctx, r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			recordAndSendError(c, span, nil, http.StatusNotFound, "user not found", err)
			return
		}

		recordAndSendError(c, span, r.Logger, http.StatusInternalServerError, "failed to get user", err)

		return
	}

	if !user.DeletedAt.Valid {
		recordAndSendError(
			c, span, r.Logger, http.StatusBadRequest,
			"user must be deleted before record removal", ErrRemoveActiveRecord,
		)

		return
	}

	updated, err := anonymizeUser(*user)
	if err != nil {
		recordAndSendError(c, span, r.Logger, http.StatusInternalServerError, "failed to anonymize user", err)
		return
	}

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		recordAndSendError(c, span, r.Logger, http.StatusInternalServerError, "failed to begin transaction", err)
		return
	}

	if _, err = updated.Update(ctx, tx, boil.Infer()); err != nil {
		msg := "error updating user record" + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += " | error rolling back transaction: " + err.Error()
		}

		recordAndSendError(c, span, r.Logger, http.StatusInternalServerError, msg, err)

		return
	}

	event, err := dbtools.AuditUserUpdated(
		ctx, tx, getCtxAuditID(c), getCtxUser(c), user, &updated,
	)
	if err != nil {
		msg := "error creating audit event for user update" + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += " | error rolling back transaction: " + err.Error()
		}

		recordAndSendError(c, span, r.Logger, http.StatusInternalServerError, msg, err)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error updating context with audit event data" + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += " | error rolling back transaction: " + err.Error()
		}

		recordAndSendError(c, span, r.Logger, http.StatusInternalServerError, msg, err)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing transaction" + err.Error()
		if err := tx.Rollback(); err != nil {
			msg += " | error rolling back transaction: " + err.Error()
		}

		recordAndSendError(c, span, r.Logger, http.StatusInternalServerError, msg, err)

		return
	}

	c.JSON(http.StatusAccepted, updated)
}
