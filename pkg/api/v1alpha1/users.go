package v1alpha1

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"dario.cat/mergo"
	"github.com/gin-gonic/gin"
	"github.com/metal-toolbox/auditevent/ginaudit"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"github.com/volatiletech/sqlboiler/v4/types"
	"go.uber.org/zap"

	"github.com/metal-toolbox/governor-api/internal/dbtools"
	"github.com/metal-toolbox/governor-api/internal/models"
	events "github.com/metal-toolbox/governor-api/pkg/events/v1alpha1"
)

const (
	// UserStatusActive is the status for an active user
	UserStatusActive = "active"

	// UserStatusPending is the status for a pending user
	UserStatusPending = "pending"

	// UserStatusSuspended is the status for a suspended user
	UserStatusSuspended = "suspended"
)

var (
	permittedListUsersParams = []string{"external_id", "email", "metadata"}
	metadataKeyPattern       = regexp.MustCompile(`^[a-zA-Z0-9_/]+$`)
)

// isValidMetadata recursively validates that all keys in the metadata map
// follow the pattern [a-zA-Z0-9_/]+
func isValidMetadata(metadata map[string]interface{}) bool {
	for key, value := range metadata {
		// Check if the key matches the pattern [a-zA-Z0-9_/]+
		if !metadataKeyPattern.MatchString(key) {
			return false
		}

		// If the value is a map, recursively validate its keys
		if nestedMap, ok := value.(map[string]interface{}); ok {
			if !isValidMetadata(nestedMap) {
				return false
			}
		}

		// If the value is a slice, check if any element is a map and validate it
		if slice, ok := value.([]interface{}); ok {
			for _, item := range slice {
				if nestedMap, ok := item.(map[string]interface{}); ok {
					if !isValidMetadata(nestedMap) {
						return false
					}
				}
			}
		}
	}

	return true
}

// User is a user response
type User struct {
	*models.User
	Memberships             []string                            `json:"memberships,omitempty"`
	MembershipsDirect       []string                            `json:"memberships_direct,omitempty"`
	MembershipRequests      []string                            `json:"membership_requests,omitempty"`
	NotificationPreferences dbtools.UserNotificationPreferences `json:"notification_preferences,omitempty"`
}

// UserReq is a user request payload
type UserReq struct {
	AvatarURL      string                  `json:"avatar_url,omitempty"`
	Email          string                  `json:"email"`
	ExternalID     string                  `json:"external_id"`
	GithubID       string                  `json:"github_id,omitempty"`
	GithubUsername string                  `json:"github_username,omitempty"`
	Name           string                  `json:"name"`
	Status         string                  `json:"status,omitempty"`
	Metadata       *map[string]interface{} `json:"metadata,omitempty"`
}

