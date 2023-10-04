package v1alpha1

const (
	// Version is the API version constant
	Version = "v1alpha1"

	// GovernorEventCreate is the action passed on create events
	GovernorEventCreate = "CREATE"
	// GovernorEventUpdate is the action passed on update events
	GovernorEventUpdate = "UPDATE"
	// GovernorEventDelete is the action passed on delete events
	GovernorEventDelete = "DELETE"
	// GovernorEventApprove is the action passed on approve events
	GovernorEventApprove = "APPROVE"
	// GovernorEventDeny is the action passed on deny events
	GovernorEventDeny = "DENY"
	// GovernorEventRevoke is the action passed on revoke events
	GovernorEventRevoke = "REVOKE"

	// GovernorUsersEventSubject is the subject name for user events (minus the subject prefix)
	GovernorUsersEventSubject = "users"
	// GovernorGroupsEventSubject is the subject name for groups events (minus the subject prefix)
	GovernorGroupsEventSubject = "groups"
	// GovernorMembersEventSubject is the subject name for members events (minus the subject prefix)
	GovernorMembersEventSubject = "members"
	// GovernorMemberRequestsEventSubject is the subject name for member request events (minus the subject prefix)
	GovernorMemberRequestsEventSubject = "members.requests"
	// GovernorHierarchiesEventSubject is the subject name for group hierarchy events (minus the subject prefix)
	GovernorHierarchiesEventSubject = "hierarchies"
	// GovernorApplicationsEventSubject is the subject name for application events (minus the subject prefix)
	GovernorApplicationsEventSubject = "apps"
	// GovernorApplicationLinksEventSubject is the subject name for application link events (minus the subject prefix)
	GovernorApplicationLinksEventSubject = "applinks"
	// GovernorApplicationLinkRequestsEventSubject is the subject name for applink request events (minus the subject prefix)
	GovernorApplicationLinkRequestsEventSubject = "applinks.requests"
	// GovernorApplicationTypesEventSubject is the subject name for application type events (minus the subject prefix)
	GovernorApplicationTypesEventSubject = "applicationtypes"
	// GovernorNotificationTypesEventSubject is the subject name for notification type events (minus the subject prefix)
	GovernorNotificationTypesEventSubject = "notification.types"
	// GovernorNotificationTargetsEventSubject is the subject name for notification target events (minus the subject prefix)
	GovernorNotificationTargetsEventSubject = "notification.targets"
	// GovernorExtensionsEventSubject is the subject name for extensions events (minus the subject prefix)
	GovernorExtensionsEventSubject = "extensions"
	// GovernorExtensionResourceDefinitionsEventSubject is the subject name for extensions resource definition events (minus the subject prefix)
	GovernorExtensionResourceDefinitionsEventSubject = "extension.erds"
)

// Event is an event notification from Governor.
type Event struct {
	Version              string `json:"version"`
	Action               string `json:"action"`
	AuditID              string `json:"audit_id,omitempty"`
	GroupID              string `json:"group_id,omitempty"`
	UserID               string `json:"user_id,omitempty"`
	ActorID              string `json:"actor_id,omitempty"`
	ApplicationID        string `json:"application_id,omitempty"`
	ApplicationTypeID    string `json:"application_type_id,omitempty"`
	NotificationTypeID   string `json:"notification_type_id,omitempty"`
	NotificationTargetID string `json:"notification_target_id,omitempty"`

	ExtensionID                   string `json:"extension_id,omitempty"`
	ExtensionResourceDefinitionID string `json:"extension_resource_definition_id,omitempty"`
	ExtensionResourceID           string `json:"extension_resource_id,omitempty"`

	// TraceContext is a map of values used for OpenTelemetry context propagation.
	TraceContext map[string]string `json:"traceContext"`
}
