package dbtools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/gosimple/slug"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/types"

	"github.com/metal-toolbox/governor-api/internal/models"
)

// RegisterHooks adds any hooks that are configured to the models library
func RegisterHooks() {
}

// SetGroupSlug assigns a Group model a slug from the Group name
func SetGroupSlug(g *models.Group) {
	g.Slug = slug.Make(g.Name)
}

// SetOrganizationSlug assigns an Organization model a slug from the Organization name
func SetOrganizationSlug(o *models.Organization) {
	o.Slug = slug.Make(o.Name)
}

// SetApplicationSlug assigns an Application model a slug from the Application name
func SetApplicationSlug(a *models.Application) {
	a.Slug = slug.Make(a.Name)
}

// SetApplicationTypeSlug assigns an ApplicationType model a slug from the ApplicationType name
func SetApplicationTypeSlug(a *models.ApplicationType) {
	a.Slug = slug.Make(a.Name)
}

func changesetLine(set []string, key string, old, new interface{}) []string {
	if reflect.DeepEqual(old, new) {
		return set
	}

	var str string

	switch o := old.(type) {
	case string:
		str = fmt.Sprintf(`%s: "%s" => "%s"`, key, o, new.(string))
	case null.String:
		str = fmt.Sprintf(`%s: "%s" => "%s"`, key, o.String, new.(null.String).String)
	case int:
		str = fmt.Sprintf(`%s: "%d" => "%d"`, key, o, new)
	case int64:
		str = fmt.Sprintf(`%s: "%d" => "%d"`, key, o, new)
	case bool:
		str = fmt.Sprintf(`%s: "%t" => "%t"`, key, o, new)
	case time.Time:
		str = fmt.Sprintf(`%s: "%s" => "%s"`, key, o.UTC().Format(time.RFC3339), new.(time.Time).UTC().Format(time.RFC3339))
	case types.JSON:
		str = fmt.Sprintf(`%s: "%s" => "%s"`, key, string(o), string(new.(types.JSON)))
	default:
		str = fmt.Sprintf(`%s: "%s" => "%s"`, key, o, new)
	}

	return append(set, str)
}

// use reflect to iterate through each element of a given type, and construct changesetLine with each attribute
// it can only accept non nil pointers
// more information for the reflect package can be found here: https://pkg.go.dev/reflect
// alternatively see the go dev blog: https://go.dev/blog/laws-of-reflection
// go playground: https://go.dev/play/p/OdraKtlfUCA
func calculateChangeset(original, new interface{}) []string {
	changeset := []string{}

	a := reflect.ValueOf(original).Elem()
	b := reflect.ValueOf(new).Elem()

	for i := 0; i < a.NumField(); i++ {
		field := a.Type().Field(i).Name

		// In some cases, just continue, we don't want to evaluate/emit changes to the object relationships and the like
		// UpdatedAt field is already used in views for audits so no need to display them
		switch field {
		case "ID":
		case "Slug":
		case "CreatedAt":
		case "UpdatedAt":
		case "R":
		case "L":
			continue
		default:
			changeset = changesetLine(changeset, field, a.Field(i).Interface(), b.Field(i).Interface())
		}
	}

	return changeset
}

func calculateGroupMembershipChangeset(origGM, newGM *models.GroupMembership) []string {
	changeset := []string{}
	changeset = changesetLine(changeset, "is_admin", origGM.IsAdmin, newGM.IsAdmin)

	return changeset
}

// AuditUserCreatedWithActor inserts an event representing user creation into the event table
func AuditUserCreatedWithActor(ctx context.Context, exec boil.ContextExecutor, pID string, actor, u *models.User) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:      null.StringFrom(pID),
		ActorID:       actorID,
		SubjectUserID: null.StringFrom(u.ID),
		Action:        "user.created",
		Changeset:     calculateChangeset(&models.User{}, u),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditUserDeleted inserts an event representing user delete into the event table
func AuditUserDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor, original, new *models.User) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:      null.StringFrom(pID),
		ActorID:       actorID,
		SubjectUserID: null.StringFrom(original.ID),
		Action:        "user.deleted",
		Changeset:     calculateChangeset(original, new),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditUserUpdated inserts an event representing a user update request into the events table
func AuditUserUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor, original, new *models.User) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:      null.StringFrom(pID),
		ActorID:       actorID,
		SubjectUserID: null.StringFrom(original.ID),
		Action:        "user.updated",
		Changeset:     calculateChangeset(original, new),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupCreated inserts an event representing group creation into the events table
func AuditGroupCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, g *models.Group) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(g.ID),
		Action:         "group.created",
		Changeset:      calculateChangeset(&models.Group{}, g),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupUpdated inserts an event representing group update into the events table
func AuditGroupUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, o, g *models.Group) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(g.ID),
		Action:         "group.updated",
		Changeset:      calculateChangeset(o, g),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupDeleted inserts an event representing group deletion into the events table
func AuditGroupDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, o, g *models.Group) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(g.ID),
		Action:         "group.deleted",
		Changeset:      calculateChangeset(o, g),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupHierarchyCreated inserts an event representing group hierarchy creation into the events table
func AuditGroupHierarchyCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupHierarchy) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(m.ParentGroupID),
		Action:         "group.hierarchy.added",
		Changeset:      calculateChangeset(&models.GroupHierarchy{}, m),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupHierarchyUpdated inserts an event representing group hierarchy update into the events table
func AuditGroupHierarchyUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupHierarchy) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(m.ParentGroupID),
		Action:         "group.hierarchy.updated",
		Changeset:      calculateChangeset(&models.GroupHierarchy{}, m),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupHierarchyDeleted inserts an event representing group hierarchy deletion into the events table
func AuditGroupHierarchyDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupHierarchy) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(m.ParentGroupID),
		Action:         "group.hierarchy.removed",
		Changeset:      []string{},
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupMembershipCreated inserts an event representing group membership creation into the events table
func AuditGroupMembershipCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupMembership) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(m.GroupID),
		SubjectUserID:  null.StringFrom(m.UserID),
		Action:         "group.member.added",
		Changeset:      calculateGroupMembershipChangeset(&models.GroupMembership{}, m),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupMembershipUpdated inserts an event representing group membership update into the events table
func AuditGroupMembershipUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, original, m *models.GroupMembership) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(m.GroupID),
		SubjectUserID:  null.StringFrom(m.UserID),
		Action:         "group.member.updated",
		Changeset:      calculateGroupMembershipChangeset(original, m),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupMembershipDeleted inserts an event representing group membership deletion into the events table
func AuditGroupMembershipDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupMembership) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(m.GroupID),
		SubjectUserID:  null.StringFrom(m.UserID),
		Action:         "group.member.removed",
		Changeset:      []string{},
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupMemberDemoted inserts an event representing group member being demoted from admin into the events table
func AuditGroupMemberDemoted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupMembership) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(m.GroupID),
		SubjectUserID:  null.StringFrom(m.UserID),
		Action:         "group.member.demoted",
		Changeset:      []string{},
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupMemberPromoted inserts an event representing group member being promoted to admin into the events table
func AuditGroupMemberPromoted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupMembership) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(m.GroupID),
		SubjectUserID:  null.StringFrom(m.UserID),
		Action:         "group.member.promoted",
		Changeset:      []string{},
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupMembershipApproved inserts an event representing group membership approval into the events table
func AuditGroupMembershipApproved(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupMembership, kind string) ([]*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	var action string

	switch kind {
	case "new_member":
		action = "group.member.request.approved"
	case "admin_promotion":
		action = "admin.promotion.request.approved"
	default:
		return nil, ErrUnknownRequestKind
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(m.GroupID),
		SubjectUserID:  null.StringFrom(m.UserID),
		Action:         action,
		Changeset:      calculateGroupMembershipChangeset(&models.GroupMembership{}, m),
		Message:        "Request was approved.",
	}

	if err := event.Insert(ctx, exec, boil.Infer()); err != nil {
		return nil, err
	}

	memEvent, err := AuditGroupMembershipCreated(ctx, exec, pID, actor, m)
	if err != nil {
		return nil, err
	}

	return []*models.AuditEvent{&event, memEvent}, nil
}