// listUsers responds with the list of all users
func (r *Router) listUsers(c *gin.Context) {
	queryMods := []qm.QueryMod{}

	if _, ok := c.GetQuery("deleted"); ok {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	for k, val := range c.Request.URL.Query() {
		r.Logger.Debug("checking query", zap.String("url.query.key", k), zap.Strings("url.query.value", val))

		if k == "deleted" {
			continue
		}

		// check for allowed parameters
		if !contains(permittedListUsersParams, k) {
			r.Logger.Warn("found illegal parameter in request", zap.String("parameter", k))

			sendError(c, http.StatusBadRequest, "illegal parameter: "+k)

			return
		}

		convertedVals := make([]interface{}, len(val))
		for i, v := range val {
			convertedVals[i] = v
		}

		switch k {
		case "email":
			// append email queries as case insensitive using LOWER()
			// Note: This does a table scan by default, but we should be able to add a functional
			// index if performance is an issue: CREATE INDEX ON users (LOWER(email));
			// alternatives are to use ILIKE which is postgres specific, and require sanitizing '%' if we want exact matches
			for _, v := range convertedVals {
				queryMods = append(queryMods, qm.Or("LOWER(email) = LOWER(?)", v))
			}
		case "metadata":
			// metadata should be a JSON formatted object used to filter users with
			// specific metadata values
			//
			// this object should be in the format of:
			// ?metadata=key1=value&metadata=path.to.nested.field=value2
			//
			const KVPartsLen = 2

			for _, searchString := range val {
				searchKV := strings.SplitN(searchString, "=", KVPartsLen)
				if len(searchKV) < KVPartsLen {
					r.Logger.Error("invalid metadata query format", zap.String("metadata", searchString))
					sendError(c, http.StatusBadRequest, "invalid metadata query format: "+searchString)

					return
				}

				searchKey, searchValue := searchKV[0], searchKV[1]
				pathComponents := strings.Split(searchKey, ".")

				for _, pc := range pathComponents {
					if !metadataKeyPattern.MatchString(pc) {
						r.Logger.Error("invalid metadata key", zap.String("key", pc))
						sendError(c, http.StatusBadRequest, "invalid metadata key: "+pc)

						return
					}
				}

				sqlPath := fmt.Sprintf("{%s}", strings.Join(pathComponents, ","))

				r.Logger.Debug(
					"adding metadata query",
					zap.String("search_key", searchKey),
					zap.String("search_value", searchValue),
					zap.String("sql_path", sqlPath),
				)

				queryMods = append(
					queryMods,
					qm.Where("metadata#>>? = ?", sqlPath, searchValue),
				)
			}
		default:
			queryMods = append(queryMods, qm.Or2(qm.WhereIn(k+" IN ?", convertedVals...)))
		}
	}

	users, err := models.Users(queryMods...).All(c.Request.Context(), r.DB)
	if err != nil {
		r.Logger.Error("error fetching users", zap.Error(err))
		sendError(c, http.StatusBadRequest, "error fetching users: "+err.Error())

		return
	}

	c.JSON(http.StatusOK, users)
}

// getUser gets a user
func (r *Router) getUser(c *gin.Context) {
	id := c.Param("id")

	queryMods := []qm.QueryMod{
		qm.Where("id = ?", id),
		qm.Load("GroupMembershipRequests"),
	}

	if _, ok := c.GetQuery("deleted"); ok {
		queryMods = append(queryMods, qm.WithDeleted())
	}

	user, err := models.Users(queryMods...).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "user not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user: "+err.Error())

		return
	}

	enumeratedMemberships, err := dbtools.GetMembershipsForUser(c.Request.Context(), r.DB.DB, user.ID, false)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error enumerating group membership: "+err.Error())
		return
	}

	memberships := make([]string, len(enumeratedMemberships))

	membershipsDirect := make([]string, 0)

	for i, m := range enumeratedMemberships {
		memberships[i] = m.GroupID

		if m.Direct {
			membershipsDirect = append(membershipsDirect, m.GroupID)
		}
	}

	requests := make([]string, len(user.R.GroupMembershipRequests))
	for i, r := range user.R.GroupMembershipRequests {
		requests[i] = r.GroupID
	}

	notificationPreferences, err := dbtools.GetNotificationPreferences(c.Request.Context(), id, r.DB, true)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error getting notification preferences: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, User{
		User:                    user,
		Memberships:             memberships,
		MembershipsDirect:       membershipsDirect,
		MembershipRequests:      requests,
		NotificationPreferences: notificationPreferences,
	})
}

