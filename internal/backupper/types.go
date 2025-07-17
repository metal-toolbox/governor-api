package backupper

import (
	"context"

	"github.com/aarondl/sqlboiler/v4/boil"
	crdbmodels "github.com/metal-toolbox/governor-api/internal/models/crdb"
	psqlmodels "github.com/metal-toolbox/governor-api/internal/models/psql"
)

// DBDriver is the type for database drivers.
type DBDriver string

const (
	// DBDriverCRDB is the database driver for CockroachDB.
	DBDriverCRDB DBDriver = "crdb"
	// DBDriverPostgres is the database driver for Postgres.
	DBDriverPostgres DBDriver = "postgres"
)

// BackupCRDB represents a backup for the CRDB database.
type BackupCRDB struct {
	ApplicationTypes crdbmodels.ApplicationTypeSlice `json:"application_types"`
	Applications     crdbmodels.ApplicationSlice     `json:"applications"`
	AuditEvents      crdbmodels.AuditEventSlice      `json:"audit_events"`

	GroupApplicationRequests crdbmodels.GroupApplicationRequestSlice `json:"group_application_requests"`
	GroupApplications        crdbmodels.GroupApplicationSlice        `json:"group_applications"`
	GroupHierarchies         crdbmodels.GroupHierarchySlice          `json:"group_hierarchies"`
	GroupMembershipRequests  crdbmodels.GroupMembershipRequestSlice  `json:"group_membership_requests"`
	GroupMemberships         crdbmodels.GroupMembershipSlice         `json:"group_memberships"`
	GroupOrganizations       crdbmodels.GroupOrganizationSlice       `json:"group_organizations"`
	Groups                   crdbmodels.GroupSlice                   `json:"groups"`

	NotificationPreferences crdbmodels.NotificationPreferenceSlice `json:"notification_preferences"`
	NotificationTargets     crdbmodels.NotificationTargetSlice     `json:"notification_targets"`
	NotificationTypes       crdbmodels.NotificationTypeSlice       `json:"notification_types"`

	Organizations crdbmodels.OrganizationSlice `json:"organizations"`

	Users crdbmodels.UserSlice `json:"users"`

	ExtensionResourceDefinitions crdbmodels.ExtensionResourceDefinitionSlice `json:"extension_resource_definitions"`
	Extensions                   crdbmodels.ExtensionSlice                   `json:"extensions"`
	SystemExtensionResources     crdbmodels.SystemExtensionResourceSlice     `json:"system_extension_resources"`
	UserExtensionResources       crdbmodels.UserExtensionResourceSlice       `json:"user_extension_resources"`
}

// BackupPSQL represents a backup for the Postgres database
type BackupPSQL struct {
	ApplicationTypes psqlmodels.ApplicationTypeSlice `json:"application_types"`
	Applications     psqlmodels.ApplicationSlice     `json:"applications"`
	AuditEvents      psqlmodels.AuditEventSlice      `json:"audit_events"`

	GroupApplicationRequests psqlmodels.GroupApplicationRequestSlice `json:"group_application_requests"`
	GroupApplications        psqlmodels.GroupApplicationSlice        `json:"group_applications"`
	GroupHierarchies         psqlmodels.GroupHierarchySlice          `json:"group_hierarchies"`
	GroupMembershipRequests  psqlmodels.GroupMembershipRequestSlice  `json:"group_membership_requests"`
	GroupMemberships         psqlmodels.GroupMembershipSlice         `json:"group_memberships"`
	GroupOrganizations       psqlmodels.GroupOrganizationSlice       `json:"group_organizations"`
	Groups                   psqlmodels.GroupSlice                   `json:"groups"`

	NotificationPreferences psqlmodels.NotificationPreferenceSlice `json:"notification_preferences"`
	NotificationTargets     psqlmodels.NotificationTargetSlice     `json:"notification_targets"`
	NotificationTypes       psqlmodels.NotificationTypeSlice       `json:"notification_types"`

	Organizations psqlmodels.OrganizationSlice `json:"organizations"`

	Users psqlmodels.UserSlice `json:"users"`

	ExtensionResourceDefinitions psqlmodels.ExtensionResourceDefinitionSlice `json:"extension_resource_definitions"`
	Extensions                   psqlmodels.ExtensionSlice                   `json:"extensions"`
	SystemExtensionResources     psqlmodels.SystemExtensionResourceSlice     `json:"system_extension_resources"`
	UserExtensionResources       psqlmodels.UserExtensionResourceSlice       `json:"user_extension_resources"`
}

type insertable interface {
	Insert(ctx context.Context, exec boil.ContextExecutor, columns boil.Columns) error
}

type restorationGroup struct {
	name string
	data []insertable
}