// AuditGroupMembershipRevoked inserts an event representing group membership revokation into the events table
func AuditGroupMembershipRevoked(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, r *models.GroupMembershipRequest) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	var action string

	switch r.Kind {
	case "new_member":
		action = "group.member.request.revoked"
	case "admin_promotion":
		action = "admin.promotion.request.revoked"
	default:
		return nil, ErrUnknownRequestKind
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(r.GroupID),
		SubjectUserID:  null.StringFrom(r.UserID),
		Action:         action,
		Changeset:      []string{},
		Message:        "Request was revoked.",
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupMembershipDenied inserts an event representing group membership denial into the events table
func AuditGroupMembershipDenied(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, r *models.GroupMembershipRequest) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	var action string

	switch r.Kind {
	case "new_member":
		action = "group.member.request.denied"
	case "admin_promotion":
		action = "admin.promotion.request.denied"
	default:
		return nil, ErrUnknownRequestKind
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(r.GroupID),
		SubjectUserID:  null.StringFrom(r.UserID),
		Action:         action,
		Changeset:      []string{},
		Message:        "Request was denied.",
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupMembershipRequestCreated inserts an event representing a group membership request into the events table
func AuditGroupMembershipRequestCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, r *models.GroupMembershipRequest) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	var action string

	switch r.Kind {
	case "new_member":
		action = "group.member.request.created"
	case "admin_promotion":
		action = "admin.promotion.request.created"
	default:
		return nil, ErrUnknownRequestKind
	}

	event := models.AuditEvent{
		ParentID:       null.StringFrom(pID),
		ActorID:        actorID,
		SubjectGroupID: null.StringFrom(r.GroupID),
		SubjectUserID:  null.StringFrom(r.UserID),
		Action:         action,
		Changeset:      []string{},
		Message:        "Request was created.",
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupOrganizationCreated inserts an event representing group linking an organization into the events table
func AuditGroupOrganizationCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupOrganization) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:              null.StringFrom(pID),
		ActorID:               actorID,
		SubjectGroupID:        null.StringFrom(m.GroupID),
		SubjectOrganizationID: null.StringFrom(m.OrganizationID),
		Action:                "group.organization.linked",
		Changeset:             []string{},
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupOrganizationDeleted inserts an event representing group unlinking an organization into the events table
func AuditGroupOrganizationDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupOrganization) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:              null.StringFrom(pID),
		ActorID:               actorID,
		SubjectGroupID:        null.StringFrom(m.GroupID),
		SubjectOrganizationID: null.StringFrom(m.OrganizationID),
		Action:                "group.organization.unlinked",
		Changeset:             []string{},
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditOrganizationCreated inserts an event representing an organization being created
func AuditOrganizationCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, o *models.Organization) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:              null.StringFrom(pID),
		ActorID:               actorID,
		SubjectOrganizationID: null.StringFrom(o.ID),
		Action:                "organization.created",
		Changeset:             calculateChangeset(&models.Organization{}, o),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditOrganizationDeleted inserts an event representing an organization being deleted
func AuditOrganizationDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, o *models.Organization) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:              null.StringFrom(pID),
		ActorID:               actorID,
		SubjectOrganizationID: null.StringFrom(o.ID),
		Action:                "organization.deleted",
		Changeset:             []string{},
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupApplicationCreated inserts an event representing group linking an application into the events table
func AuditGroupApplicationCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupApplication) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:             null.StringFrom(pID),
		ActorID:              actorID,
		SubjectGroupID:       null.StringFrom(m.GroupID),
		SubjectApplicationID: null.StringFrom(m.ApplicationID),
		Action:               "group.application.linked",
		Changeset:            []string{},
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupApplicationDeleted inserts an event representing group unlinking an application into the events table
func AuditGroupApplicationDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupApplication) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:             null.StringFrom(pID),
		ActorID:              actorID,
		SubjectGroupID:       null.StringFrom(m.GroupID),
		SubjectApplicationID: null.StringFrom(m.ApplicationID),
		Action:               "group.application.unlinked",
		Changeset:            []string{},
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditApplicationCreated inserts an event representing an application being created
func AuditApplicationCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.Application) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:             null.StringFrom(pID),
		ActorID:              actorID,
		SubjectApplicationID: null.StringFrom(a.ID),
		Action:               "application.created",
		Changeset:            calculateChangeset(&models.Application{}, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditApplicationDeleted inserts an event representing an application being deleted
func AuditApplicationDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.Application) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:             null.StringFrom(pID),
		ActorID:              actorID,
		SubjectApplicationID: null.StringFrom(a.ID),
		Action:               "application.deleted",
		Changeset:            []string{},
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditApplicationUpdated inserts an event representing application update into the events table
func AuditApplicationUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, o, a *models.Application) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:             null.StringFrom(pID),
		ActorID:              actorID,
		SubjectApplicationID: null.StringFrom(a.ID),
		Action:               "application.updated",
		Changeset:            calculateChangeset(o, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditApplicationTypeCreated inserts an event representing an application type being created
func AuditApplicationTypeCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.ApplicationType) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "application_type.created",
		Changeset: calculateChangeset(&models.ApplicationType{}, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditApplicationTypeDeleted inserts an event representing an application type being deleted
func AuditApplicationTypeDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.ApplicationType) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "application_type.deleted",
		Changeset: calculateChangeset(a, &models.ApplicationType{}),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditApplicationTypeUpdated inserts an event representing application type update into the events table
func AuditApplicationTypeUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, o, a *models.ApplicationType) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "application_type.updated",
		Changeset: calculateChangeset(o, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupApplicationApproved inserts an event representing group application approval into the events table
func AuditGroupApplicationApproved(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, m *models.GroupApplication) ([]*models.AuditEvent, error) {
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:             null.StringFrom(pID),
		ActorID:              actorID,
		SubjectApplicationID: null.StringFrom(m.ApplicationID),
		SubjectGroupID:       null.StringFrom(m.GroupID),
		Action:               "group.application.request.approved",
		Message:              "Request was approved.",
	}

	if err := event.Insert(ctx, exec, boil.Infer()); err != nil {
		return nil, err
	}

	memEvent, err := AuditGroupApplicationCreated(ctx, exec, pID, actor, m)
	if err != nil {
		return nil, err
	}

	return []*models.AuditEvent{&event, memEvent}, nil
}

// AuditGroupApplicationDenied inserts an event representing group application denial into the events table
func AuditGroupApplicationDenied(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, r *models.GroupApplicationRequest) (*models.AuditEvent, error) {
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:             null.StringFrom(pID),
		ActorID:              actorID,
		SubjectApplicationID: null.StringFrom(r.ApplicationID),
		SubjectGroupID:       null.StringFrom(r.GroupID),
		Action:               "group.application.request.denied",
		Changeset:            []string{},
		Message:              "Request was denied.",
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupApplicationRequestCreated inserts an event representing a group application request into the events table
func AuditGroupApplicationRequestCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, r *models.GroupApplicationRequest) (*models.AuditEvent, error) {
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:             null.StringFrom(pID),
		ActorID:              actorID,
		SubjectApplicationID: null.StringFrom(r.ApplicationID),
		SubjectGroupID:       null.StringFrom(r.GroupID),
		Action:               "group.application.request.created",
		Changeset:            []string{},
		Message:              "Created requested to link application to group.",
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditGroupApplicationRequestRevoked inserts an event representing group application request revokation into the events table
func AuditGroupApplicationRequestRevoked(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, r *models.GroupApplicationRequest) (*models.AuditEvent, error) {
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:             null.StringFrom(pID),
		ActorID:              actorID,
		SubjectApplicationID: null.StringFrom(r.ApplicationID),
		SubjectGroupID:       null.StringFrom(r.GroupID),
		Action:               "group.application.request.revoked",
		Changeset:            []string{},
		Message:              "Request was revoked.",
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditNotificationTypeCreated inserts an event representing a notification type being created
func AuditNotificationTypeCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.NotificationType) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "notification_type.created",
		Changeset: calculateChangeset(&models.NotificationType{}, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditNotificationTypeDeleted inserts an event representing an notification type being deleted
func AuditNotificationTypeDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.NotificationType) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "notification_type.deleted",
		Changeset: calculateChangeset(a, &models.NotificationType{}),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditNotificationTypeUpdated inserts an event representing notification type update into the events table
func AuditNotificationTypeUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, o, a *models.NotificationType) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "notification_type.updated",
		Changeset: calculateChangeset(o, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditNotificationTargetCreated inserts an event representing a notification target being created
func AuditNotificationTargetCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.NotificationTarget) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "notification_target.created",
		Changeset: calculateChangeset(&models.NotificationTarget{}, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditNotificationTargetDeleted inserts an event representing an notification target being deleted
func AuditNotificationTargetDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.NotificationTarget) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "notification_target.deleted",
		Changeset: calculateChangeset(a, &models.NotificationTarget{}),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditNotificationTargetUpdated inserts an event representing notification target update into the events table
func AuditNotificationTargetUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, o, a *models.NotificationTarget) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "notification_target.updated",
		Changeset: calculateChangeset(o, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditNotificationPreferencesUpdated inserts an event representing notification preferences update into the events table
func AuditNotificationPreferencesUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, userID string, o, a UserNotificationPreferences) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	type userNotificationPreferencesAuditRecord struct {
		Preferences string `json:"preferences"`
	}

	beforeJSON, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}

	afterJSON, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}

	before := &userNotificationPreferencesAuditRecord{Preferences: string(beforeJSON)}
	after := &userNotificationPreferencesAuditRecord{Preferences: string(afterJSON)}

	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:      null.StringFrom(pID),
		ActorID:       actorID,
		Action:        "notification_preferences.updated",
		SubjectUserID: null.NewString(userID, true),
		Changeset:     calculateChangeset(before, after),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditExtensionCreated inserts an event representing a extension being created
func AuditExtensionCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.Extension) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "extension.created",
		Changeset: calculateChangeset(&models.Extension{}, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditExtensionUpdated inserts an event representing a extension being created
func AuditExtensionUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, o, a *models.Extension) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "extension.updated",
		Changeset: calculateChangeset(o, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditExtensionDeleted inserts an event representing an extension being deleted
func AuditExtensionDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.Extension) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "extension.deleted",
		Changeset: calculateChangeset(a, &models.Extension{}),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditExtensionResourceDefinitionCreated inserts an event representing a extension being created
func AuditExtensionResourceDefinitionCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, erd *models.ExtensionResourceDefinition) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "extension.erd.created",
		Changeset: calculateChangeset(&models.ExtensionResourceDefinition{}, erd),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditExtensionResourceDefinitionUpdated inserts an event representing a extension being created
func AuditExtensionResourceDefinitionUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, o, a *models.ExtensionResourceDefinition) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "extension.erd.updated",
		Changeset: calculateChangeset(o, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditExtensionResourceDefinitionDeleted inserts an event representing a extension being created
func AuditExtensionResourceDefinitionDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, erd *models.ExtensionResourceDefinition) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "extension.erd.deleted",
		Changeset: calculateChangeset(erd, &models.ExtensionResourceDefinition{}),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditSystemExtensionResourceCreated inserts an event representing an extension resource being created
func AuditSystemExtensionResourceCreated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.SystemExtensionResource) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "extension.resource.created",
		Changeset: calculateChangeset(&models.SystemExtensionResource{}, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditSystemExtensionResourceUpdated inserts an event representing a extension being created
func AuditSystemExtensionResourceUpdated(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, o, a *models.SystemExtensionResource) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "extension.resource.updated",
		Changeset: calculateChangeset(o, a),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}

// AuditSystemExtensionResourceDeleted inserts an event representing an extension being deleted
func AuditSystemExtensionResourceDeleted(ctx context.Context, exec boil.ContextExecutor, pID string, actor *models.User, a *models.SystemExtensionResource) (*models.AuditEvent, error) {
	// TODO non-user API actors don't exist in the governor database,
	// we need to figure out how to handle that relationship in the audit table
	var actorID null.String
	if actor != nil {
		actorID = null.StringFrom(actor.ID)
	}

	event := models.AuditEvent{
		ParentID:  null.StringFrom(pID),
		ActorID:   actorID,
		Action:    "extension.resource.deleted",
		Changeset: calculateChangeset(a, &models.SystemExtensionResource{}),
	}

	return &event, event.Insert(ctx, exec, boil.Infer())
}