// createUser creates a user in the database
func (r *Router) createUser(c *gin.Context) {
	req := UserReq{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	// check for required parameters
	if req.Email == "" {
		sendError(c, http.StatusBadRequest, "missing email in user request")
		return
	}

	if req.Name == "" {
		sendError(c, http.StatusBadRequest, "missing name in user request")
		return
	}

	user := &models.User{
		Email: req.Email,
		Name:  req.Name,
	}

	// check if user already exists
	exists, err := models.Users(
		qm.Where("LOWER(email) = LOWER(?)", req.Email),
	).Exists(c.Request.Context(), r.DB)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "error checking user exists: "+err.Error())
		return
	}

	if exists {
		sendError(c, http.StatusConflict, "user already exists")
		return
	}

	// add optional parameters
	if req.ExternalID != "" {
		user.ExternalID = null.StringFrom(req.ExternalID)
	}

	if req.AvatarURL != "" {
		user.AvatarURL = null.StringFrom(req.AvatarURL)
	}

	if req.GithubID != "" {
		ghID, err := strconv.ParseInt(req.GithubID, 10, 64) //nolint:gomnd
		if err != nil {
			sendError(c, http.StatusBadRequest, "error parsing github id string as int")
			return
		}

		user.GithubID = null.Int64From(ghID)
	}

	if req.GithubUsername != "" {
		user.GithubUsername = null.StringFrom(req.GithubUsername)
	}

	if req.Status != "" {
		user.Status = null.StringFrom(req.Status)
	}

	// if the user has no external_id and the status is not explicitly set, we assume it's pending
	if req.ExternalID == "" && req.Status == "" {
		user.Status = null.StringFrom(UserStatusPending)
	}

	user.Metadata = types.JSON{}

	if req.Metadata == nil {
		req.Metadata = &map[string]interface{}{}
	}

	if !isValidMetadata(*req.Metadata) {
		sendError(c, http.StatusBadRequest, "invalid metadata keys, must match pattern [a-zA-Z0-9_/]+")
		return
	}

	if err := user.Metadata.Marshal(req.Metadata); err != nil {
		sendError(c, http.StatusBadRequest, "error marshalling metadata: "+err.Error())
		return
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting create user transaction: "+err.Error())
		return
	}

	if err := user.Insert(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := "error creating user: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditUserCreatedWithActor(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), user)
	if err != nil {
		msg := "error creating user (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error creating user (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing user create, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// only publish events for active users
	if !isActiveUser(user) {
		c.JSON(http.StatusAccepted, user)
		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorUsersEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventCreate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: "",
		UserID:  user.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish user create event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, user)
}

// updateUser updates a user in the database. It is also used to activate a pending user by setting their status
// to `active` and emitting events for all the groups they may be a member of.
//
//nolint:gocyclo
func (r *Router) updateUser(c *gin.Context) {
	id := c.Param("id")

	user, err := models.FindUser(c.Request.Context(), r.DB, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "user not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user "+err.Error())

		return
	}

	original := *user

	userActivated := false

	req := UserReq{}
	if err := c.BindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "unable to bind request: "+err.Error())
		return
	}

	if req == (UserReq{}) {
		sendError(c, http.StatusBadRequest, "missing user request parameters")
		return
	}

	if req.AvatarURL != "" {
		user.AvatarURL = null.StringFrom(req.AvatarURL)
	}

	if req.Email != "" {
		user.Email = req.Email
	}

	if req.ExternalID != "" {
		user.ExternalID = null.StringFrom(req.ExternalID)
	}

	if req.Name != "" {
		user.Name = req.Name
	}

	if req.Status != "" {
		if req.Status == UserStatusPending {
			sendError(c, http.StatusBadRequest, "existing users cannot be made pending")
			return
		}

		if user.Status.String != UserStatusActive && req.Status == UserStatusSuspended {
			sendError(c, http.StatusBadRequest, "only active users can be suspended")
			return
		}

		if user.Status.String == UserStatusPending && req.Status == UserStatusActive {
			userActivated = true
		}

		user.Status = null.StringFrom(req.Status)
	}

	if req.GithubID != "" {
		ghID, err := strconv.ParseInt(req.GithubID, 10, 64) //nolint:gomnd
		if err != nil {
			sendError(c, http.StatusBadRequest, "error parsing github id string as int")
			return
		}

		user.GithubID = null.Int64From(ghID)
	}

	if req.GithubUsername != "" {
		user.GithubUsername = null.StringFrom(req.GithubUsername)
	}

	if req.Metadata != nil {
		current := map[string]interface{}{}
		incoming := *req.Metadata

		if err := user.Metadata.Unmarshal(&current); err != nil {
			sendError(c, http.StatusBadRequest, "error unmarshalling user metadata: "+err.Error())
			return
		}

		// merge the new metadata with the existing one
		if err := mergo.Merge(
			&current, incoming,
			mergo.WithOverride,
			mergo.WithOverrideEmptySlice,
		); err != nil {
			sendError(c, http.StatusBadRequest, "error merging user metadata: "+err.Error())
			return
		}

		if !isValidMetadata(current) {
			sendError(c, http.StatusBadRequest, "invalid metadata keys, must match pattern [a-zA-Z0-9_/]+")
			return
		}

		if err := user.Metadata.Marshal(&current); err != nil {
			sendError(c, http.StatusBadRequest, "error marshalling user metadata: "+err.Error())
			return
		}
	}

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting update transaction: "+err.Error())
		return
	}

	if _, err := user.Update(c.Request.Context(), tx, boil.Infer()); err != nil {
		msg := "error updating user: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditUserUpdated(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), &original, user)
	if err != nil {
		msg := "error updating user (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error updating user (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing user update, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// only publish events for active users
	if !isActiveUser(user) {
		c.JSON(http.StatusAccepted, user)
		return
	}

	// if this was a pending user that got activated, we publish a user create event and
	// events for all the groups they should be a member of
	if userActivated {
		r.Logger.Debug("activating user", zap.Any("user", user))

		if err := r.EventBus.Publish(c.Request.Context(), events.GovernorUsersEventSubject, &events.Event{
			Version: events.Version,
			Action:  events.GovernorEventCreate,
			AuditID: c.GetString(ginaudit.AuditIDContextKey),
			ActorID: getCtxActorID(c),
			GroupID: "",
			UserID:  user.ID,
		}); err != nil {
			r.Logger.Warn("failed to publish user create event, downstream changes may be delayed", zap.Error(err))
		}

		memberships, err := user.GroupMemberships(
			qm.Load(models.GroupMembershipRels.Group),
		).All(c.Request.Context(), r.DB)
		if err != nil {
			sendError(c, http.StatusInternalServerError, "failed to get user memberships "+err.Error())
			return
		}

		for _, m := range memberships {
			if err := r.EventBus.Publish(c.Request.Context(), events.GovernorMembersEventSubject, &events.Event{
				Version: events.Version,
				Action:  events.GovernorEventCreate,
				AuditID: c.GetString(ginaudit.AuditIDContextKey),
				ActorID: getCtxActorID(c),
				GroupID: m.GroupID,
				UserID:  user.ID,
			}); err != nil {
				r.Logger.Warn("failed to publish members create event, downstream changes may be delayed", zap.Error(err))
			}
		}

		c.JSON(http.StatusAccepted, user)

		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorUsersEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventUpdate,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: "",
		UserID:  user.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish user update event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, user)
}

// deleteUser marks a user as deleted in the database
func (r *Router) deleteUser(c *gin.Context) {
	id := c.Param("id")

	user, err := models.Users(
		qm.Where("id = ?", id),
		qm.Load("GroupMemberships"),
		qm.Load("GroupMembershipRequests"),
	).One(c.Request.Context(), r.DB)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendError(c, http.StatusNotFound, "user not found: "+err.Error())
			return
		}

		sendError(c, http.StatusInternalServerError, "error getting user "+err.Error())

		return
	}

	original := *user

	tx, err := r.DB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		sendError(c, http.StatusBadRequest, "error starting delete transaction: "+err.Error())
		return
	}

	// delete all group memberships
	if _, err := user.R.GroupMemberships.DeleteAll(c.Request.Context(), tx); err != nil {
		msg := "error deleting group membership, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// delete all group membership requests
	if _, err := user.R.GroupMembershipRequests.DeleteAll(c.Request.Context(), tx); err != nil {
		msg := "error deleting group membership requests, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// soft delete the user
	if _, err := user.Delete(c.Request.Context(), tx, false); err != nil {
		msg := "error deleting user, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	event, err := dbtools.AuditUserDeleted(c.Request.Context(), tx, getCtxAuditID(c), getCtxUser(c), &original, user)
	if err != nil {
		msg := "error deleting user (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := updateContextWithAuditEventData(c, event); err != nil {
		msg := "error deleting user (audit): " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg += "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	if err := tx.Commit(); err != nil {
		msg := "error committing user delete, rolling back: " + err.Error()

		if err := tx.Rollback(); err != nil {
			msg = msg + "error rolling back transaction: " + err.Error()
		}

		sendError(c, http.StatusBadRequest, msg)

		return
	}

	// only publish events for active users
	if !isActiveUser(user) {
		c.JSON(http.StatusAccepted, user)
		return
	}

	if err := r.EventBus.Publish(c.Request.Context(), events.GovernorUsersEventSubject, &events.Event{
		Version: events.Version,
		Action:  events.GovernorEventDelete,
		AuditID: c.GetString(ginaudit.AuditIDContextKey),
		ActorID: getCtxActorID(c),
		GroupID: "",
		UserID:  user.ID,
	}); err != nil {
		sendError(c, http.StatusBadRequest, "failed to publish user delete event, downstream changes may be delayed "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, user)
}

// isActiveUser returns true if events related to the given user should be published.
// Currently this includes active and suspended users.
func isActiveUser(user *models.User) bool {
	if user == nil {
		return false
	}

	return user.Status.String == UserStatusActive || user.Status.String == UserStatusSuspended
}
